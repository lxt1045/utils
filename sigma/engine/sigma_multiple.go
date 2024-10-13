package engine

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync/atomic"
	"time"

	"github.com/lxt1045/utils/delay"
	"github.com/lxt1045/utils/gid"
	"github.com/lxt1045/utils/log"
	"github.com/lxt1045/utils/tag"
	"gopkg.in/yaml.v2"
)

func NewRulesetMulByDir[T any](ctx context.Context, dir string, in T) (rs *StreamRuleset[T], err error) {
	ymls := [][]byte{}
	err = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if info.IsDir() || (!strings.HasSuffix(path, ".yml") && !strings.HasSuffix(path, ".yaml")) {
			return nil
		}
		bs, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		ymls = append(ymls, bs)
		return nil
	})
	if err != nil {
		return
	}

	return NewRulesetMul(ctx, ymls, in)
}

func NewOneRulesetMul[T any](ctx context.Context, bs []byte, in T) (rs *StreamRuleset[T], err error) {
	return NewRulesetMul(ctx, [][]byte{bs}, in)
}

func NewRulesetMul[T any](ctx context.Context, ymls [][]byte, in T) (ruleset *StreamRuleset[T], err error) {
	rs := &StreamRuleset[T]{}

	rs.tis, err = tag.NewTagInfos(&in, "json")
	if err != nil {
		return
	}

	for _, str := range ymls {
		err = rs.addOneRule([]byte(str))
		if err != nil {
			return
		}
	}

	err = rs.Build()
	if err != nil {
		return
	}

	ruleset = rs
	return
}

func (s *StreamRuleset[T]) addOneRule(bs []byte) (err error) {
	var yml SigmaYmlMul
	err = yaml.Unmarshal(bs, &yml)
	if err != nil {
		if len(bs) > 120 {
			bs = bs[:120]
		}
		err = UtilsUnexpected.Clonef("yaml error:%v, yml:%s ...", err, string(bs))
		return
	}

	/*
		有存储的流式计算：
		select '2' as rule_id, '特殊软件行为' as rule_name, count(rul_id) as count,
		  'MEDIUM' as seriousness,'image' as techniqueType, collect(rule_is) as ruleid_list,
		  collect(log_id,cuuid) as log_list,
		from inputStream
		where (rule_id=='1063' or rule_id=='2099' or rule_id=='1701' or rule_id=='1604')
		  AND window.time = 5sec
		group by cuuid
		having have_equal(rule2.event_data_Image,rule4.event_data_Image),  // 存在满足条件的数据
		  contains_all(ruleid_list,[1063,2099,1701,1604]) // 或 in_order(ruleid_list,[1063,2099,1701,1604])
	*/

	rule := &streamRule[T]{
		ruleset:    s,
		yml:        &yml,
		groups:     make(map[string]*Group[T]),
		mCountsIdx: make(map[int64]int),
	}
	rule.timeWindow, err = parseTimeWindow(yml.Detection.Condition.TimeWindow, yml.RuleID)
	if err != nil {
		return
	}
	rule.delayQueue = delay.New[eventData[T]](1024*1024, rule.timeWindow, false)
	// rule.delayQueue = delay.New[eventData[T]](0, rule.timeWindow)

	if len(yml.Detection.Selection) == 0 {
		err = UtilsUnexpected.Clone("rule_id[%d]; selection is empty")
		log.Ctx(context.TODO()).Error().Caller().Err(err).Msg("rule")
		err = nil
		return
	}

	for key, ruleID := range yml.Detection.Selection {
		countLimit := yml.Detection.Condition.CountLimits[key]
		rule.mCountsIdx[ruleID] = len(rule.countLimits)
		rule.countLimits = append(rule.countLimits, int32(countLimit))
	}

	if groupKeys := yml.Detection.Condition.GroupBys; len(groupKeys) > 0 {
		for _, key := range groupKeys {
			ti := s.tis[key]
			if ti == nil {
				err = UtilsUnexpected.Clonef("group key[%s] not found")
				return
			}
			gGetElem, e := structElemToStrFunc[T](ti)
			if e != nil {
				err = e
				return
			}
			if rule.fGetGroupKey == nil {
				rule.fGetGroupKey = gGetElem
			} else {
				f0 := rule.fGetGroupKey
				rule.fGetGroupKey = func(m *T) (str string) {
					return f0(m) + ":" + gGetElem(m)
				}
			}
		}
	}

	if havings := yml.Detection.Condition.Havings; len(havings) > 0 {
		rule.nHavingConditions = len(havings)
		for i, having := range havings {
			err = rule.addHaving(i, having)
			if err != nil {
				return
			}
		}
	}

	if countLimits := yml.Detection.Condition.CountLimits; len(countLimits) > 0 {
		needCount := 0
		for _, c := range countLimits {
			if c > 0 {
				needCount += c
				break
			}
		}
		if needCount > 1 || (needCount == 1 && len(yml.Detection.Selection) > 1) {
			rule.preFuncs = append(rule.preFuncs, func(d eventData[T]) {
				atomic.AddInt32(&d.group.counts[d.idxCounts], 1)
			})
			rule.postFuncs = append(rule.postFuncs, func(d eventData[T]) {
				atomic.AddInt32(&d.group.counts[d.idxCounts], -1)
			})
		} else {
			rule.countLimits = nil // 没有配置统计
		}
	}

	rule.mainRuleID = yml.Detection.Selection[yml.Detection.Object]
	if rule.mainRuleID == 0 && (len(rule.preFuncs) > 0 || len(rule.postFuncs) > 0) {
		err = UtilsUnexpected.Clonef("rule_id[%d]; detection.object not exist: %s", yml.RuleID, yml.Detection.Object)
		return
	}

	s.rules = append(s.rules, rule)
	return
}

