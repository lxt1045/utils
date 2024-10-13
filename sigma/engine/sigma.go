package engine

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strconv"
	"strings"

	"github.com/lxt1045/utils/log"
	"github.com/lxt1045/utils/tag"
	"gopkg.in/yaml.v2"
)

func NewRulesetByDir[T any](ctx context.Context, dir string, in T) (rs *Ruleset[T], err error) {
	bsYmls := [][]byte{}
	err = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || (!strings.HasSuffix(path, ".yml") && !strings.HasSuffix(path, ".yaml")) {
			return err
		}
		bs, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		bsYmls = append(bsYmls, bs)
		return nil
	})
	if err != nil {
		return
	}

	return NewRuleset(ctx, bsYmls, in)
}

func NewOneRuleset[T any](ctx context.Context, file string, in T) (rs *Ruleset[T], err error) {
	bs, err := os.ReadFile(file)
	if err != nil {
		return
	}
	return NewRuleset(ctx, [][]byte{bs}, in)
}

func NewRuleset[T any](ctx context.Context, bsYmls [][]byte, in T) (rs *Ruleset[T], err error) {
	tis, err := tag.NewTagInfos(in, "json")
	if err != nil {
		return
	}
	rules := []SigmaRule[T]{}
	for _, bsYml := range bsYmls {
		var yml SigmaYml[T]
		err = yaml.Unmarshal(bsYml, &yml)
		if err != nil {
			err = UtilsUnexpected.Clonef("err:%+v, rule:%s", err, string(bsYml))
			return
		}
		rule, err1 := parseRule[T](ctx, yml, tis)
		if err1 != nil {
			if UtilsUnexpected.Is(err1) {
				continue
			}
			err = err1
			return
		}

		rules = append(rules, rule)
	}

	rs = &Ruleset[T]{
		rules: rules,
		ctx:   ctx,
	}
	err = rs.Build()
	return
}

func (rs *Ruleset[T]) Build() (err error) {
	rs.ymls = make(map[int64]*SigmaYml[T])
	for _, rule := range rs.rules {
		rs.ymls[rule.yml.RuleID] = rule.yml

		if len(rule.Conditions) == 0 {
			log.Ctx(rule.ctx).Warn().Int64("rule_id", rule.yml.RuleID).Msg("rule.Conditions is empty")
			continue
		}
		detections := make([]func(m *T) bool, 0, len(rule.Conditions)) // 针对单个 sigma 文件的逻辑命令列表
		or := rule.Conditions[0].OR
		for _, c := range rule.Conditions {
			detection, err1 := c.toCMD()
			if err = err1; err != nil {
				return
			}
			detections = append(detections, detection)

			// 再次i检查一下一致性
			if or != c.OR {
				err = UtilsUnexpected.Clonef("id[%d]; and/or Mixed", rule.yml.RuleID)
				return
			}
		}

		sigmaCMD := cmd[T]{
			RuleID:     rule.yml.RuleID,
			RuleStatus: rule.yml.Status,
			yml:        rule.yml,
		}
		if or {
			sigmaCMD.detection = func(m *T) bool {
				for _, detection := range detections {
					if detection(m) {
						return true
					}
				}
				return false
			}
		} else {
			sigmaCMD.detection = func(m *T) bool {
				for _, detection := range detections {
					if !detection(m) {
						return false
					}
				}
				return true
			}
		}
		rs.cmds = append(rs.cmds, sigmaCMD)
	}
	return
}

