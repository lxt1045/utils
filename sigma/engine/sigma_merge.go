package engine

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/lxt1045/utils/gid"
	"github.com/lxt1045/utils/log"
	"github.com/lxt1045/utils/tag"
	"gopkg.in/yaml.v2"
)

func NewRulesetMergeByDir[T any](ctx context.Context, dir string, in T) (rs *MergeRuleset[T], err error) {
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

	return NewRulesetMerge(ctx, ymls, in)
}

func NewOneRulesetMerge[T any](ctx context.Context, bs []byte, in T) (rs *MergeRuleset[T], err error) {
	return NewRulesetMerge(ctx, [][]byte{bs}, in)
}

func NewRulesetMerge[T any](ctx context.Context, ymls [][]byte, in T) (ruleset *MergeRuleset[T], err error) {
	rs := &MergeRuleset[T]{}

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

func (s *MergeRuleset[T]) addOneRule(bs []byte) (err error) {
	var yml SigmaYmlMerge
	err = yaml.Unmarshal(bs, &yml)
	if err != nil {
		if len(bs) > 120 {
			bs = bs[:120]
		}
		err = UtilsUnexpected.Clonef("yaml error:%v, yml:%s ...", err, string(bs))
		return
	}

	if yml.Type != "merger" {
		log.Ctx(context.TODO()).Error().Caller().
			Err(UtilsUnexpected.Clonef("yaml type err:%s, yml:%s ...", yml.Type)).Msg("MergeRuleset")
		return
	}

	rule := &mergeRule[T]{
		ruleset: s,
		yml:     &yml,
	}
	rule.timeWindow, err = parseTimeWindow(yml.Detection.Condition.TimeWindow, yml.RuleID)
	if err != nil {
		return
	}
	if len(yml.Detection.Selection) == 0 {
		log.Ctx(context.TODO()).Error().Caller().
			Err(UtilsUnexpected.Clone("rule_id[%d]; selection is empty")).Msg("rule")
		return
	}

	groupKeys := yml.Detection.Condition.GroupBys
	fs := make([]func(*T) string, 0, 2)
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
		fs = append(fs, gGetElem)
	}

	keyPre := strconv.FormatInt(yml.RuleID, 10)
	switch len(fs) {
	case 0:
		rule.fGetGroupKey = func(*T) string {
			return keyPre
		}
	case 1:
		rule.fGetGroupKey = func(l *T) string {
			return keyPre + ":" + fs[0](l)
		}
	case 2:
		rule.fGetGroupKey = func(l *T) string {
			return keyPre + ":" + fs[0](l) + ":" + fs[1](l)
		}
	case 3:
		rule.fGetGroupKey = func(l *T) string {
			return keyPre + ":" + fs[0](l) + ":" + fs[1](l) + ":" + fs[2](l)
		}
	case 4:
		rule.fGetGroupKey = func(l *T) string {
			return keyPre + ":" + fs[0](l) + ":" + fs[1](l) + ":" + fs[2](l) + ":" + fs[3](l)
		}
	default:
		// 大于 4 就不做循环展开了，小概率事件没必要
		rule.fGetGroupKey = func(l *T) (key string) {
			key = keyPre
			for _, f := range fs {
				key += ":" + f(l)
			}
			return
		}
	}

	s.rules = append(s.rules, rule)
	return
}

func (rule *mergeRule[T]) ToEvalFunc(singleRuleID int64) (fEval func(m *T) []DataHit[T], err error) {
	singleRuleID64 := singleRuleID
	mergeRuleID64 := rule.yml.RuleID
	score := int16(rule.yml.Initial)

	fEval = func(m *T) (hits []DataHit[T]) {
		newEventID, tsNow := gid.GetGID(), time.Now().UnixNano()
		g := rule.MustGetGroup(m, newEventID, tsNow)
		lastEventID := atomic.LoadInt64(&g.lastEventID)
		var mainRuleID int64

		// 没有 count 统计条件，也没有 having 限制条件，纯粹的合并成一个单独的事件
		if rule.timeWindow > 0 {
			ts := gid.GIDToTs(lastEventID)
			if ts+rule.timeWindow/int64(time.Second) < gid.GetTsNow() {
				swaped := atomic.CompareAndSwapInt64(&g.lastEventID, lastEventID, gid.GetGID())
				if swaped {
					mainRuleID = singleRuleID64
				}
				lastEventID = atomic.LoadInt64(&g.lastEventID)
			}
		} else if lastEventID == 0 {
			swaped := atomic.CompareAndSwapInt64(&g.lastEventID, 0, gid.GetGID())
			if swaped {
				mainRuleID = singleRuleID64
			}
			lastEventID = atomic.LoadInt64(&g.lastEventID)
		}
		hits = append(hits, DataHit[T]{
			MulRuleID:    mergeRuleID64,
			SingleRuleID: singleRuleID64,
			MainRuleID:   mainRuleID,
			EventID:      lastEventID,
			Data:         m,
			Score:        score,
		})
		return
	}

	return
}