// addHaving 增加 having 条件
// Having 条件计算一次就可以多次使用，所以需要标记到event中，event进入和退出时修改全局的 heving状态！！！
func (s *streamRule[T]) addHaving(idxHaving int, str string) (err error) {
	parseHavingKey := func(str string) (elem HavingElem, err error) {
		vs := strings.SplitN(str, ".", 2)
		if len(vs) != 2 {
			err = UtilsUnexpected.Clonef("rule_id[%d]; has no . : %s", s.yml.RuleID, str)
			return
		}
		ok := false
		rule := strings.TrimSpace(vs[0])
		elem.Key = strings.TrimSpace(vs[1])
		elem.RuleID, ok = s.yml.Detection.Selection[rule]
		if !ok {
			err = UtilsUnexpected.Clonef("rule_id[%d]; having condition error: %s", s.yml.RuleID, str)
			return
		}
		elem.ti = tagToInfo(elem.Key, s.ruleset.tis)
		if elem.ti == nil {
			in := new(T)
			err = UtilsUnexpected.Clonef("rule_id[%d]; key[%s] is not exist in %+T", s.yml.RuleID, elem.Key, *in)
			return
		}
		return
	}

	fToHavings := func(str string) (havings []string, err error) {
		str = strings.TrimSpace(str)
		if len(str) < 2 || str[0] != '(' || str[len(str)-1] != ')' {
			err = UtilsUnexpected.Clonef("rule_id[%d]; having is error[%s]", s.yml.RuleID, str)
			return
		}
		str = str[1 : len(str)-1]
		havings = strings.Split(str, ",")
		return
	}

	var fMaker func(sub, str string) bool
	var havings []string
	switch {
	case strings.HasPrefix(str, "equalnotmatchcase"):
		fMaker = func(sub, str string) bool {
			if len(sub) != len(str) {
				return false
			}
			// strings.ToLower()
			return strings.EqualFold(sub, str)
		}
		str1 := str[len("equalnotmatchcase"):]
		havings, err = fToHavings(str1)
		if err != nil {
			return
		}
	case strings.HasPrefix(str, "equal"):
		fMaker = func(sub, str string) bool {
			return sub == str
		}
		str1 := str[len("equal"):]
		havings, err = fToHavings(str1)
		if err != nil {
			return
		}
	case strings.HasPrefix(str, "containsnotmatchcase"):
	case strings.HasPrefix(str, "contains"):
	case strings.HasPrefix(str, "startswithnotmatchcase"):
	case strings.HasPrefix(str, "startswith"):
	case strings.HasPrefix(str, "endswithnotmatchcase"):
	case strings.HasPrefix(str, "endswith"):
	}

	if fMaker == nil {
		havings = strings.Split(str, "=") // 相等规则
	}
	if len(havings) == 2 {
		left, right := strings.TrimSpace(havings[0]), strings.TrimSpace(havings[1])
		havingElemL, err1 := parseHavingKey(left)
		if err = err1; err != nil {
			return
		}
		havingElemR, err1 := parseHavingKey(right)
		if err = err1; err != nil {
			return
		}
		if havingElemL.ti.BaseKind != havingElemR.ti.BaseKind {
			err = UtilsUnexpected.Clonef("rule_id[%d]; tiL.BaseKind[%v] != tiR.BaseKind[%v]",
				s.yml.RuleID, havingElemL.ti.BaseKind.String(), havingElemR.ti.BaseKind.String())
			return
		}
		switch havingElemL.ti.BaseKind {
		case reflect.String:
			err = addHavingEqual[string](s, idxHaving, havingElemL, havingElemR)
		case reflect.Int64:
			err = addHavingEqual[int64](s, idxHaving, havingElemL, havingElemR)
		case reflect.Bool:
			err = addHavingEqual[bool](s, idxHaving, havingElemL, havingElemR)
		default:
			err = UtilsUnexpected.Clonef("rule_id[%d]; tiL.BaseKind[%v] is not support",
				s.yml.RuleID, havingElemL.ti.BaseKind.String())
			return
		}
		return
	}

	err = UtilsUnexpected.Clonef("rule_id[%d]; having error: %s", s.yml.RuleID, str)
	return
}