// parseRule 解析 sigma 的 yaml 配置信息
func parseRule[T any](ctx context.Context, yml SigmaYml[T], tis map[string]*tag.TagInfo) (rule SigmaRule[T], err error) {
	condition, ok := yml.Detection["condition"].(string)
	if !ok || condition == "" {
		err = UtilsUnexpected.Clonef("sigma id[%d] has no condition", yml.RuleID)
		log.Ctx(ctx).Error().Caller().Err(err).Msg("rule")
		return
	}

	rule = SigmaRule[T]{
		Selections: make(map[string]Selection),
		yml:        &yml,
		ctx:        ctx,
	}
	for i := len(yml.Tags) - 1; i >= 0; i-- {
		yml.Tag += yml.Tags[i]
	}

	for k, v := range yml.Detection {
		if k == "condition" {
			continue
		}
		var s Selection
		mSelection, ok := v.(map[interface{}]interface{})
		if !ok {
			err = UtilsUnexpected.Clonef(
				"sigma id[%d]; selection[%+v] type error, must be \"map[interface{}]interface{}\", not %T",
				yml.RuleID, v, v)
			return
		}
		if len(mSelection) > 1 {
			err = UtilsUnexpected.Clonef("sigma id[%d] selection[%+v], len > 1", yml.RuleID, v)
			return
		}
		for mk, v := range mSelection {
			k := mk.(string)

			keys := strings.Split(k, "|")

			// 获取 key
			s.Key = keys[0]
			s.Ti = tagToInfo(s.Key, tis)
			if s.Ti == nil {
				err = UtilsUnexpected.Clonef("sigma id[%d] selection[%+v], key[%s] not found in obj: %T",
					yml.RuleID, v, s.Key, *new(T))
				return
			}

			if len(keys) > 1 {
				s.Func = keys[1]
			}
			if len(keys) > 2 && keys[2] == "all" {
				s.All = true // 表示以下并列条件必须同时满足；否则表示只要满足一个条件即可
			}

			// 其实，这几个也可以在这里用 []interface{} 存储，后面使用的时候再断言成具体类型
			switch x := v.(type) {
			case string:
				s.Strs = append(s.Strs, x)
			case int:
				s.Int64s = append(s.Int64s, int64(x))
			case bool:
				s.Bools = append(s.Bools, x)
			default:
				if v == nil {
					continue
				}
				values, ok := v.([]interface{})
				if !ok {
					err = UtilsUnexpected.Clonef("v:%+v, type:%T", v, v)
					return
				}
				for _, v := range values {
					switch x := v.(type) {
					case string:
						s.Strs = append(s.Strs, x)
					case int:
						s.Int64s = append(s.Int64s, int64(x))
					case bool:
						s.Bools = append(s.Bools, x)
					default:
						err = UtilsUnexpected.Clonef("type not support, k:%v, v:%v", k, v)
						return
					}
				}
			}
		}

		rule.Selections[k] = s
	}

	rule.Conditions, err = rule.ParseCondition(condition, rule.Selections)
	if err != nil {
		return
	}

	err = rule.yml.ParseOffset()
	if err != nil {
		return
	}
	return
}

func (rule *SigmaRule[T]) ParseCondition(str string, selections map[string]Selection) (branchs []SigmaBranch[T], err error) {
	str = strings.ReplaceAll(str, "(", " ( ")
	str = strings.ReplaceAll(str, ")", " ) ")
	strs := strings.Split(str, " ")

	if selections == nil {
		err = UtilsUnexpected.Clonef("id[%d]; selections dose not make", rule.yml.RuleID)
		return
	}
	branchs, _, err = rule.parseBranch(strs, selections)
	return
}

