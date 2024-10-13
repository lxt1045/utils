package engine

import (
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/lxt1045/utils/delay"
	"github.com/lxt1045/utils/tag"
)

/*
	需要增加能表明主事件的 key ， 用于计算进程树的中间节点
*/
// SigmaYmlMul 多条配置信息
type SigmaYmlMul struct {
	Title         string `yaml:"title"`          //规则名称
	RuleID        int64  `yaml:"id"`             // 规则id
	Type          string `yaml:"type"`           // 规则类型 type: multiple/single/merger
	Status        int    `yaml:"status"`         // 灰度状态(1是正常，0是关闭，2是灰度)
	TechniqueType string `yaml:"technique_type"` //
	Description   string `yaml:"description"`    // 描述
	Seriousness   string `yaml:"seriousness"`    // 危险等级分为INFO(信息)、LOW(低)、MEDIUM(中)、HIGH(高)
	Platform      string `yaml:"platform"`       //规则名称
	Initial       int    `yaml:"initial"`        // 30
	Analyzetype   int    `yaml:"analyzetype"`    // 1

	Detection DetectionMul `yaml:"detection"` // 规则具体内容 // detection.condition 规则判断逻辑
}

type DetectionMul struct {
	Object    string           `yaml:"object"`    // # 主体规则，以该规则为主体，画关联图谱进程树。
	Ordered   bool             `yaml:"ordered"`   // 根据rule有序的；有序时 count不起作用
	Selection map[string]int64 `yaml:"selection"` // where 子语句; AND 关系
	Condition ConditionMul     `yaml:"condition"` //  where 条件
}

type ConditionMul struct {
	TimeWindow  string         `yaml:"time_window"` // time window ; 时间窗口
	GroupBys    []string       `yaml:"group_by"`    // group by
	Havings     []string       `yaml:"having"`      // having ; group by 的 where 条件（WHERE 关键字无法与合计函数一起使用）
	CountLimits map[string]int `yaml:"count"`       // count limit ; 统计下限，低于统计值的数据行丢弃
}

// StreamRuleset 流处理的处理中心
type StreamRuleset[T any] struct {
	rules []*streamRule[T] // 所有规则
	tis   map[string]*tag.TagInfo

	allLastEventIDAdddr map[int64]*int64                    // map[rule_id]&rule_id
	mEvals              map[int64][]func(m *T) []DataHit[T] // map[singleRuleID]Evals
}

func (rs StreamRuleset[T]) Eval(ruleID int64, m *T) (hitss [][]DataHit[T]) {
	fEvals := rs.mEvals[ruleID]
	for _, f := range fEvals {
		hits := f(m)
		if len(hits) > 0 {
			hitss = append(hitss, hits)
		}
	}
	return
}

func (rs StreamRuleset[T]) ResetEventID(eventID int64) {
	pEventID := rs.allLastEventIDAdddr[eventID]
	if pEventID != nil {
		atomic.CompareAndSwapInt64(pEventID, eventID, 0)
	}
}

func (rs *StreamRuleset[T]) Build() (err error) {
	rs.mEvals = make(map[int64][]func(m *T) []DataHit[T])
	rs.allLastEventIDAdddr = make(map[int64]*int64)
	// 处理 Eval 函数
	for _, rule := range rs.rules {
		for ruleID := range rule.mCountsIdx {
			fEval, err1 := rule.ToEvalFunc(ruleID)
			if err1 != nil {
				err = err1
				return
			}
			rs.mEvals[ruleID] = append(rs.mEvals[ruleID], fEval)
		}
	}

	// 处理 singleRuleID map，便于提前跳过

	return
}

type eventData[T any] struct {
	singleRuleID int64 // 单条规则的 规则id
	idxCounts    int
	data         *T
	group        *Group[T]
}

func (data eventData[T]) Post() {
	for _, postFunc := range data.group.postFuncs {
		postFunc(data)
	}
}

// 泛型 T 日志类型
// 执行流计算时需要的参数都记录在这
type streamRule[T any] struct { // sigma 配置中的 rule 别名
	delayQueue *delay.Queue[eventData[T]] // 跟时间窗口相关; TODO 把延时队列放到每个group中，减少锁争抢？
	ruleset    *StreamRuleset[T]

	countLimits []int32 // 每个 rule 一个统计技术变量

	// 没有group by的时候用 group1 提高性能；否则，用 groups + groupsLock
	fGetGroupKey func(m *T) (key string)
	group1       *Group[T]
	groups       map[string]*Group[T]
	groupsLock   sync.RWMutex

	// having 条件                 // having 条件的数量
	preFuncs          []func(d eventData[T]) // 前置处理(统计前处理)； 每一行 having 条件一个 func
	postFuncs         []func(d eventData[T]) // 后置处理(时间窗口后执行)；每一行 having 条件一个 func
	nHavingConditions int

	mainRuleID int64         // 主事件规则ID
	timeWindow int64         // ns 时间窗口
	yml        *SigmaYmlMul  //
	mCountsIdx map[int64]int // map[rule_id]idxCount
}