func addHavingEqual[E string | int64 | bool, T any](s *streamRule[T], idxHaving int, elemL, elemR HavingElem) (err error) {
	fGetElemL, err := structElemToStrFunc[T](elemL.ti)
	if err != nil {
		return
	}
	fGetElemR, err := structElemToStrFunc[T](elemR.ti)
	if err != nil {
		return
	}

	lRuleID32 := elemL.RuleID
	rRuleID32 := elemR.RuleID
	preFunc := func(d eventData[T]) {
		ruleID, m, g := d.singleRuleID, d.data, d.group
		if ruleID == lRuleID32 {
			v := fGetElemL(m)
			having := &g.havings[idxHaving]
			d := data[T]{
				start: time.Now().UnixNano(),
				data:  m,
			}

			having.leftLock.Lock()   //
			having.leftMap[v] = d    // 新增
			having.leftLock.Unlock() //

			having.rightLock.RLock()    //
			value := having.rightMap[v] //
			having.rightLock.RUnlock()  //
			if value.data == nil {
				return
			}

			newPair := havingEqualPair[T]{
				left:  m,
				right: value.data,
				start: value.start, // 这个时间肯定是获取到的更早，毋庸置疑
			}
			for {
				lastPair := having.pair.Load()

				// 如果不是最新的命中者，不用替换
				if lastPair != nil && lastPair.start > value.start {
					break
				}

				swapped := having.pair.CompareAndSwap(lastPair, &newPair)
				if swapped {
					break
				}
			}
			return
		}
		if ruleID == rRuleID32 {
			v := fGetElemR(m)
			having := &g.havings[idxHaving]
			d := data[T]{
				start: time.Now().UnixNano(),
				data:  m,
			}

			having.rightLock.Lock()   //
			having.rightMap[v] = d    // 新增
			having.rightLock.Unlock() //

			having.leftLock.RLock()    //
			value := having.leftMap[v] //
			having.leftLock.RUnlock()  //
			if value.data == nil {
				return
			}

			newPair := havingEqualPair[T]{
				right: m,
				left:  value.data,
				start: value.start, // 这个时间肯定是获取到的更早，毋庸置疑
			}
			for {
				lastPair := having.pair.Load()

				// 如果不是最新的命中者，不用替换
				if lastPair != nil && lastPair.start > value.start {
					break
				}

				swapped := having.pair.CompareAndSwap(lastPair, &newPair)
				if swapped {
					break
				}
			}
			return
		}
	}
	s.preFuncs = append(s.preFuncs, preFunc)

	postFunc := func(d eventData[T]) {
		ruleID, m, g := d.singleRuleID, d.data, d.group
		if ruleID == lRuleID32 {
			v := fGetElemL(m)
			having := &g.havings[idxHaving]

			having.leftLock.Lock()
			if having.leftMap[v].data == m {
				delete(having.leftMap, v)
			}
			having.leftLock.Unlock() //

			lastPair := having.pair.Load()
			if lastPair == nil || d.data != lastPair.left {
				return // 如果不是最新的命中者，不用处理
			}
			newPair := havingEqualPair[T]{}
			swapped := having.pair.CompareAndSwap(lastPair, &newPair)
			if swapped {
			}
			return
		}
		if ruleID == rRuleID32 {
			v := fGetElemR(m)
			having := &g.havings[idxHaving]

			having.rightLock.Lock()
			if having.rightMap[v].data == m {
				delete(having.rightMap, v)
			}
			having.rightLock.Unlock() //

			lastPair := having.pair.Load()
			if lastPair == nil || d.data != lastPair.right {
				return // 如果不是最新的命中者，不用处理
			}
			newPair := havingEqualPair[T]{}
			swapped := having.pair.CompareAndSwap(lastPair, &newPair)
			if swapped {
			}
			return
		}
	}
	s.postFuncs = append(s.postFuncs, postFunc)
	return
}