// parseBranch 解析条件分支
func (rule *SigmaRule[T]) parseBranch(strs []string, selections map[string]Selection) (branchs []SigmaBranch[T], n int, err error) {
	branch := SigmaBranch[T]{
		OR:   false,
		rule: rule,
	}
	for i := 0; i < len(strs); i++ {
		s := strs[i]
		s = strings.TrimSpace(s)

		switch strings.ToLower(s) {
		case "":
			continue

		// 括号开头，开启新的一段; 深度优先
		case "(":
			var used int
			branch.Sons, used, err = rule.parseBranch(strs[i+1:], selections)
			if err != nil {
				return
			}
			i += used + 1

			// 储存旧条件，并开启新的并列条件
			branchs = append(branchs, branch)
			branch = SigmaBranch[T]{
				OR:   false,
				rule: rule,
			}

		// 反括号结束，结束的一段
		case ")":
			n = i + 1
			return

		case "and":
			branch.OR = false
			if branch.Not {
				err = UtilsUnexpected.Clonef("id[%d]; and after not", rule.yml.RuleID)
				return
			}
			// 检查下一个条件（and、or）和上一个是否一致
			if len(branchs) == 1 {
				// 开始的selection的条件是不确定的，由后一个指定
				branchs[0].OR = branch.OR
			} else {
				c := branchs[len(branchs)-1]
				if c.OR == true {
					branchsSon := branchs
					branchs = append(branchs[:0:0], SigmaBranch[T]{
						OR:   branch.OR,
						Sons: branchsSon,
						rule: rule,
					})
					var used int
					branch.Sons, used, err = rule.parseBranch(strs[i+1:], selections)
					if err != nil {
						return
					}
					i += used + 1

					// 储存旧条件，并开启新的并列条件
					branchs = append(branchs, branch)
					branch = SigmaBranch[T]{
						OR:   false,
						rule: rule,
					}
				}
			}

		case "or":
			branch.OR = true
			if branch.Not {
				err = UtilsUnexpected.Clonef("id[%d]; or after not", rule.yml.RuleID)
				return
			}
			// 检查下一个条件（and、or）和上一个是否一致
			if len(branchs) == 1 {
				branchs[0].OR = branch.OR
			} else {
				c := branchs[len(branchs)-1]
				if c.OR == false {
					branchsSon := branchs
					branchs = append(branchs[:0:0], SigmaBranch[T]{
						OR:   branch.OR,
						Sons: branchsSon,
						rule: rule,
					})
					var used int
					branch.Sons, used, err = rule.parseBranch(strs[i+1:], selections)
					if err != nil {
						return
					}
					i += used + 1

					// 储存旧条件，并开启新的并列条件
					branchs = append(branchs, branch)
					branch = SigmaBranch[T]{
						OR:   false,
						rule: rule,
					}
				}
			}

		case "not":
			branch.Not = true

		default:
			branch.SelectionKey = s

			// 储存旧条件，并开启新的并列条件
			ok := false
			branch.Selection, ok = selections[branch.SelectionKey]
			if !ok {
				err = UtilsUnexpected.Clonef("id[%d]; selection[%s] is not exist",
					rule.yml.RuleID, branch.SelectionKey)
				return
			}
			branchs = append(branchs, branch)
			branch = SigmaBranch[T]{
				OR:   false,
				rule: rule,
			}
		}
	}
	return
}

// 将单个 selection 转换为 cmd 类型，方便执行
func (c SigmaBranch[T]) toCMD() (fDetection func(m *T) bool, err error) {
	if len(c.Sons) > 0 {
		return c.sonsToCMD()
	}

	if c.SelectionKey == "" {
		err = UtilsUnexpected.Clonef("id[%d]; SelectionKey is empty", c.rule.yml.RuleID)
		return
	}

	// 普通条件： 没有all 函数，条件数量大于1，属于 OR 类型
	if s := c.Selection; !s.All &&
		(len(c.Selection.Strs) > 1 || len(s.Int64s) > 1 || len(s.Bools) > 1) {
		return c.orToCMD()
	}

	// 属于 AND 逻辑
	return c.andToCMD()
}

func (c SigmaBranch[T]) sonsToCMD() (detection func(m *T) bool, err error) {
	if len(c.Sons) == 0 {
		err = UtilsUnexpected.Clonef("[sonsToCMD] sons is empty")
		return
	}
	fs := make([]func(m *T) bool, 0, len(c.Sons))
	for _, c := range c.Sons {
		f, err1 := c.toCMD()
		if err = err1; err != nil {
			return
		}
		fs = append(fs, f)
	}

	or := c.Sons[0].OR
	if or == true {
		detection = func(m *T) (result bool) {
			ok := func() bool {
				for _, f := range fs {
					if f(m) {
						return true
					}
				}
				return false
			}()
			return c.Not != ok
		}
		return
	}

	detection = func(m *T) (result bool) {
		ok := func() bool {
			for _, f := range fs {
				if !f(m) {
					return false
				}
			}
			return true
		}()
		return c.Not != ok
	}

	return
}

func (c SigmaBranch[T]) orToCMD() (detection func(m *T) bool, err error) {
	typ := c.Selection.Ti.BaseKind
	switch typ {
	case reflect.String:
		return c.orToCMDStr()
	case reflect.Int64:
		return c.orToCMDInt64()
	case reflect.Bool:
		return c.orToCMDBool()
	default:
		err = UtilsUnexpected.Clonef("id[%d]; type[%s] not support", c.rule.yml.RuleID, typ.String())
		return
	}
}

