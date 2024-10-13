package engine

import (
	"testing"
	"time"

	"github.com/lxt1045/utils/tag"
	"github.com/stretchr/testify/assert"
)

func Test_parseTimeWindow(t *testing.T) {
	t.Run("sec", func(t *testing.T) {
		ns, err := parseTimeWindow("5sec", 0)
		assert.Nil(t, err, "")
		assert.Equal(t, ns, int64(time.Second*5))
	})
	t.Run("min", func(t *testing.T) {
		ns, err := parseTimeWindow("5 min", 0)
		assert.Nil(t, err, "")
		assert.Equal(t, ns, int64(time.Minute*5))
	})
	t.Run("hour", func(t *testing.T) {
		ns, err := parseTimeWindow("5 hour", 0)
		assert.Nil(t, err, "")
		assert.Equal(t, ns, int64(time.Hour*5))
	})
}

func Test_addHavingEqual(t *testing.T) {
	type Log struct {
		A string
		B int64
		C bool
	}
	tis, err := tag.NewTagInfos(Log{}, "json")
	assert.Nil(t, err, "")
	ruleIDL, ruleIDR := int64(111), int64(112)

	t.Run("string", func(t *testing.T) {
		rule := &streamRule[Log]{}
		elemL := HavingElem{
			ti:     tagToInfo("A", tis),
			RuleID: ruleIDL,
		}
		elemR := HavingElem{
			ti:     tagToInfo("A", tis),
			RuleID: ruleIDR,
		}
		err = addHavingEqual[string](rule, 0, elemL, elemR)
		assert.Nil(t, err, "")

		// todo
		f := rule.preFuncs[0]

		mL := &Log{
			A: "lll",
		}
		mR := &Log{
			A: "rrr",
		}
		g := &Group[Log]{
			counts: make([]int32, 1),
			havings: []having[Log]{{
				leftMap:  make(map[string]data[Log]),
				rightMap: make(map[string]data[Log]),
			}},
		}

		t.Run("1", func(t *testing.T) {
			f(eventData[Log]{
				singleRuleID: 1,
				idxCounts:    0,
				data:         mL,
				group:        g,
			})
			assert.Equal(t, len(g.havings[0].leftMap), 0)
			assert.Equal(t, len(g.havings[0].rightMap), 0)
			pair := g.havings[0].pair.Load()
			assert.Nil(t, pair)
		})
		t.Run("2", func(t *testing.T) {
			f(eventData[Log]{
				singleRuleID: ruleIDL,
				idxCounts:    0,
				data:         mL,
				group:        g,
			})
			assert.Equal(t, len(g.havings[0].leftMap), 1)
			assert.Equal(t, len(g.havings[0].rightMap), 0)
			pair := g.havings[0].pair.Load()
			assert.Nil(t, pair)
			assert.Equal(t, g.havings[0].leftMap[mL.A].data, mL)
		})
		t.Run("3", func(t *testing.T) {
			f(eventData[Log]{
				singleRuleID: ruleIDR,
				idxCounts:    0,
				data:         mR,
				group:        g,
			})
			assert.Equal(t, len(g.havings[0].leftMap), 1)
			assert.Equal(t, len(g.havings[0].rightMap), 1)
			pair := g.havings[0].pair.Load()
			assert.Nil(t, pair)
			assert.Equal(t, g.havings[0].rightMap[mR.A].data, mR)
		})

		t.Run("4", func(t *testing.T) {
			mLR := &(*mL)
			f(eventData[Log]{
				singleRuleID: ruleIDR,
				idxCounts:    0,
				data:         mLR,
				group:        g,
			})
			assert.Equal(t, len(g.havings[0].leftMap), 1)
			assert.Equal(t, len(g.havings[0].rightMap), 2)
			pair := g.havings[0].pair.Load()
			assert.NotNil(t, pair)
			assert.Equal(t, pair.left, mL)
			assert.Equal(t, pair.right, mLR)
			assert.Equal(t, g.havings[0].rightMap[mR.A].data, mR)
			assert.Equal(t, g.havings[0].rightMap[mLR.A].data, mLR)
		})
	})
}