func (rule *streamRule[T]) ToEvalFunc(singleRuleID int64) (fEval func(m *T) []DataHit[T], err error) {
	idxCounts, ok := rule.mCountsIdx[singleRuleID]
	if !ok {
		return
	}
	singleRuleID64 := singleRuleID
	mulRuleID64 := rule.yml.RuleID
	mainRuleID64 := rule.mainRuleID
	score := int16(rule.yml.Initial)

	// 特殊规则: 单条即是多条
	if len(rule.preFuncs) == 0 && len(rule.postFuncs) == 0 {
		fEval = func(m *T) (hits []DataHit[T]) {
			g := rule.GetGroup(m)
			lastEventID := atomic.LoadInt64(&g.lastEventID)
			var mainRuleID int64

			// 没有 count 统计条件，也没有 having 限制条件，纯粹的合并成一个单独的事件
			if rule.timeWindow > 0 {
				ts := gid.GIDToTs(lastEventID)
				if ts+rule.timeWindow/int64(time.Second) < gid.GetTsNow() {
					newEventID := gid.GetGID()
					swaped := atomic.CompareAndSwapInt64(&g.lastEventID, lastEventID, newEventID)
					if swaped {
						mainRuleID = singleRuleID64
						rule.ruleset.allLastEventIDAdddr[newEventID] = &g.lastEventID
						if lastEventID > 0 {
							delete(rule.ruleset.allLastEventIDAdddr, lastEventID)
						}
					}
					lastEventID = atomic.LoadInt64(&g.lastEventID)
				}
			} else if lastEventID == 0 {
				newEventID := gid.GetGID()
				swaped := atomic.CompareAndSwapInt64(&g.lastEventID, 0, newEventID)
				if swaped {
					mainRuleID = singleRuleID64
					rule.ruleset.allLastEventIDAdddr[newEventID] = &g.lastEventID
				}
				lastEventID = atomic.LoadInt64(&g.lastEventID)
			}
			hits = append(hits, DataHit[T]{
				MulRuleID:    mulRuleID64,
				SingleRuleID: singleRuleID64,
				MainRuleID:   mainRuleID,
				EventID:      lastEventID,
				Data:         m,
				Score:        score,
				RuleType:     RuleTypeMerger,
			})
			return
		}

		return
	}

	// 返回命中的事件列表
	fEval = func(m *T) (hits []DataHit[T]) {
		// if g == nil {
		// 	return
		// }

		// 1. TODO 对排序怎么处理？
		/*
			排序思路，加n个 cache， 每个对应 rule的排序level，当该level 来最新值时，
			将依赖的上一个level的最基础level的时间（肯定是最早进入的）记下。如此，
			每个level记下的都是排序到本level的最新序列，也就是可以持续最长时间存在的序列！！！
		*/

		// 2. 缺失规则实现：
		/*
			比如 rule1,rule2,rule3,rule4 , 已经出现了三个 rule1,rule2,rule4。
			此时命中缺少一项的规则，然后把作为事件丢进延时队列；如果时间窗口(5s、10s)内，
			rule3 出现，就把队列中的该事件标记为失效；如果没出现，则延时到期后输出为事件。
		*/

		/*
			此处，可能 rule.GetGroup(m) 会无限增大，所以需要在 group_by 包含出 agent_id 以外的其他字段时，
			需要用 delay 表延迟 time_windows 的事件后，再删除 group_by 避免无限膨胀
		*/
		g := rule.GetGroup(m)
		lastEventID := atomic.LoadInt64(&g.lastEventID)

		dataDelay := eventData[T]{
			singleRuleID: singleRuleID64, // 单条规则的 rule_id
			idxCounts:    idxCounts,
			data:         m,
			group:        g,
		}

		// 3. 对 having 处理
		for _, preFunc := range rule.preFuncs {
			preFunc(dataDelay)
		}

		// 必要的处理完成后，加入延时队列
		rule.delayQueue.Push(dataDelay)

		// 统计数量；不通过就直接返回吧
		for i, n := range g.counts {
			if n < rule.countLimits[i] {
				atomic.CompareAndSwapInt64(&g.lastEventID, lastEventID, 0)
				return
			}
		}

		// 统计 having 条件
		for i := range g.havings {
			pair := g.havings[i].pair.Load()
			if pair == nil || pair.start == 0 {
				// 没有命中 或者 超过时间窗口；把上一个 eventID 清除
				atomic.CompareAndSwapInt64(&g.lastEventID, lastEventID, 0)
				return
			}
		}

		// 仅输出当前 *T,并使用上次的 event_id
		if lastEventID > 0 {
			hits = append(hits, DataHit[T]{
				MulRuleID:    mulRuleID64,
				SingleRuleID: singleRuleID64,
				EventID:      lastEventID,
				Data:         m,
				Score:        score,
				RuleType:     RuleTypeMultiple,
				// MainRuleID:   mainRuleID64, // 后续依赖此值生成event，这个分支不需要生成新事件，所以不返回
			})
			return
		}

		// 输出 group 队列中的所有数据，并生成新的event_id
		newEventID := gid.GetGID()
		swapped := atomic.CompareAndSwapInt64(&g.lastEventID, 0, newEventID)
		if !swapped {
			newEventID1 := atomic.LoadInt64(&g.lastEventID)
			if newEventID1 != 0 {
				newEventID = newEventID1 // 得保证 newEventID 不为 0
			}
		}
		// atomic.StoreInt64(&g.lastEventID, newEventID)
		rule.delayQueue.Range(func(m eventData[T]) {
			// 同一个 group 才输出
			if m.group == g {
				hits = append(hits, DataHit[T]{
					MulRuleID:    mulRuleID64,
					SingleRuleID: m.singleRuleID,
					EventID:      newEventID,
					Data:         m.data,
					MainRuleID:   mainRuleID64, //
					Score:        score,
					RuleType:     RuleTypeMultiple,
				})
			}
		})
		return
	}

	return
}