func (c SigmaBranch[T]) orToCMDStr() (fDetection func(m *T) bool, err error) {
	selection := c.Selection

	offset := selection.Ti.Offset
	fElem := StructElem[string, T]

	// 没有函数，直接 OR
	if selection.Func == "" {
		fDetection = func(m *T) (result bool) {
			v := fElem(m, offset)
			if v == "" {
				return false
			}
			ok := func() bool {
				for _, str := range selection.Strs {
					if str == v {
						return true // 同一个 selection 下的条件，满足一个即可
					}
				}
				return false
			}()
			return c.Not != ok
		}
		return
	}

	// 长度
	if selection.Func == "len" {
		var nInt64 int64
		for _, str := range selection.Strs {
			nInt64, err = strconv.ParseInt(str, 10, 64)
			if err != nil {
				err = UtilsUnexpected.Clone(err.Error())
			}
			selection.Int64s = append(selection.Int64s, nInt64)
		}

		if len(selection.Int64s) == 1 {
			n := int(selection.Int64s[0])
			fDetection = func(m *T) (result bool) {
				v := fElem(m, offset)
				return c.Not != (len(v) == n)
			}
			return
		}

		fDetection = func(m *T) (result bool) {
			v := fElem(m, offset)
			l := len(v)
			ok := func() bool {
				for _, n := range selection.Int64s {
					if l == int(n) {
						return true
					}
				}
				return false
			}()
			return c.Not != ok
		}
		return
	}

	// 正则表达式
	if selection.Func == "re" {
		// 预计算正则表达式
		res := make([]*regexp.Regexp, len(selection.Strs))
		for i := range res {
			res[i], err = regexpCompile(selection.Strs[i])
			if err != nil {
				err = UtilsUnexpected.Clonef("id[%d]; err:%+v, re:%+v",
					c.rule.yml.RuleID, err, selection.Strs[i])
				return
			}
		}

		fDetection = func(m *T) (result bool) {
			v := fElem(m, offset)
			ok := func() bool {
				for _, re := range res {
					if re.MatchString(v) {
						return true
					}
				}
				return false
			}()
			return c.Not != ok
		}

		return
	}
	if selection.Func == "containsnotmatchcase" {
		funs := make([]func(string) bool, 0, len(selection.Strs))
		for _, substr := range selection.Strs {
			f := kmpContainsNoCaseMaker(substr)
			funs = append(funs, f)
		}
		fDetection = func(m *T) (result bool) {
			v := fElem(m, offset)
			if v == "" {
				return false
			}
			ok := func() bool {
				for _, f := range funs {
					if f(v) {
						return true
					}
				}
				return false
			}()
			return c.Not != ok
		}
		return
	}

	// 普通函数
	f := selection.toRealFunc(selection.Func)
	if f == nil {
		err = UtilsUnexpected.Clonef("rule_id[%d]; func[%s] is not exist", c.rule.yml.RuleID, selection.Func)
		return
	}

	fDetection = func(m *T) (result bool) {
		v := fElem(m, offset)
		if v == "" {
			return false
		}
		ok := func() bool {
			for _, str := range selection.Strs {
				if f(v, str) {
					return true
				}
			}
			return false
		}()
		return c.Not != ok
	}
	return
}

func (c SigmaBranch[T]) orToCMDInt64() (detection func(m *T) bool, err error) {
	selection := c.Selection

	offset := selection.Ti.Offset
	fElem := StructElem[int64, T]

	// 没有函数，直接 OR
	if selection.Func == "" {
		if len(selection.Int64s) == 0 {
			var nInt64 int64
			for _, str := range selection.Strs {
				nInt64 = 0
				if str != "" {
					if strings.HasPrefix(str, "0x") {
						nInt64, err = strconv.ParseInt(str[2:], 16, 64)
					} else {
						nInt64, err = strconv.ParseInt(str, 10, 64)
					}
					if err != nil {
						err = UtilsUnexpected.Clonef("c:%+v, err:%v", c, err.Error())
						return
					}
				}
				selection.Int64s = append(selection.Int64s, nInt64)
			}
		}
		detection = func(m *T) (result bool) {
			v := fElem(m, offset)
			ok := func() bool {
				for _, str := range selection.Int64s {
					if str == v {
						return true
					}
				}
				return false
			}()
			return c.Not != ok
		}
		return
	}

	err = UtilsUnexpected.Clonef("rule_id[%d]; selection key[%s]; integer do not support funcs",
		c.rule.yml.RuleID, c.SelectionKey)
	return
}

