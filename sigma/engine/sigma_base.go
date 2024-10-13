package engine

import (
	"context"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"unsafe"

	"github.com/lxt1045/utils/tag"
)

type Ruleset[T any] struct {
	cmds []cmd[T]

	rules []SigmaRule[T]
	ctx   context.Context
	ymls  map[int64]*SigmaYml[T]
}

func (rs *Ruleset[T]) SplitByPlatform() (setLinux, setWin, setNetwork *Ruleset[T]) {
	setLinux = &Ruleset[T]{
		rules: rs.rules,
		ctx:   rs.ctx,
		ymls:  rs.ymls,
	}
	setWin = &Ruleset[T]{
		rules: rs.rules,
		ctx:   rs.ctx,
		ymls:  rs.ymls,
	}
	setNetwork = &Ruleset[T]{
		rules: rs.rules,
		ctx:   rs.ctx,
		ymls:  rs.ymls,
	}

	for _, cmd := range rs.cmds {
		if strings.EqualFold(cmd.yml.Platform, "linux") {
			setLinux.cmds = append(setLinux.cmds, cmd)
			continue
		}
		if strings.EqualFold(cmd.yml.Platform, "network") {
			setNetwork.cmds = append(setNetwork.cmds, cmd)
			continue
		}
		setWin.cmds = append(setWin.cmds, cmd)
	}
	return
}

func (rs *Ruleset[T]) Eval(m *T, in []int64) (ruleIDs []int64) {
	if cap(in) > 0 {
		ruleIDs = in[:0]
	}
	for _, cmd := range rs.cmds {
		if cmd.detection(m) {
			ruleIDs = append(ruleIDs, cmd.RuleID)
		}
	}
	return
}

func (rs *Ruleset[T]) Eval2(m *T, in []*SigmaYml[T]) (ruleIDs []*SigmaYml[T]) {
	if cap(in) > 0 {
		ruleIDs = in[:0]
	}
	for _, cmd := range rs.cmds {
		if cmd.detection(m) {
			ruleIDs = append(ruleIDs, cmd.yml)
		}
	}
	return
}

func (rs *Ruleset[T]) GetYml(ruleID int64) *SigmaYml[T] {
	return rs.ymls[ruleID]
}

func (rs *Ruleset[T]) GetYmls() (ymls []*SigmaYml[T]) {
	for _, r := range rs.cmds {
		ymls = append(ymls, r.yml)
	}
	return
}

// SigmaRule sigma 加工后的条件信息
type SigmaRule[T any] struct {
	Selections map[string]Selection
	Conditions []SigmaBranch[T] // 树形结构，类似 AST (抽象语法树)

	yml *SigmaYml[T]
	ctx context.Context
}

// SigmaBranch sigma 里的条件分支
type SigmaBranch[T any] struct {
	rule *SigmaRule[T]

	SelectionKey string    // selection 的名字
	Selection    Selection `json:"-"` // SelectionKey 在parse阶段就转成 Selection，避免运行时损耗

	CMD func(m *T) bool `json:"-"` // 最终执行的语句

	Not  bool
	OR   bool             // true: OR ; false:  AND ;
	Sons []SigmaBranch[T] // 子条件: 一个括号一个子条件; 条件变化时也要换一个条件，比如 OR 后出现 AND；
}

