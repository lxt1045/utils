package engine

import (
	"sync"

	"github.com/lxt1045/utils/tag"
)

/*
	需要增加能表明主事件的 key ， 用于计算进程树的中间节点
*/
// SigmaYmlMerge 多条配置信息
type SigmaYmlMerge struct {
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

	Detection DetectionMerge `yaml:"detection"` // 规则具体内容 // detection.condition 规则判断逻辑
}

type DetectionMerge struct {
	Selection map[string]int64 `yaml:"selection"` // where 子语句; AND 关系
	Condition ConditionMerge   `yaml:"condition"` //  where 条件
}

type ConditionMerge struct {
	TimeWindow string   `yaml:"time_window"` // time window ; 时间窗口； 支持: hour/min/sec
	GroupBys   []string `yaml:"group_by"`    // group by
}

// MergeRuleset 流处理的处理中心
type MergeRuleset[T any] struct {
	rules []*mergeRule[T] // 所有规则
	tis   map[string]*tag.TagInfo

	// fEvals []func(ruleID int, m *T) []DataHit[T]
	mEvals map[int64][]func(m *T) []DataHit[T] // map[singleRuleID]Evals
}

func (rs MergeRuleset[T]) Eval(ruleID int64, m *T) (hitss [][]DataHit[T]) {
	fEvals := rs.mEvals[ruleID]
	for _, f := range fEvals {
		hits := f(m)
		if len(hits) > 0 {
			hitss = append(hitss, hits)
		}
	}
	return
}

func (rs *MergeRuleset[T]) Build() (err error) {
	rs.mEvals = make(map[int64][]func(m *T) []DataHit[T])
	// 处理 Eval 函数
	for _, rule := range rs.rules {
		for _, ruleID := range rule.yml.Detection.Selection {
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

// 泛型 T 日志类型
// 执行流计算时需要的参数都记录在这
type mergeRule[T any] struct {
	ruleset *MergeRuleset[T]

	fGetGroupKey func(m *T) (key string)
	groups       map[string]*MGroup
	groupsLock   sync.RWMutex

	timeWindow int64          // ns 时间窗口
	yml        *SigmaYmlMerge //
}

type MGroup struct {
	lastEventID int64
	deadline    int64 // lastEventID 失效时间
	sync.Mutex        // 加锁实现单飞: singleflight
}

func (r *mergeRule[T]) MustGetGroup(m *T, eventID, tsNow int64) (g *MGroup) {
	key := r.fGetGroupKey(m)
	r.groupsLock.RLock()
	g = r.groups[key]
	r.groupsLock.RUnlock()
	if g != nil && g.deadline < tsNow {
		return
	}
	g1 := &MGroup{}
	r.groupsLock.Lock()
	defer func() {
		r.groupsLock.Unlock()
		if g != g1 {
			g.Lock()
			if g.lastEventID != 0 {
				g.Unlock()
				return
			}
		}
		defer g.Unlock() // derfer 嵌套defer
		g2 := r.getGroupFromDB(key, eventID, tsNow)
		g.lastEventID = g2.lastEventID
		g.deadline = g2.deadline
	}()
	g = r.groups[key]
	if g != nil {
		return
	}
	g = g1
	g.Lock()          // 先加锁，避免被其他协程抢占
	r.groups[key] = g // group 只增长没有删除，有可能导致膨胀
	return
}

func (r *mergeRule[T]) getGroupFromDB(key string, eventID, tsNow int64) (g MGroup) {

	g.lastEventID = eventID
	g.deadline = tsNow + r.timeWindow
	//
	return
}