func (c SigmaBranch[T]) orToCMDBool() (detection func(m *T) bool, err error) {
	selection := c.Selection

	offset := selection.Ti.Offset
	fElem := StructElem[bool, T]

	// 没有函数，直接 OR
	if selection.Func == "" {
		for _, str := range selection.Strs {
			selection.Bools = append(selection.Bools, str == "true")
		}
		if len(selection.Bools) != 1 {
			err = UtilsUnexpected.Clonef("The bool type supports only one condition")
			return
		}
		b := selection.Strs[0] == "true"
		selection.Bools = append(selection.Bools, b)

		detection = func(m *T) (result bool) {
			v := fElem(m, offset)
			return c.Not != (b == v)
		}
		return
	}

	err = UtilsUnexpected.Clonef("rule_id[%d]; selection key[%s]; integer do not support funcs",
		c.rule.yml.RuleID, c.SelectionKey)
	return
}

func (c SigmaBranch[T]) andToCMD() (detection func(m *T) bool, err error) {
	selection := c.Selection

	typ := selection.Ti.BaseKind
	switch typ {
	case reflect.String:
		return c.andToCMDStr()
	case reflect.Int64:
		return c.andToCMDInt64()
	case reflect.Bool:
		return c.andToCMDBool()
	default:
		err = UtilsUnexpected.Clonef("type not support:%s", selection.Ti.BaseType.String())
		return
	}
}

func (c SigmaBranch[T]) andToCMDStr() (fDetection func(m *T) bool, err error) {
	selection := c.Selection

	offset := selection.Ti.Offset
	fElem := StructElem[string, T]

	if selection.Func == "" {
		if len(selection.Strs) == 1 {
			fDetection = func(m *T) (result bool) {
				v := fElem(m, offset)
				if v == "" {
					return false
				}
				return c.Not != (selection.Strs[0] == v)
			}
			return
		}

		fDetection = func(m *T) (result bool) {
			v := fElem(m, offset)
			if v == "" {
				return false
			}
			ok := func() bool {
				for _, str := range selection.Strs {
					if str != v {
						return false
					}
				}
				return true
			}()
			return c.Not != ok
		}
		return
	}

	// 长度
	if selection.Func == "len" {
		var nInt64 int64
		for _, str := range selection.Strs {
			nInt64, err = strconv.ParseInt(str, 10, 64)
			if err != nil {
				err = UtilsUnexpected.Clone(err.Error())
			}
			selection.Int64s = append(selection.Int64s, nInt64)
		}

		if len(selection.Int64s) == 1 {
			n := int(selection.Int64s[0])
			fDetection = func(m *T) (result bool) {
				v := fElem(m, offset)
				return c.Not != (len(v) == n)
			}
			return
		}

		// 这个是有问题的 len 不能同事等于不同值
		// err = UtilsUnexpected.Clonef("len ")

		fDetection = func(m *T) (result bool) {
			return c.Not != false
		}
		return
	}

	// 正则表达式
	if selection.Func == "re" {
		// 预计算正则表达式
		res := make([]*regexp.Regexp, len(selection.Strs))
		for i := range res {
			res[i], err = regexpCompile(selection.Strs[i])
			if err != nil {
				err = UtilsUnexpected.Clonef("ruleid[%d] ;err:%+v, obj:%+v", c.rule.yml.RuleID, err, selection)
				return
			}
		}

		if len(selection.Strs) == 1 {
			fDetection = func(m *T) (result bool) {
				v := fElem(m, offset)
				if v == "" {
					return false
				}
				return c.Not != res[0].MatchString(v)
			}
			return
		}

		fDetection = func(m *T) (result bool) {
			v := fElem(m, offset)
			if v == "" {
				return false
			}
			ok := func() bool {
				for _, re := range res {
					if !re.MatchString(v) {
						return false
					}
				}
				return true
			}()
			return c.Not != ok
		}
		return
	}

	if selection.Func == "containsnotmatchcase" {
		funs := make([]func(string) bool, 0, len(selection.Strs))
		for _, substr := range selection.Strs {
			f := kmpContainsNoCaseMaker(substr)
			funs = append(funs, f)
		}
		fDetection = func(m *T) (result bool) {
			v := fElem(m, offset)
			if v == "" {
				return false
			}
			ok := func() bool {
				for _, f := range funs {
					if !f(v) {
						return false
					}
				}
				return true
			}()
			return c.Not != ok
		}
		return
	}

	// 普通函数
	f := selection.toRealFunc(selection.Func)
	if f == nil {
		err = UtilsUnexpected.Clonef("rule_id[%d]; func[%s] is not exist", c.rule.yml.RuleID, selection.Func)
		return
	}

	if len(selection.Strs) == 1 {
		fDetection = func(m *T) (result bool) {
			v := fElem(m, offset)
			if v == "" {
				return false
			}
			return c.Not != f(v, selection.Strs[0])
		}
		return
	}

	fDetection = func(m *T) (result bool) {
		v := fElem(m, offset)
		if v == "" {
			return false
		}
		ok := func() bool {
			for _, str := range selection.Strs {
				if !f(v, str) {
					return false
				}
			}
			return true
		}()
		return c.Not != ok
	}
	return
}