/*
	ID: 编号： {大分类编号}	{二级分类 三位数字}{初始分 两位}{系统 1 Wnindows 2Linux}{各级别编号 三位}
	|				|	3位			|	2位		|		1位					|		3位		|
		{大分类编号}	{二级分类}		{初始分}	{系统 1 Wnindows 2Linux}	{各级别编号}
	1001302004:	 1		001				30				2							004
	大分类说明(Analyzetype配置)：
	"001-运行进程"
	"002-文件"
	"003-认证"
	"004-执行"
	"005-启动程序"
	"006-管道"
	"007-通信"
	"008-资源访问"
	"009-进程访问"
	"010-网络"
	"011-进程加载"
	"012-注册表"
*/
// SigmaYml[T] sigma 原始配置信息
type SigmaYml[T any] struct {
	Title          string                 `yaml:"title"`          //规则名称
	RuleID         int64                  `yaml:"id"`             // 规则id
	Type           string                 `yaml:"type"`           // 规则类型 type: multiple/single/merger
	Status         int                    `yaml:"status"`         // 灰度状态(1是正常，0是关闭，2是灰度)
	Decisioncenter int                    `yaml:"decisioncenter"` // 是否进行多条判断(1是不进行多条判断，2是进行多条判断)
	Description    string                 `yaml:"description"`
	Detection      map[string]interface{} `yaml:"detection"` // 规则具体内容 // detection.condition 规则判断逻辑
	Level          string                 `yaml:"level"`     // 危险等级分为INFO(信息)、LOW(低)、MEDIUM(中)、HIGH(高)
	Tags           []string               `yaml:"tags"`      // attack分类
	Tag            string                 `yaml:"-"`
	TagInfos       map[string]*tag.TagInfo

	// 以下是新增的特性
	Time          string   `yaml:"time"` // 2023.9.13
	Example       string   `yaml:"example"`
	Analyze       string   `yaml:"analyze"`
	Common        string   `yaml:"common"`
	Environment   string   `yaml:"environment"`
	Class         string   `yaml:"class"`
	Source        string   `yaml:"source"`
	Focus         string   `yaml:"focus"`
	Author        string   `yaml:"author"`
	Version       string   `yaml:"version"`        // 1.0.0
	Application   string   `yaml:"application"`    // 默认规则
	Why           string   `yaml:"why"`            // 监控 ifconfig ip
	FalsePositive string   `yaml:"false_positive"` //
	Frequency     string   `yaml:"frequency"`      //
	Platform      string   `yaml:"platform"`       // Linux
	Much          string   `yaml:"much"`           //
	Initial       int      `yaml:"initial"`        // 30
	Analyzetype   int      `yaml:"analyzetype"`    // 1
	Behavior      Behavior `yaml:"behavior"`       // 30
	Offsets       Offsets  `yaml:"-"`              // Behavior 在pb.Log的偏移量
}

type Behavior struct {
	Object      string   `yaml:"Object"`      // #一级主体
	Objects     []string `yaml:"Objects"`     // #一级主体
	Subject     string   `yaml:"Subject"`     // #一级主体为空选为二级
	When        string   `yaml:"When"`        // #执行时间
	Who         string   `yaml:"Who"`         // #身份权限/准确执行体
	What        string   `yaml:"What"`        // #动作
	Role        string   `yaml:"Role"`        // #权限
	Where       string   `yaml:"Where"`       // #执行位置
	Why         string   `yaml:"Why"`         // 应该为解释说明内容
	Whom        string   `yaml:"Whom"`        // 关联
	Which       string   `yaml:"Which"`       // 标识mark吧,正常/攻击/脚本/服务类似
	How         string   `yaml:"How"`         // 怎么做的描述
	HowMuch     string   `yaml:"HowMuch"`     // 频率
	Effect      string   `yaml:"Effect"`      // 应该为变化区间的东西
	Environment string   `yaml:"Environment"` // 应该为标识上下文
}

type Offsets struct {
	Object  uintptr
	Objects struct {
		Connectors []string // 连接符
		Offsets    []uintptr
	}
	ObjectType      int
	Subject         uintptr
	SubjectType     int
	When            uintptr
	WhenType        int
	Who             uintptr
	WhoType         int
	What            uintptr
	WhatType        int
	Role            uintptr
	RoleType        int
	Where           uintptr
	WhereType       int
	Why             uintptr
	WhyType         int
	Whom            uintptr
	WhomType        int
	Which           uintptr
	WhichType       int
	How             uintptr
	HowType         int
	HowMuch         uintptr
	HowMuchType     int
	Effect          uintptr
	EffectType      int
	Environment     uintptr
	EnvironmentType int
}