func (r *streamRule[T]) GetGroup(m *T) (g *Group[T]) {
	if r.fGetGroupKey == nil {
		if r.group1 == nil {
			r.groupsLock.Lock()
			defer r.groupsLock.Unlock()
			if r.group1 == nil {
				r.group1 = r.NewGroup()
			}
		}
		return r.group1
	}
	key := r.fGetGroupKey(m)
	r.groupsLock.RLock()
	g = r.groups[key]
	r.groupsLock.RUnlock()
	if g != nil {
		return
	}
	g1 := r.NewGroup()
	r.groupsLock.Lock()
	defer r.groupsLock.Unlock()
	g = r.groups[key]
	if g != nil {
		return
	}
	r.groups[key] = g1 // group 只增长没有删除，有可能导致膨胀
	g = g1
	return
}

type Group[T any] struct {
	counts    []int32 // 每个 rule 一个统计技术变量
	havings   []having[T]
	postFuncs []func(eventData[T])

	// 上次导出时间。
	// 如果不超过窗口，则使用上次的 event_id 并只导出自己；
	// 否则，还需要把队列中属于本 group 的所有 *T 都导出成同一个事件
	lastEventID int64
}

func (r *streamRule[T]) NewGroup() (g *Group[T]) {
	g = &Group[T]{
		counts:    make([]int32, len(r.countLimits)),
		havings:   make([]having[T], r.nHavingConditions),
		postFuncs: r.postFuncs,
	}
	for i := range g.havings {
		g.havings[i].leftMap = make(map[string]data[T])
		g.havings[i].rightMap = make(map[string]data[T])
	}
	return
}

type having[T any] struct {
	// left             atomic.Pointer[T]                  // 操作符左侧的值必须是T的成员（作为最新缓存，便于删除时更新结果）（通过原子操作读写）
	pair atomic.Pointer[havingEqualPair[T]] // 操作符的右边的值： 1. T的成员；（通过原子操作读写）
	// rightStaticValue string                             // 操作符的右边的值： 2. ==、like
	// rightInValues    map[string]struct{}                // 操作符的右边的值： 3. in

	leftMap   map[string]data[T] // 右侧条件快速索引
	leftLock  sync.RWMutex
	rightMap  map[string]data[T] // 左侧条件快速索引
	rightLock sync.RWMutex

	fDoDelay func(*T) //用于删除
}

type data[T any] struct {
	start int64 // 延时长度, 单位: s
	data  *T
}

type RuleType int16

const (
	RuleTypeSingle   RuleType = 1
	RuleTypeMultiple RuleType = 2
	RuleTypeMerger   RuleType = 3
)

type DataHit[T any] struct {
	MulRuleID    int64
	MainRuleID   int64 // 主事件ID
	SingleRuleID int64
	EventID      int64
	Score        int16
	RuleType     RuleType
	Data         *T
}

// 为了实现原子操作，把他们组成一对，达到原子读写的目的
type havingEqualPair[T any] struct {
	left  *T
	right *T
	start int64 // 过期时间
}

type HavingElem struct {
	RuleID int64
	Key    string
	ti     *tag.TagInfo
}

// 返回单位 ns
func parseTimeWindow(t string, ruleID int64) (ns int64, err error) {
	if "" == t {
		return
	}
	var i int64
	switch {
	case strings.HasSuffix(t, "sec"):
		n := strings.TrimSpace(t[:len(t)-len("sec")])
		i, err = strconv.ParseInt(n, 10, 64)
		if err == nil {
			ns = int64(time.Second) * i
		}
	case strings.HasSuffix(t, "min"):
		n := strings.TrimSpace(t[:len(t)-len("min")])
		i, err = strconv.ParseInt(n, 10, 64)
		if err == nil {
			ns = int64(time.Minute) * i
		}
	case strings.HasSuffix(t, "hour"):
		n := strings.TrimSpace(t[:len(t)-len("hour")])
		i, err = strconv.ParseInt(n, 10, 64)
		if err == nil {
			ns = int64(time.Hour) * i
		}
	default:
		err = UtilsUnexpected.Clonef("rule_id[%d]; time error:%+v", ruleID, t)
	}
	return
}