func parseToInt64(str, info string, ruleID int64) (nInt64 int64, err error) {
	switch str {
	case "true":
		fallthrough
	case `"true"`:
		fallthrough
	case `'true'`:
		nInt64 = 2
		return
	case "false":
		fallthrough
	case `"false"`:
		fallthrough
	case `'false'`:
		nInt64 = 1
		return
	}
	if strings.HasPrefix(str, "0x") {
		nInt64, err = strconv.ParseInt(str[2:], 16, 64)
	} else {
		nInt64, err = strconv.ParseInt(str, 10, 64)
	}
	if err != nil {
		err = UtilsUnexpected.Clonef("rule_id[%d]; str:%+v, err:%+v", ruleID, info, err)
		return
	}
	return
}

func (c SigmaBranch[T]) andToCMDInt64() (detection func(m *T) bool, err error) {
	selection := c.Selection

	offset := selection.Ti.Offset
	fElem := StructElem[int64, T]

	if selection.Func == "" {
		if len(selection.Int64s) == 0 {
			var nInt64 int64
			for _, str := range selection.Strs {
				nInt64 = 0
				if str != "" {
					nInt64, err = parseToInt64(str, str, c.rule.yml.RuleID)
					if err != nil {
						return
					}
				}
				selection.Int64s = append(selection.Int64s, nInt64)
			}
		}
		if len(selection.Int64s) == 1 {
			n := selection.Int64s[0]
			detection = func(m *T) (result bool) {
				v := fElem(m, offset)
				return c.Not != (n == v)
			}
			return
		}

		detection = func(m *T) (result bool) {
			v := fElem(m, offset)
			if v == 0 {
				return false
			}
			ok := func() bool {
				for _, str := range selection.Int64s {
					if str != v {
						return false
					}
				}
				return true
			}()
			return c.Not != ok
		}
		return
	}

	err = UtilsUnexpected.Clonef("rule_id[%d]; selection key[%s]; integer do not support funcs",
		c.rule.yml.RuleID, c.SelectionKey)
	return
}

func (c SigmaBranch[T]) andToCMDBool() (detection func(m *T) bool, err error) {
	selection := c.Selection

	offset := selection.Ti.Offset
	fElem := StructElem[bool, T]

	if selection.Func == "" {
		for _, str := range selection.Strs {
			selection.Bools = append(selection.Bools, str == "true")
		}
		if len(selection.Bools) != 1 {
			err = UtilsUnexpected.Clonef("The bool type supports only one condition")
			return
		}
		b := selection.Strs[0] == "true"

		detection = func(m *T) (result bool) {
			v := fElem(m, offset)
			return c.Not != (b == v)
		}
		return
	}

	err = UtilsUnexpected.Clonef("rule_id[%d]; selection key[%s]; integer do not support funcs",
		c.rule.yml.RuleID, c.SelectionKey)
	return
}