const (
	OffsetsTypeString = 0
	OffsetsTypeInt64  = 1
	OffsetsTypeTime   = 2
)

func (s *SigmaYml[T]) GetLogTag(key string) *tag.TagInfo {
	if s.TagInfos == nil {
		var err error
		s.TagInfos, err = tag.NewTagInfos(new(T), "json")
		if err != nil {
			panic(err)
		}
	}
	return s.TagInfos[key]
}

func (s *SigmaYml[T]) ParseOffset() (err error) {
	behavior := reflect.ValueOf(&s.Behavior)
	typBehavior := reflect.TypeOf(&s.Behavior)
	offsets := reflect.ValueOf(&s.Offsets)
	behavior = behavior.Elem()
	typBehavior = typBehavior.Elem()
	offsets = offsets.Elem()

	if l := len(s.Behavior.Objects); l > 0 {
		for i := 0; i < len(s.Behavior.Objects); i += 2 {
			/*
				Objects:
				    - parent_path
				    - ";"
				    - path
				或者：
				Objects:
				    - "以进程"
				    - parent_path
				    - "路径"
				    - path
				    - "启动"
			*/

			key := s.Behavior.Objects[i]
			ti := s.GetLogTag(key)
			if ti == nil {
				err = UtilsUnexpected.Clonef("rule_id[%d]; Field[Objects] Value[%s] is not exist", s.RuleID, key)
				return
			}
			if ti.BaseKind != reflect.String {
				err = UtilsUnexpected.Clonef("key[%s]-Field[Objects] must be string/int64 type, got %s", key, ti.BaseKind)
				return
			}
			s.Offsets.Objects.Offsets = append(s.Offsets.Objects.Offsets, ti.Offset)
			if l > i+1 {
				s.Offsets.Objects.Connectors = append(s.Offsets.Objects.Connectors, s.Behavior.Objects[i+1])
			}
		}
	}

	for i := 0; i < behavior.NumField(); i++ {
		v := behavior.Field(i)
		t := typBehavior.Field(i)
		name := t.Name //v.Type().Name()
		if name == "Objects" {
			continue // Objects 需要特殊处理
		}
		vSet := offsets.FieldByName(name)
		if !vSet.CanSet() {
			err = UtilsUnexpected.Clonef("Field[%s] can not set", name)
			return
		}
		key := v.String()
		if key == "" {
			continue
		}
		ti := s.GetLogTag(key)
		if ti == nil {
			err = UtilsUnexpected.Clonef("rule_id[%d]; Field[%s] Value[%s] is not exist", s.RuleID, name, key)
			return
		}
		if ti.BaseKind != reflect.String && ti.BaseKind != reflect.Int64 {
			err = UtilsUnexpected.Clonef("key[%s]-Field[%s] must be string/int64 type, got %s", key, name, ti.BaseKind)
			return
		}
		if ti.BaseKind == reflect.Int64 {
			typeName := name + "Type"
			vTypeSet := offsets.FieldByName(typeName)
			if !vTypeSet.CanSet() {
				err = UtilsUnexpected.Clonef("Field[%s] can not set", typeName)
				return
			}
			var typ int64 = OffsetsTypeInt64
			if strings.HasSuffix(ti.TagName, "time") {
				typ = OffsetsTypeTime
			}

			vTypeSet.SetInt(typ)
		}
		vv := reflect.ValueOf(ti.Offset)
		vSet.Set(vv)
	}
	return
}

type Selection struct {
	Key    string
	Strs   []string
	Int64s []int64
	Bools  []bool
	Func   string
	All    bool
	Ti     *tag.TagInfo
}
type cmd[T any] struct {
	RuleID     int64
	detection  func(m *T) bool
	yml        *SigmaYml[T]
	RuleStatus int
}

// var bytesPool = make([][]byte, 128)
// func containsnotmatchcase(s, substr string) (ok bool) {
// 	// lower := strings.ToLower(s)
// 	// TODO: 这里遍历转小写+strings.Contains 速度应该比不上 KMP 算法； 有时间就改
// 	bs := bytesPool[gid][:0]
// 	lastIdx := 0
// 	for i := 0; i < len(s); i++ {
// 		c := s[i]
// 		if 'A' <= c && c <= 'Z' {
// 			bs = append(bs, s[lastIdx:i]...)
// 			bs = append(bs, c+'a'-'A')
// 			lastIdx = i + 1
// 		}
// 	}
// 	lower := s
// 	if lastIdx > 0 {
// 		if lastIdx < len(s) {
// 			bs = append(bs, s[lastIdx:]...)
// 		}
// 		lower = unsafe.String(&bs[0], len(bs))
// 	}
// 	strings.Contains(lower, substr)
// 	bytesPool[gid] = bs
// 	return
// }

func startswithnotmatchcase(s, substr string) (ok bool) {
	// return strings.HasPrefix(strings.ToLower(s), substr)

	if len(s) < len(substr) {
		return false
	}
	return strings.EqualFold(s[:len(substr)], substr)
}

func endswithnotmatchcase(s, substr string) (ok bool) {
	// strings.HasSuffix(strings.ToLower(s), substr)

	if len(s) < len(substr) {
		return false
	}
	return strings.EqualFold(s[len(s)-len(substr):], substr)
}

var mfuncs = map[string]func(s, substr string) bool{
	// "containsnotmatchcase":   containsnotmatchcase,
	"contains":               strings.Contains,
	"startswith":             strings.HasPrefix,
	"startswithnotmatchcase": startswithnotmatchcase,
	"endswith":               strings.HasSuffix,
	"endswithnotmatchcase":   endswithnotmatchcase,
}

func (do *Selection) toRealFunc(fName string) (f func(s, substr string) bool) {
	f = mfuncs[fName]
	if f == nil {
		return
	}
	if strings.HasSuffix(fName, "notmatchcase") {
		// 忽略大小写时，全部转成小写
		for i := range do.Strs {
			do.Strs[i] = strings.ToLower(do.Strs[i])
		}
	}
	return
}

func regexpCompile(str string) (reg *regexp.Regexp, err error) {
	str, err = strconv.Unquote(strings.Replace(strconv.Quote(str), `\\u`, `\u`, -1))
	if err != nil {
		return
	}
	reg, err = regexp.Compile(str)
	if err != nil {
		return
	}
	return
}

func structElemToStrFunc[T any](ti *tag.TagInfo) (f func(*T) string, err error) {
	offset := ti.Offset
	switch ti.BaseKind {
	case reflect.String:
		f = func(m *T) (str string) {
			return StructElem[string](m, offset)
		}
	case reflect.Int64:
		f = func(m *T) (str string) {
			n := StructElem[int64](m, offset)
			return strconv.FormatInt(n, 10)
		}
	case reflect.Bool:
		f = func(m *T) (str string) {
			b := StructElem[bool](m, offset)
			if b {
				return "true"
			}
			return "false"
		}
	default:
		err = UtilsUnexpected.Clonef("not support the elem type:%s", ti.BaseKind.String())
		return
	}
	return
}

func StructElem[E string | int64 | bool, T any](p *T, offset uintptr) (pOut E) {
	p1 := unsafe.Pointer(p)
	p2 := unsafe.Pointer(uintptr(p1) + uintptr(offset))
	return *(*E)(p2)
}

func StructElemPointer[E string | int64 | bool, T any](p *T, offset uintptr) (pOut *E) {
	p1 := unsafe.Pointer(p)
	p2 := unsafe.Pointer(uintptr(p1) + uintptr(offset))
	return (*E)(p2)
}
