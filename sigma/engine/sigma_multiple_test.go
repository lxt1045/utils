package engine

import (
	"context"
	"encoding/json"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/bytedance/mockey"
	"github.com/stretchr/testify/assert"
)

func Test_OneSigmaMul(t *testing.T) {
	sigmaYml, err := os.ReadFile("D:/project/go/src/github.com/lxt1045/sigma_rule/right_mul/101001202001.yml")
	assert.Nil(t, err)

	ctx := context.TODO()
	rs, err := NewOneRulesetMul(ctx, []byte(sigmaYml), Log{})
	if err != nil {
		t.Fatal(err)
	}
	input := Log{
		AgentId:     88,
		EventId:     1,
		ImageLoaded: "1234567890",
	}
	ruleIDs := rs.Eval(1001201888, &input)

	bs, _ := json.Marshal(&ruleIDs)
	t.Logf("results:%+v", string(bs))
	assert.Equal(t, len(ruleIDs), 1)

	t.Log("end...")
}

func Test_sigmaMul2(t *testing.T) {
	tNow, err := time.Parse(time.RFC3339, "2023-10-02T00:00:00Z")
	if err != nil {
		t.Fatal(err)
	}
	pNow := &tNow
	// fUnPatch := mockey.Mock(time.Now).Return(tNow).Build().UnPatch
	defer mockey.Mock(time.Now).To(func() time.Time { return *pNow }).Build().UnPatch()

	sigmaYml := `title: 特殊进程行为--创建管道并发起网络连接
id: 345
status: 1
technique_type: 反弹shell
description: 待补充
seriousness: MEDIUM
platform: ""
initial: 90
detection:
  object: rule1
  ordered: false
  selection:
    rule1: 1
    rule2: 2
  condition:
    time_window: 5sec
    group_by:
    - agent_id
    having:
    - rule1.pid=rule2.pid
    count: {}
`

	ctx := context.TODO()
	rs, err := NewOneRulesetMul(ctx, []byte(sigmaYml), Log{})
	if err != nil {
		t.Fatal(err)
	}
	{
		input := Log{AgentId: 88, EventId: 1, Pid: 111}
		ruleIDs := rs.Eval(1, &input)
		assert.Equal(t, len(ruleIDs), 0)
	}
	*pNow = pNow.Add(time.Second)
	{
		input := Log{AgentId: 88, EventId: 2, Pid: 111}
		ruleIDs := rs.Eval(2, &input)
		assert.Equal(t, len(ruleIDs), 1)
		lastEventID := ruleIDs[0][0].EventID
		n := 0
		for _, c := range ruleIDs {
			for _, x := range c {
				if x.EventID != lastEventID {
					t.Fatal("has 2 lastEventID")
				}
				n++
			}
		}
		assert.Equal(t, n, 2) // 总共4个事件
	}
	*pNow = pNow.Add(time.Second * 6)
	{
		input := Log{AgentId: 88, EventId: 1, Pid: 111}
		ruleIDs := rs.Eval(1, &input)
		assert.Equal(t, len(ruleIDs), 0)
	}

	t.Log("end...")
}

func Test_sigmaMul(t *testing.T) {
	tNow, err := time.Parse(time.RFC3339, "2023-10-02T00:00:00Z")
	if err != nil {
		t.Fatal(err)
	}
	pNow := &tNow
	// fUnPatch := mockey.Mock(time.Now).Return(tNow).Build().UnPatch
	defer mockey.Mock(time.Now).To(func() time.Time { return *pNow }).Build().UnPatch()

	sigmaYml := `
title: 特殊软件行为
id: 2
status: 1
technique_type: 爆破
description: "描述"
seriousness: HIGH
initial: 70
detection:
  object: rule3 # 主体规则，以该规则为主体，画关联图谱进程树。
  ordered: false # 根据rule有序的; 有序时 count不起作用; 排序时，可以先判断是否都有，再判断顺序，减少计算量
  selection: # 时间滑动窗口内包含全部事件(有序？统计数量？);所以他们才有资格进入流式计算的条件
    rule1: 1 # 全是单条的 rule_id
    rule2: 2
    rule3: 3
  condition:
    time_window: 5sec
    group_by:
      - agent_id
    having: # 与关系 AND
      - rule1.image_loaded=rule2.image_loaded # 运算符 =
    count: # 如果不配置的话,默认是0
      rule1: 1
      rule2: 1
      rule3: 2`

	ctx := context.TODO()
	rs, err := NewOneRulesetMul(ctx, []byte(sigmaYml), Log{})
	if err != nil {
		t.Fatal(err)
	}
	{
		input := Log{AgentId: 88, EventId: 1, ImageLoaded: "1234567890"}
		ruleIDs := rs.Eval(1, &input)
		assert.Equal(t, len(ruleIDs), 0)
	}
	{
		input := Log{AgentId: 99, EventId: 1, ImageLoaded: "1234567890"}
		ruleIDs := rs.Eval(1, &input)
		assert.Equal(t, len(ruleIDs), 0)
	}
	*pNow = pNow.Add(time.Second)
	{
		input := Log{AgentId: 88, EventId: 2, ImageLoaded: "1234567890"}
		ruleIDs := rs.Eval(2, &input)
		assert.Equal(t, len(ruleIDs), 0)
	}
	{
		input := Log{AgentId: 99, EventId: 2, ImageLoaded: "1234567890"}
		ruleIDs := rs.Eval(2, &input)
		assert.Equal(t, len(ruleIDs), 0)
	}
	*pNow = pNow.Add(time.Second)
	{
		input := Log{AgentId: 88, EventId: 3, ImageLoaded: "bb1234567890"}
		ruleIDs := rs.Eval(3, &input)
		assert.Equal(t, len(ruleIDs), 0)
	}
	{
		input := Log{AgentId: 99, EventId: 3, ImageLoaded: "bb1234567890"}
		ruleIDs := rs.Eval(3, &input)
		assert.Equal(t, len(ruleIDs), 0)
	}
	*pNow = pNow.Add(time.Second)
	var lastEventID, lastEventID99 int64
	{
		input := Log{AgentId: 88, EventId: 4, ImageLoaded: "aa1234567890"}
		ruleIDs := rs.Eval(3, &input)
		assert.NotEqual(t, len(ruleIDs), 0)
		lastEventID = ruleIDs[0][0].EventID
		assert.NotEqual(t, ruleIDs[0][0].MainRuleID, int64(0))
		n, mainRuleID := 0, ruleIDs[0][0].MainRuleID
		assert.NotEqual(t, mainRuleID, int64(0))
		assert.Equal(t, ruleIDs[0][0].Score, int16(70))
		for _, c := range ruleIDs {
			for _, x := range c {
				if x.EventID != lastEventID {
					t.Fatal("has 2 lastEventID")
				}
				n++
			}
		}
		assert.Equal(t, n, 4) // 总共4个事件
	}
	{
		input := Log{AgentId: 99, EventId: 4, ImageLoaded: "aa1234567890"}
		ruleIDs := rs.Eval(3, &input)
		assert.NotEqual(t, ruleIDs, 0)
		lastEventID99 = ruleIDs[0][0].EventID
		assert.NotEqual(t, ruleIDs[0][0].MainRuleID, int64(0))
		n, mainRuleID := 0, ruleIDs[0][0].MainRuleID
		assert.NotEqual(t, mainRuleID, int64(0))
		assert.Equal(t, ruleIDs[0][0].Score, int16(70))
		for _, c := range ruleIDs {
			for _, x := range c {
				if x.EventID != lastEventID99 {
					t.Fatal("has 2 lastEventID")
				}
				n++
			}
		}
		assert.Equal(t, n, 4) // 总共4个事件
	}
	assert.NotEqual(t, lastEventID, lastEventID99)
	*pNow = pNow.Add(time.Second)
	{
		input := Log{
			AgentId:     88,
			EventId:     5,
			ImageLoaded: "aa1234567890",
		}
		ruleIDs := rs.Eval(2, &input)
		n, mainRuleID := 0, ruleIDs[0][0].MainRuleID
		assert.Equal(t, mainRuleID, int64(0))
		assert.Equal(t, ruleIDs[0][0].Score, int16(70))
		for _, c := range ruleIDs {
			for _, x := range c {
				if x.EventID != lastEventID {
					t.Fatal("has 2 lastEventID")
				}
				n++
			}
		}
		assert.Equal(t, n, 1) // 此时输出一个事件，事件id和之前的一样
		assert.Equal(t, ruleIDs[0][0].EventID, lastEventID)
	}
	{
		input := Log{AgentId: 99, EventId: 5, ImageLoaded: "aa1234567890"}
		ruleIDs := rs.Eval(2, &input)
		n, mainRuleID := 0, ruleIDs[0][0].MainRuleID
		assert.Equal(t, mainRuleID, int64(0))
		assert.Equal(t, ruleIDs[0][0].Score, int16(70))
		for _, c := range ruleIDs {
			for _, x := range c {
				if x.EventID != lastEventID99 {
					t.Fatal("has 2 lastEventID")
				}
				n++
			}
		}
		assert.Equal(t, n, 1)
		assert.Equal(t, ruleIDs[0][0].EventID, lastEventID99)
	}
	*pNow = pNow.Add(time.Second * 2)
	{
		input := Log{AgentId: 88, EventId: 6, ImageLoaded: "1234567890"}
		ruleIDs := rs.Eval(3, &input)
		assert.Equal(t, len(ruleIDs), 0)
	}
	{
		input := Log{AgentId: 99, EventId: 6, ImageLoaded: "1234567890"}
		ruleIDs := rs.Eval(3, &input)
		assert.Equal(t, len(ruleIDs), 0)
	}
	{
		input := Log{AgentId: 88, EventId: 7, ImageLoaded: "1234567890"}
		ruleIDs := rs.Eval(1, &input)
		assert.NotEqual(t, ruleIDs[0][0].EventID, lastEventID)
		n, mainRuleID := 0, ruleIDs[0][0].MainRuleID
		assert.NotEqual(t, mainRuleID, int64(0))
		lastEventID = ruleIDs[0][0].EventID
		assert.Equal(t, ruleIDs[0][0].Score, int16(70))
		for _, c := range ruleIDs {
			for _, x := range c {
				if x.EventID != lastEventID {
					t.Fatal("has 2 lastEventID")
				}
				n++
			}
		}
		assert.Equal(t, n, 6)
	}
	{
		input := Log{AgentId: 99, EventId: 7, ImageLoaded: "1234567890"}
		ruleIDs := rs.Eval(1, &input)
		assert.NotEqual(t, ruleIDs[0][0].EventID, lastEventID99)
		n, mainRuleID := 0, ruleIDs[0][0].MainRuleID
		assert.NotEqual(t, mainRuleID, int64(0))
		lastEventID99 = ruleIDs[0][0].EventID
		assert.Equal(t, ruleIDs[0][0].Score, int16(70))
		for _, c := range ruleIDs {
			for _, x := range c {
				if x.EventID != lastEventID99 {
					t.Fatal("has 2 lastEventID")
				}
				n++
			}
		}
		assert.Equal(t, n, 6)
	}

	t.Log("end...")
}

func Test_sigmaMul_onlyHaving(t *testing.T) {
	tNow, err := time.Parse(time.RFC3339, "2023-10-02T00:00:00Z")
	if err != nil {
		t.Fatal(err)
	}
	pNow := &tNow
	// fUnPatch := mockey.Mock(time.Now).Return(tNow).Build().UnPatch
	defer mockey.Mock(time.Now).To(func() time.Time { return *pNow }).Build().UnPatch()

	sigmaYml := `
title: 特殊软件行为
id: 2
status: 1
technique_type: 爆破
description: "描述"
seriousness: HIGH

detection:
  object: rule3 
  ordered: false 
  selection: 
    rule1: 1 
    rule2: 2
    rule3: 3
  condition:
    time_window: 5sec
    group_by:
      - agent_id
    having: # 与关系 AND
      - rule1.image_loaded=rule2.image_loaded # 运算符 =
    count: {} `

	ctx := context.TODO()
	rs, err := NewOneRulesetMul(ctx, []byte(sigmaYml), Log{})
	if err != nil {
		t.Fatal(err)
	}
	{
		input := Log{AgentId: 88, EventId: 1, ImageLoaded: "1234567890"}
		ruleIDs := rs.Eval(1, &input)
		assert.Equal(t, len(ruleIDs), 0)
	}
	{
		input := Log{AgentId: 99, EventId: 1, ImageLoaded: "1234567890"}
		ruleIDs := rs.Eval(1, &input)
		assert.Equal(t, len(ruleIDs), 0)
	}
	*pNow = pNow.Add(time.Second)
	var lastEventID, lastEventID99 int64
	{
		input := Log{AgentId: 88, EventId: 2, ImageLoaded: "1234567890"}
		ruleIDs := rs.Eval(2, &input)
		assert.NotEqual(t, len(ruleIDs), 0)
		lastEventID = ruleIDs[0][0].EventID
		n := 0
		for _, c := range ruleIDs {
			for _, x := range c {
				if x.EventID != lastEventID {
					t.Fatal("has 2 lastEventID")
				}
				n++
			}
		}
		assert.Equal(t, n, 2) // 总共4个事件
	}
	{
		input := Log{AgentId: 99, EventId: 2, ImageLoaded: "1234567890"}
		ruleIDs := rs.Eval(2, &input)
		assert.NotEqual(t, len(ruleIDs), 0)
		lastEventID99 = ruleIDs[0][0].EventID
		n := 0
		for _, c := range ruleIDs {
			for _, x := range c {
				if x.EventID != lastEventID99 {
					t.Fatal("has 2 lastEventID")
				}
				n++
			}
		}
		assert.Equal(t, n, 2) // 总共4个事件
	}
	assert.NotEqual(t, lastEventID, lastEventID99)
	*pNow = pNow.Add(time.Second)
	*pNow = pNow.Add(time.Second)
	{
		input := Log{AgentId: 88, EventId: 4, ImageLoaded: "aa1234567890"}
		ruleIDs := rs.Eval(3, &input)
		assert.Equal(t, len(ruleIDs), 1)
		n := 0
		for _, c := range ruleIDs {
			for _, x := range c {
				if x.EventID != lastEventID {
					t.Fatal("has 2 lastEventID")
				}
				n++
			}
		}
		assert.Equal(t, n, 1) // 此时输出一个事件，事件id和之前的一样
		assert.Equal(t, ruleIDs[0][0].EventID, lastEventID)
	}
	{
		input := Log{AgentId: 99, EventId: 4, ImageLoaded: "aa1234567890"}
		ruleIDs := rs.Eval(3, &input)
		assert.NotEqual(t, ruleIDs, 0)
		n := 0
		for _, c := range ruleIDs {
			for _, x := range c {
				if x.EventID != lastEventID99 {
					t.Fatal("has 2 lastEventID")
				}
				n++
			}
		}
		assert.Equal(t, n, 1)
		assert.Equal(t, ruleIDs[0][0].EventID, lastEventID99)
	}
	*pNow = pNow.Add(time.Second)
	{
		input := Log{
			AgentId:     88,
			EventId:     5,
			ImageLoaded: "aa1234567890",
		}
		ruleIDs := rs.Eval(2, &input)
		n := 0
		for _, c := range ruleIDs {
			for _, x := range c {
				if x.EventID != lastEventID {
					t.Fatal("has 2 lastEventID")
				}
				n++
			}
		}
		assert.Equal(t, n, 1) // 此时输出一个事件，事件id和之前的一样
		assert.Equal(t, ruleIDs[0][0].EventID, lastEventID)
	}
	{
		input := Log{AgentId: 99, EventId: 5, ImageLoaded: "aa1234567890"}
		ruleIDs := rs.Eval(2, &input)
		n := 0
		for _, c := range ruleIDs {
			for _, x := range c {
				if x.EventID != lastEventID99 {
					t.Fatal("has 2 lastEventID")
				}
				n++
			}
		}
		assert.Equal(t, n, 1)
		assert.Equal(t, ruleIDs[0][0].EventID, lastEventID99)
	}

	t.Log("end...")
}

func Test_sigmaMul_noPreFunc(t *testing.T) {
	tNow, err := time.Parse(time.RFC3339, "2023-10-02T00:00:00Z")
	if err != nil {
		t.Fatal(err)
	}
	pNow := &tNow
	// fUnPatch := mockey.Mock(time.Now).Return(tNow).Build().UnPatch
	defer mockey.Mock(time.Now).To(func() time.Time { return *pNow }).Build().UnPatch()

	sigmaYml := `
title: 特殊软件行为
id: 2
status: 1
technique_type: 爆破
description: "描述"
seriousness: HIGH

detection:
  # object: rule3 
  ordered: false 
  selection: 
    rule1: 1 
    rule2: 2
    rule3: 3
  condition:
    time_window: 
    
    group_by:
      - agent_id
    having: []
    count: {}
`

	ctx := context.TODO()
	rs, err := NewOneRulesetMul(ctx, []byte(sigmaYml), Log{})
	if err != nil {
		t.Fatal(err)
	}
	var lastEventID, lastEventID99 int64
	{
		input := Log{AgentId: 88, EventId: 1, ImageLoaded: "1234567890"}
		ruleIDs := rs.Eval(1, &input)
		assert.Equal(t, len(ruleIDs), 1)
		lastEventID = ruleIDs[0][0].EventID
		assert.NotEqual(t, ruleIDs[0][0].MainRuleID, int64(0))
		assert.NotEqual(t, lastEventID, int64(0))
	}
	{
		input := Log{AgentId: 99, EventId: 1, ImageLoaded: "1234567890"}
		ruleIDs := rs.Eval(1, &input)
		assert.Equal(t, len(ruleIDs), 1)
		lastEventID99 = ruleIDs[0][0].EventID
		assert.NotEqual(t, ruleIDs[0][0].MainRuleID, int64(0))
		assert.NotEqual(t, lastEventID99, int64(0))
	}
	assert.NotEqual(t, lastEventID, lastEventID99)
	*pNow = pNow.Add(time.Second)
	{
		input := Log{AgentId: 88, EventId: 2, ImageLoaded: "1234567890"}
		ruleIDs := rs.Eval(2, &input)
		assert.Equal(t, len(ruleIDs), 1)
		assert.Equal(t, ruleIDs[0][0].EventID, lastEventID)
		assert.Equal(t, ruleIDs[0][0].MainRuleID, int64(0))
	}
	{
		input := Log{AgentId: 99, EventId: 2, ImageLoaded: "1234567890"}
		ruleIDs := rs.Eval(2, &input)
		assert.Equal(t, len(ruleIDs), 1)
		assert.Equal(t, ruleIDs[0][0].EventID, lastEventID99)
		assert.Equal(t, ruleIDs[0][0].MainRuleID, int64(0))
	}
	*pNow = pNow.Add(time.Second)
	{
		input := Log{AgentId: 88, EventId: 3, ImageLoaded: "bb1234567890"}
		ruleIDs := rs.Eval(3, &input)
		assert.Equal(t, len(ruleIDs), 1)
		assert.Equal(t, ruleIDs[0][0].EventID, lastEventID)
		assert.Equal(t, ruleIDs[0][0].MainRuleID, int64(0))
	}
	{
		input := Log{AgentId: 99, EventId: 3, ImageLoaded: "bb1234567890"}
		ruleIDs := rs.Eval(3, &input)
		assert.Equal(t, len(ruleIDs), 1)
		assert.Equal(t, ruleIDs[0][0].EventID, lastEventID99)
		assert.Equal(t, ruleIDs[0][0].MainRuleID, int64(0))
	}

	// reset
	rs.ResetEventID(lastEventID)
	{
		input := Log{AgentId: 88, EventId: 3, ImageLoaded: "bb1234567890"}
		ruleIDs := rs.Eval(3, &input)
		assert.Equal(t, len(ruleIDs), 1)
		assert.NotEqual(t, ruleIDs[0][0].EventID, lastEventID)
		assert.NotEqual(t, ruleIDs[0][0].EventID, int64(0))
		assert.NotEqual(t, ruleIDs[0][0].MainRuleID, int64(0))
	}
	{
		input := Log{AgentId: 99, EventId: 3, ImageLoaded: "bb1234567890"}
		ruleIDs := rs.Eval(3, &input)
		assert.Equal(t, len(ruleIDs), 1)
		assert.Equal(t, ruleIDs[0][0].EventID, lastEventID99)
		assert.Equal(t, ruleIDs[0][0].MainRuleID, int64(0))
	}

	t.Log("end...")
}

/*
go test -benchmem -run=^$ -bench ^Benchmark_EvalMul$ github.com/lxt1045/broker/output/sigma -count=1 -v -cpuprofile cpu.prof -c
go test -benchmem -run=^$ -bench ^Benchmark_EvalMul$ github.com/lxt1045/broker/output/sigma -count=1 -v -memprofile cpu.prof -c
go tool pprof sigma.test.exe cpu.prof
go tool pprof -http=:8080 cpu.prof
*/
func Benchmark_EvalMul(b *testing.B) {
	sigmaYml := `
title: 特殊软件行为
id: 2
status: 1
technique_type: 爆破
description: "描述"
seriousness: HIGH

detection:
  object: rule3 # 主体规则，以该规则为主体，画关联图谱进程树。
  ordered: false # 根据rule有序的; 有序时 count不起作用; 排序时，可以先判断是否都有，再判断顺序，减少计算量
  selection: # 时间滑动窗口内包含全部事件(有序？统计数量？);所以他们才有资格进入流式计算的条件
    rule1: 1063 # 全是单条的 rule_id
    rule2: 2099
    rule3: 1701
  condition:
    time_window: 1sec
    group_by:
      - agent_id
    having: # 与关系 AND
      - rule1.image_loaded=rule2.image_loaded # 运算符 =
    count: # 如果不配置的话,默认是0
      rule1: 1
      rule2: 1
      rule3: 2`

	ctx := context.TODO()
	rs, err := NewOneRulesetMul(ctx, []byte(sigmaYml), Log{})
	if err != nil {
		b.Fatal(err)
	}

	input := Log{
		EventId:     1,
		ImageLoaded: "1234567890",
	}
	ruleIDs := rs.Eval(1063, &input)

	bs, _ := json.Marshal(&ruleIDs)
	b.Logf("results:%+v", string(bs))

	for i := 0; i < 1; i++ {
		b.Run("1063", func(b *testing.B) {
			rs, err = NewOneRulesetMul(ctx, []byte(sigmaYml), Log{})
			if err != nil {
				b.Fatal(err)
			}
			for j := 0; j < b.N; j++ {
				rs.Eval(1063, &input)
			}
		})
		b.Run("2099", func(b *testing.B) {
			rs, err = NewOneRulesetMul(ctx, []byte(sigmaYml), Log{})
			if err != nil {
				b.Fatal(err)
			}
			for j := 0; j < b.N; j++ {
				rs.Eval(2099, &input)
			}
		})
		b.Run("1701", func(b *testing.B) {
			rs, err = NewOneRulesetMul(ctx, []byte(sigmaYml), Log{})
			if err != nil {
				b.Fatal(err)
			}
			for j := 0; j < b.N; j++ {
				rs.Eval(1701, &input)
			}
		})
		b.Run("10086", func(b *testing.B) {
			rs, err = NewOneRulesetMul(ctx, []byte(sigmaYml), Log{})
			if err != nil {
				b.Fatal(err)
			}
			for j := 0; j < b.N; j++ {
				rs.Eval(10086, &input)
			}
		})
		b.Run("hit", func(b *testing.B) {
			rs, err = NewOneRulesetMul(ctx, []byte(sigmaYml), Log{})
			if err != nil {
				b.Fatal(err)
			}
			rs.Eval(1063, &input)
			rs.Eval(2099, &input)
			for j := 0; j < b.N; j++ {
				rs.Eval(1701, &input)
			}
		})
	}
}

func Test_MulEval_BenchMark(t *testing.T) {
	const (
		M = 1
		N = 1000 * 1000 * 1000
		T = 60
	)
	t.Logf("M:%d, N:%d, T:%d", M, N, T)

	sigmaYml := `
title: 特殊软件行为
id: 2
status: 1
technique_type: 爆破
description: "描述"
seriousness: HIGH

detection:
  object: rule3 # 主体规则，以该规则为主体，画关联图谱进程树。
  ordered: false # 根据rule有序的; 有序时 count不起作用; 排序时，可以先判断是否都有，再判断顺序，减少计算量
  selection: # 时间滑动窗口内包含全部事件(有序？统计数量？);所以他们才有资格进入流式计算的条件
    rule1: 1063 # 全是单条的 rule_id
    rule2: 2099
    rule3: 1701
  condition:
    time_window: 1sec
    group_by:
      - agent_id
    having: # 与关系 AND
      - rule1.image_loaded=rule2.image_loaded # 运算符 =
    count: # 如果不配置的话,默认是0
      rule1: 1
      rule2: 1
      rule3: 2`

	ctx := context.TODO()
	rs, err := NewOneRulesetMul(ctx, []byte(sigmaYml), Log{})
	if err != nil {
		t.Fatal(err)
	}

	input := Log{
		EventId:     1,
		ImageLoaded: "1234567890",
	}
	ruleIDs := rs.Eval(1063, &input)
	{
		bs, _ := json.Marshal(&ruleIDs)
		t.Logf("results:%+v", string(bs))
	}

	t.Run("no-hit", func(t *testing.T) {
		var wg sync.WaitGroup
		var msgSuss int64
		ctx := context.TODO()
		ctx, _ = context.WithTimeout(ctx, time.Second*T)
		showQps(ctx, &msgSuss)

		for j := 0; j < M; j++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for i := 0; i < N; i++ {
					rs.Eval(1701, &input)
					atomic.AddInt64(&msgSuss, 1)

					select {
					case <-ctx.Done():
						i = N + 1
					default:
					}
				}
			}()
		}
		wg.Wait()
		t.Log("end...")
	})

	t.Run("hit", func(t *testing.T) {
		var wg sync.WaitGroup
		var msgSuss int64
		ctx := context.TODO()
		ctx, _ = context.WithTimeout(ctx, time.Second*T)
		showQps(ctx, &msgSuss)

		rs.Eval(1063, &input)
		rs.Eval(2099, &input)
		for j := 0; j < M; j++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for i := 0; i < N; i++ {
					rs.Eval(1701, &input)
					atomic.AddInt64(&msgSuss, 1)

					select {
					case <-ctx.Done():
						i = N + 1
					default:
					}
				}
			}()
		}
		wg.Wait()
		t.Log("end...")
	})
}

type Log struct {
	LogId                     int64    `protobuf:"varint,1,opt,name=log_id,json=logId,proto3" json:"log_id,omitempty"`
	Platform                  string   `protobuf:"bytes,2,opt,name=platform,proto3" json:"platform,omitempty"`
	AgentId                   int64    `protobuf:"varint,3,opt,name=agent_id,json=agentId,proto3" json:"agent_id,omitempty"`
	Time                      int64    `protobuf:"varint,4,opt,name=time,proto3" json:"time,omitempty"`
	EventTime                 int64    `protobuf:"varint,5,opt,name=event_time,json=eventTime,proto3" json:"event_time,omitempty"`
	Mark                      int64    `protobuf:"varint,6,opt,name=mark,proto3" json:"mark,omitempty"`
	Source                    string   `protobuf:"bytes,7,opt,name=source,proto3" json:"source,omitempty"`
	SessionId                 int64    `protobuf:"varint,8,opt,name=session_id,json=sessionId,proto3" json:"session_id,omitempty"`
	Username                  string   `protobuf:"bytes,9,opt,name=username,proto3" json:"username,omitempty"`
	ParentUsername            string   `protobuf:"bytes,10,opt,name=parent_username,json=parentUsername,proto3" json:"parent_username,omitempty"`
	ParentCmdline             string   `protobuf:"bytes,11,opt,name=parent_cmdline,json=parentCmdline,proto3" json:"parent_cmdline,omitempty"`
	ParentPath                string   `protobuf:"bytes,12,opt,name=parent_path,json=parentPath,proto3" json:"parent_path,omitempty"`
	Description               string   `protobuf:"bytes,13,opt,name=description,proto3" json:"description,omitempty"`
	Product                   string   `protobuf:"bytes,14,opt,name=product,proto3" json:"product,omitempty"`
	Company                   string   `protobuf:"bytes,15,opt,name=company,proto3" json:"company,omitempty"`
	EventId                   int64    `protobuf:"varint,16,opt,name=event_id,json=eventId,proto3" json:"event_id,omitempty"`
	LogonId                   int64    `protobuf:"varint,17,opt,name=logon_id,json=logonId,proto3" json:"logon_id,omitempty"`
	LogonGuid                 string   `protobuf:"bytes,18,opt,name=logon_guid,json=logonGuid,proto3" json:"logon_guid,omitempty"`
	Pid                       int64    `protobuf:"varint,19,opt,name=pid,proto3" json:"pid,omitempty"`
	Name                      string   `protobuf:"bytes,20,opt,name=name,proto3" json:"name,omitempty"`
	Path                      string   `protobuf:"bytes,21,opt,name=path,proto3" json:"path,omitempty"`
	Cmdline                   string   `protobuf:"bytes,22,opt,name=cmdline,proto3" json:"cmdline,omitempty"`
	State                     string   `protobuf:"bytes,23,opt,name=state,proto3" json:"state,omitempty"`
	Cwd                       string   `protobuf:"bytes,24,opt,name=cwd,proto3" json:"cwd,omitempty"`
	Root                      string   `protobuf:"bytes,25,opt,name=root,proto3" json:"root,omitempty"`
	Euid                      int64    `protobuf:"varint,26,opt,name=euid,proto3" json:"euid,omitempty"`
	Egid                      int64    `protobuf:"varint,27,opt,name=egid,proto3" json:"egid,omitempty"`
	OnDisk                    int64    `protobuf:"varint,28,opt,name=on_disk,json=onDisk,proto3" json:"on_disk,omitempty"`
	Ppid                      int64    `protobuf:"varint,29,opt,name=ppid,proto3" json:"ppid,omitempty"`
	Pgroup                    int64    `protobuf:"varint,30,opt,name=pgroup,proto3" json:"pgroup,omitempty"`
	VirtualProcess            int64    `protobuf:"varint,31,opt,name=virtual_process,json=virtualProcess,proto3" json:"virtual_process,omitempty"`
	Upid                      int64    `protobuf:"varint,32,opt,name=upid,proto3" json:"upid,omitempty"`
	Uppid                     int64    `protobuf:"varint,33,opt,name=uppid,proto3" json:"uppid,omitempty"`
	Env                       string   `protobuf:"bytes,34,opt,name=env,proto3" json:"env,omitempty"`
	Syscall                   string   `protobuf:"bytes,35,opt,name=syscall,proto3" json:"syscall,omitempty"`
	Type                      string   `protobuf:"bytes,36,opt,name=type,proto3" json:"type,omitempty"`
	Flags                     int64    `protobuf:"varint,37,opt,name=flags,proto3" json:"flags,omitempty"`
	ExitCode                  int64    `protobuf:"varint,38,opt,name=exit_code,json=exitCode,proto3" json:"exit_code,omitempty"`
	TokenElevationType        string   `protobuf:"bytes,39,opt,name=token_elevation_type,json=tokenElevationType,proto3" json:"token_elevation_type,omitempty"`
	TokenElevationStatus      int64    `protobuf:"varint,40,opt,name=token_elevation_status,json=tokenElevationStatus,proto3" json:"token_elevation_status,omitempty"`
	MandatoryLabel            string   `protobuf:"bytes,41,opt,name=mandatory_label,json=mandatoryLabel,proto3" json:"mandatory_label,omitempty"`
	Socket                    int64    `protobuf:"varint,42,opt,name=socket,proto3" json:"socket,omitempty"`
	Family                    int64    `protobuf:"varint,43,opt,name=family,proto3" json:"family,omitempty"`
	Protocol                  string   `protobuf:"bytes,44,opt,name=protocol,proto3" json:"protocol,omitempty"`
	Initiated                 int64    `protobuf:"varint,45,opt,name=initiated,proto3" json:"initiated,omitempty"`
	LocalAddress              string   `protobuf:"bytes,46,opt,name=local_address,json=localAddress,proto3" json:"local_address,omitempty"`
	RemoteAddress             string   `protobuf:"bytes,47,opt,name=remote_address,json=remoteAddress,proto3" json:"remote_address,omitempty"`
	LocalPort                 int64    `protobuf:"varint,48,opt,name=local_port,json=localPort,proto3" json:"local_port,omitempty"`
	RemotePort                int64    `protobuf:"varint,49,opt,name=remote_port,json=remotePort,proto3" json:"remote_port,omitempty"`
	PipeName                  string   `protobuf:"bytes,50,opt,name=pipe_name,json=pipeName,proto3" json:"pipe_name,omitempty"`
	PipeStatus                string   `protobuf:"bytes,51,opt,name=pipe_status,json=pipeStatus,proto3" json:"pipe_status,omitempty"`
	Mode                      string   `protobuf:"bytes,52,opt,name=mode,proto3" json:"mode,omitempty"`
	DestPath                  string   `protobuf:"bytes,53,opt,name=dest_path,json=destPath,proto3" json:"dest_path,omitempty"`
	Uid                       string   `protobuf:"bytes,54,opt,name=uid,proto3" json:"uid,omitempty"`
	Gid                       string   `protobuf:"bytes,55,opt,name=gid,proto3" json:"gid,omitempty"`
	Size_                     int64    `protobuf:"varint,56,opt,name=size,proto3" json:"size,omitempty"`
	Mtime                     int64    `protobuf:"varint,57,opt,name=mtime,proto3" json:"mtime,omitempty"`
	Ctime                     int64    `protobuf:"varint,58,opt,name=ctime,proto3" json:"ctime,omitempty"`
	FileId                    string   `protobuf:"bytes,59,opt,name=file_id,json=fileId,proto3" json:"file_id,omitempty"`
	IsExecutable              int64    `protobuf:"varint,60,opt,name=is_executable,json=isExecutable,proto3" json:"is_executable,omitempty"`
	FileVersion               string   `protobuf:"bytes,61,opt,name=file_version,json=fileVersion,proto3" json:"file_version,omitempty"`
	ProductVersion            string   `protobuf:"bytes,62,opt,name=product_version,json=productVersion,proto3" json:"product_version,omitempty"`
	OriginalFilename          string   `protobuf:"bytes,63,opt,name=original_filename,json=originalFilename,proto3" json:"original_filename,omitempty"`
	Action                    string   `protobuf:"bytes,64,opt,name=action,proto3" json:"action,omitempty"`
	Sha256                    string   `protobuf:"bytes,65,opt,name=sha256,proto3" json:"sha256,omitempty"`
	GrantedAccess             string   `protobuf:"bytes,66,opt,name=granted_access,json=grantedAccess,proto3" json:"granted_access,omitempty"`
	SourcePguid               string   `protobuf:"bytes,67,opt,name=source_pguid,json=sourcePguid,proto3" json:"source_pguid,omitempty"`
	SourcePid                 int64    `protobuf:"varint,68,opt,name=source_pid,json=sourcePid,proto3" json:"source_pid,omitempty"`
	SourcePath                string   `protobuf:"bytes,69,opt,name=source_path,json=sourcePath,proto3" json:"source_path,omitempty"`
	TargetPguid               string   `protobuf:"bytes,70,opt,name=target_pguid,json=targetPguid,proto3" json:"target_pguid,omitempty"`
	TargetPid                 int64    `protobuf:"varint,71,opt,name=target_pid,json=targetPid,proto3" json:"target_pid,omitempty"`
	TargetPath                string   `protobuf:"bytes,72,opt,name=target_path,json=targetPath,proto3" json:"target_path,omitempty"`
	TargetFilename            string   `protobuf:"bytes,73,opt,name=target_filename,json=targetFilename,proto3" json:"target_filename,omitempty"`
	CallTrace                 string   `protobuf:"bytes,74,opt,name=call_trace,json=callTrace,proto3" json:"call_trace,omitempty"`
	ImageLoaded               string   `protobuf:"bytes,75,opt,name=image_loaded,json=imageLoaded,proto3" json:"image_loaded,omitempty"`
	Signed                    int64    `protobuf:"varint,76,opt,name=signed,proto3" json:"signed,omitempty"`
	Signature                 string   `protobuf:"bytes,77,opt,name=signature,proto3" json:"signature,omitempty"`
	SignatureStatus           string   `protobuf:"bytes,78,opt,name=signature_status,json=signatureStatus,proto3" json:"signature_status,omitempty"`
	Key                       string   `protobuf:"bytes,79,opt,name=key,proto3" json:"key,omitempty"`
	TargetObject              string   `protobuf:"bytes,80,opt,name=target_object,json=targetObject,proto3" json:"target_object,omitempty"`
	Details                   string   `protobuf:"bytes,81,opt,name=details,proto3" json:"details,omitempty"`
	EventType                 string   `protobuf:"bytes,82,opt,name=event_type,json=eventType,proto3" json:"event_type,omitempty"`
	NewKey                    string   `protobuf:"bytes,83,opt,name=new_key,json=newKey,proto3" json:"new_key,omitempty"`
	RelativePath              string   `protobuf:"bytes,84,opt,name=relative_path,json=relativePath,proto3" json:"relative_path,omitempty"`
	Query                     string   `protobuf:"bytes,85,opt,name=query,proto3" json:"query,omitempty"`
	Script                    string   `protobuf:"bytes,86,opt,name=script,proto3" json:"script,omitempty"`
	Consumer                  string   `protobuf:"bytes,87,opt,name=consumer,proto3" json:"consumer,omitempty"`
	Filter                    string   `protobuf:"bytes,88,opt,name=filter,proto3" json:"filter,omitempty"`
	QueryName                 string   `protobuf:"bytes,89,opt,name=query_name,json=queryName,proto3" json:"query_name,omitempty"`
	QueryStatus               string   `protobuf:"bytes,90,opt,name=query_status,json=queryStatus,proto3" json:"query_status,omitempty"`
	QueryResults              string   `protobuf:"bytes,91,opt,name=query_results,json=queryResults,proto3" json:"query_results,omitempty"`
	NewThreadId               int64    `protobuf:"varint,92,opt,name=new_thread_id,json=newThreadId,proto3" json:"new_thread_id,omitempty"`
	StartAddress              string   `protobuf:"bytes,93,opt,name=start_address,json=startAddress,proto3" json:"start_address,omitempty"`
	StartModule               string   `protobuf:"bytes,94,opt,name=start_module,json=startModule,proto3" json:"start_module,omitempty"`
	StartFunction             string   `protobuf:"bytes,95,opt,name=start_function,json=startFunction,proto3" json:"start_function,omitempty"`
	Tty                       string   `protobuf:"bytes,96,opt,name=tty,proto3" json:"tty,omitempty"`
	SshConnection             string   `protobuf:"bytes,97,opt,name=ssh_connection,json=sshConnection,proto3" json:"ssh_connection,omitempty"`
	LdPreload                 string   `protobuf:"bytes,98,opt,name=ld_preload,json=ldPreload,proto3" json:"ld_preload,omitempty"`
	LdLibraryPath             string   `protobuf:"bytes,99,opt,name=ld_library_path,json=ldLibraryPath,proto3" json:"ld_library_path,omitempty"`
	Proxy                     string   `protobuf:"bytes,100,opt,name=proxy,proto3" json:"proxy,omitempty"`
	StandardIn                string   `protobuf:"bytes,101,opt,name=standard_in,json=standardIn,proto3" json:"standard_in,omitempty"`
	StandardOut               string   `protobuf:"bytes,102,opt,name=standard_out,json=standardOut,proto3" json:"standard_out,omitempty"`
	StandardError             string   `protobuf:"bytes,103,opt,name=standard_error,json=standardError,proto3" json:"standard_error,omitempty"`
	LogonType                 string   `protobuf:"bytes,104,opt,name=logon_type,json=logonType,proto3" json:"logon_type,omitempty"`
	AuthenticationPackageName string   `protobuf:"bytes,105,opt,name=authentication_package_name,json=authenticationPackageName,proto3" json:"authentication_package_name,omitempty"`
	IpAddress                 string   `protobuf:"bytes,106,opt,name=ip_address,json=ipAddress,proto3" json:"ip_address,omitempty"`
	ServiceName               string   `protobuf:"bytes,107,opt,name=service_name,json=serviceName,proto3" json:"service_name,omitempty"`
	Extra1                    string   `protobuf:"bytes,108,opt,name=extra1,proto3" json:"extra1,omitempty"`
	Extra2                    string   `protobuf:"bytes,109,opt,name=extra2,proto3" json:"extra2,omitempty"`
	Extra3                    string   `protobuf:"bytes,110,opt,name=extra3,proto3" json:"extra3,omitempty"`
	Extra4                    string   `protobuf:"bytes,111,opt,name=extra4,proto3" json:"extra4,omitempty"`
	Extra5                    string   `protobuf:"bytes,112,opt,name=extra5,proto3" json:"extra5,omitempty"`
	Extra6                    string   `protobuf:"bytes,113,opt,name=extra6,proto3" json:"extra6,omitempty"`
	Extra7                    string   `protobuf:"bytes,114,opt,name=extra7,proto3" json:"extra7,omitempty"`
	Extra8                    string   `protobuf:"bytes,115,opt,name=extra8,proto3" json:"extra8,omitempty"`
	Extra9                    string   `protobuf:"bytes,116,opt,name=extra9,proto3" json:"extra9,omitempty"`
	Extra10                   string   `protobuf:"bytes,117,opt,name=extra10,proto3" json:"extra10,omitempty"`
	Extra11                   string   `protobuf:"bytes,118,opt,name=extra11,proto3" json:"extra11,omitempty"`
	Extra12                   string   `protobuf:"bytes,119,opt,name=extra12,proto3" json:"extra12,omitempty"`
	Extra13                   string   `protobuf:"bytes,120,opt,name=extra13,proto3" json:"extra13,omitempty"`
	Extra14                   string   `protobuf:"bytes,121,opt,name=extra14,proto3" json:"extra14,omitempty"`
	Extra15                   string   `protobuf:"bytes,122,opt,name=extra15,proto3" json:"extra15,omitempty"`
	Extra16                   string   `protobuf:"bytes,123,opt,name=extra16,proto3" json:"extra16,omitempty"`
	Extra17                   string   `protobuf:"bytes,124,opt,name=extra17,proto3" json:"extra17,omitempty"`
	Extra18                   string   `protobuf:"bytes,125,opt,name=extra18,proto3" json:"extra18,omitempty"`
	Extra19                   string   `protobuf:"bytes,126,opt,name=extra19,proto3" json:"extra19,omitempty"`
	Extra20                   string   `protobuf:"bytes,127,opt,name=extra20,proto3" json:"extra20,omitempty"`
	LocalHost                 string   `protobuf:"bytes,128,opt,name=local_host,json=localHost,proto3" json:"local_host,omitempty"`
	RemoteHost                string   `protobuf:"bytes,129,opt,name=remote_host,json=remoteHost,proto3" json:"remote_host,omitempty"`
	SourceUser                string   `protobuf:"bytes,130,opt,name=source_user,json=sourceUser,proto3" json:"source_user,omitempty"`
	TargetUser                string   `protobuf:"bytes,131,opt,name=target_user,json=targetUser,proto3" json:"target_user,omitempty"`
	ServiceSid                string   `protobuf:"bytes,132,opt,name=service_sid,json=serviceSid,proto3" json:"service_sid,omitempty"`
	TicketOptions             string   `protobuf:"bytes,133,opt,name=ticket_options,json=ticketOptions,proto3" json:"ticket_options,omitempty"`
	TicketEncryptionType      string   `protobuf:"bytes,134,opt,name=ticket_encryption_type,json=ticketEncryptionType,proto3" json:"ticket_encryption_type,omitempty"`
	SubjectUsersid            string   `protobuf:"bytes,135,opt,name=subject_usersid,json=subjectUsersid,proto3" json:"subject_usersid,omitempty"`
	SubjectUsername           string   `protobuf:"bytes,136,opt,name=subject_username,json=subjectUsername,proto3" json:"subject_username,omitempty"`
	SubjectDomainName         string   `protobuf:"bytes,137,opt,name=subject_domain_name,json=subjectDomainName,proto3" json:"subject_domain_name,omitempty"`
	SubjectLogonId            string   `protobuf:"bytes,138,opt,name=subject_logon_id,json=subjectLogonId,proto3" json:"subject_logon_id,omitempty"`
	TargetUsersid             string   `protobuf:"bytes,139,opt,name=target_usersid,json=targetUsersid,proto3" json:"target_usersid,omitempty"`
	TargetUsername            string   `protobuf:"bytes,140,opt,name=target_username,json=targetUsername,proto3" json:"target_username,omitempty"`
	TargetDomainName          string   `protobuf:"bytes,141,opt,name=target_domain_name,json=targetDomainName,proto3" json:"target_domain_name,omitempty"`
	TargetLogonId             string   `protobuf:"bytes,142,opt,name=target_logon_id,json=targetLogonId,proto3" json:"target_logon_id,omitempty"`
	LogonProcessName          string   `protobuf:"bytes,143,opt,name=logon_process_name,json=logonProcessName,proto3" json:"logon_process_name,omitempty"`
	WorkstationName           string   `protobuf:"bytes,144,opt,name=workstation_name,json=workstationName,proto3" json:"workstation_name,omitempty"`
	TransmittedServices       string   `protobuf:"bytes,145,opt,name=transmitted_services,json=transmittedServices,proto3" json:"transmitted_services,omitempty"`
	LmPackageName             string   `protobuf:"bytes,146,opt,name=lm_package_name,json=lmPackageName,proto3" json:"lm_package_name,omitempty"`
	KeyLength                 string   `protobuf:"bytes,147,opt,name=key_length,json=keyLength,proto3" json:"key_length,omitempty"`
	IpPort                    string   `protobuf:"bytes,148,opt,name=ip_port,json=ipPort,proto3" json:"ip_port,omitempty"`
	ImpersonationLevel        string   `protobuf:"bytes,149,opt,name=impersonation_level,json=impersonationLevel,proto3" json:"impersonation_level,omitempty"`
	RestrictedAdminMode       string   `protobuf:"bytes,150,opt,name=restricted_admin_mode,json=restrictedAdminMode,proto3" json:"restricted_admin_mode,omitempty"`
	RemoteCredentialGuard     string   `protobuf:"bytes,151,opt,name=remote_credential_guard,json=remoteCredentialGuard,proto3" json:"remote_credential_guard,omitempty"`
	TargetOutboundUsername    string   `protobuf:"bytes,152,opt,name=target_outbound_username,json=targetOutboundUsername,proto3" json:"target_outbound_username,omitempty"`
	TargetOutboundDomainName  string   `protobuf:"bytes,153,opt,name=target_outbound_domain_name,json=targetOutboundDomainName,proto3" json:"target_outbound_domain_name,omitempty"`
	VirtualAccount            string   `protobuf:"bytes,154,opt,name=virtual_account,json=virtualAccount,proto3" json:"virtual_account,omitempty"`
	TargetLinkedLogonId       string   `protobuf:"bytes,155,opt,name=target_linked_logon_id,json=targetLinkedLogonId,proto3" json:"target_linked_logon_id,omitempty"`
	ElevatedToken             string   `protobuf:"bytes,156,opt,name=elevated_token,json=elevatedToken,proto3" json:"elevated_token,omitempty"`
	TargetLogonGuid           string   `protobuf:"bytes,157,opt,name=target_logon_guid,json=targetLogonGuid,proto3" json:"target_logon_guid,omitempty"`
	SidList                   string   `protobuf:"bytes,158,opt,name=sid_list,json=sidList,proto3" json:"sid_list,omitempty"`
	PrivilegeList             string   `protobuf:"bytes,159,opt,name=privilege_list,json=privilegeList,proto3" json:"privilege_list,omitempty"`
	EnabledPrivilegeList      string   `protobuf:"bytes,160,opt,name=enabled_privilege_list,json=enabledPrivilegeList,proto3" json:"enabled_privilege_list,omitempty"`
	DisabledPrivilegeList     string   `protobuf:"bytes,161,opt,name=disabled_privilege_list,json=disabledPrivilegeList,proto3" json:"disabled_privilege_list,omitempty"`
	ObjectServer              string   `protobuf:"bytes,162,opt,name=object_server,json=objectServer,proto3" json:"object_server,omitempty"`
	ObjectType                string   `protobuf:"bytes,163,opt,name=object_type,json=objectType,proto3" json:"object_type,omitempty"`
	ObjectName                string   `protobuf:"bytes,164,opt,name=object_name,json=objectName,proto3" json:"object_name,omitempty"`
	HandleId                  string   `protobuf:"bytes,165,opt,name=handle_id,json=handleId,proto3" json:"handle_id,omitempty"`
	AccessMask                string   `protobuf:"bytes,166,opt,name=access_mask,json=accessMask,proto3" json:"access_mask,omitempty"`
	ServiceFileName           string   `protobuf:"bytes,167,opt,name=service_file_name,json=serviceFileName,proto3" json:"service_file_name,omitempty"`
	ServiceType               string   `protobuf:"bytes,168,opt,name=service_type,json=serviceType,proto3" json:"service_type,omitempty"`
	ServiceStartType          string   `protobuf:"bytes,169,opt,name=service_start_type,json=serviceStartType,proto3" json:"service_start_type,omitempty"`
	ServiceAccount            string   `protobuf:"bytes,170,opt,name=service_account,json=serviceAccount,proto3" json:"service_account,omitempty"`
	TaskName                  string   `protobuf:"bytes,171,opt,name=task_name,json=taskName,proto3" json:"task_name,omitempty"`
	TaskContent               string   `protobuf:"bytes,172,opt,name=task_content,json=taskContent,proto3" json:"task_content,omitempty"`
	ShareName                 string   `protobuf:"bytes,173,opt,name=share_name,json=shareName,proto3" json:"share_name,omitempty"`
	ShareLocalPath            string   `protobuf:"bytes,174,opt,name=share_local_path,json=shareLocalPath,proto3" json:"share_local_path,omitempty"`
	RelativeTargetName        string   `protobuf:"bytes,175,opt,name=relative_target_name,json=relativeTargetName,proto3" json:"relative_target_name,omitempty"`
	AccessList                string   `protobuf:"bytes,176,opt,name=access_list,json=accessList,proto3" json:"access_list,omitempty"`
	AccessReason              string   `protobuf:"bytes,177,opt,name=access_reason,json=accessReason,proto3" json:"access_reason,omitempty"`
	RestrictedSidCount        string   `protobuf:"bytes,178,opt,name=restricted_sid_count,json=restrictedSidCount,proto3" json:"restricted_sid_count,omitempty"`
	ResourceAttributes        string   `protobuf:"bytes,179,opt,name=resource_attributes,json=resourceAttributes,proto3" json:"resource_attributes,omitempty"`
	TransactionId             string   `protobuf:"bytes,180,opt,name=transaction_id,json=transactionId,proto3" json:"transaction_id,omitempty"`
	CertIssuerName            string   `protobuf:"bytes,181,opt,name=cert_issuer_name,json=certIssuerName,proto3" json:"cert_issuer_name,omitempty"`
	CertSerialNumber          string   `protobuf:"bytes,182,opt,name=cert_serial_number,json=certSerialNumber,proto3" json:"cert_serial_number,omitempty"`
	CertThumbprint            string   `protobuf:"bytes,183,opt,name=cert_thumbprint,json=certThumbprint,proto3" json:"cert_thumbprint,omitempty"`
	PreAuthType               string   `protobuf:"bytes,184,opt,name=pre_auth_type,json=preAuthType,proto3" json:"pre_auth_type,omitempty"`
	SamAccountName            string   `protobuf:"bytes,185,opt,name=sam_account_name,json=samAccountName,proto3" json:"sam_account_name,omitempty"`
	SidHistory                string   `protobuf:"bytes,186,opt,name=sid_history,json=sidHistory,proto3" json:"sid_history,omitempty"`
	MemberName                string   `protobuf:"bytes,187,opt,name=member_name,json=memberName,proto3" json:"member_name,omitempty"`
	MemberSid                 string   `protobuf:"bytes,188,opt,name=member_sid,json=memberSid,proto3" json:"member_sid,omitempty"`
	CallerProcessId           string   `protobuf:"bytes,189,opt,name=caller_process_id,json=callerProcessId,proto3" json:"caller_process_id,omitempty"`
	CallerProcessName         string   `protobuf:"bytes,190,opt,name=caller_process_name,json=callerProcessName,proto3" json:"caller_process_name,omitempty"`
	ProfileChanged            string   `protobuf:"bytes,191,opt,name=profile_changed,json=profileChanged,proto3" json:"profile_changed,omitempty"`
	RuleId                    string   `protobuf:"bytes,192,opt,name=rule_id,json=ruleId,proto3" json:"rule_id,omitempty"`
	RuleName                  string   `protobuf:"bytes,193,opt,name=rule_name,json=ruleName,proto3" json:"rule_name,omitempty"`
	SettingType               string   `protobuf:"bytes,194,opt,name=setting_type,json=settingType,proto3" json:"setting_type,omitempty"`
	SettingValue              string   `protobuf:"bytes,195,opt,name=settingValue,proto3" json:"settingValue,omitempty"`
	Application               string   `protobuf:"bytes,196,opt,name=application,proto3" json:"application,omitempty"`
	Status                    string   `protobuf:"bytes,197,opt,name=status,proto3" json:"status,omitempty"`
	Workstation               string   `protobuf:"bytes,198,opt,name=workstation,proto3" json:"workstation,omitempty"`
	PackageName               string   `protobuf:"bytes,199,opt,name=package_name,json=packageName,proto3" json:"package_name,omitempty"`
	DomainPolicyChanged       string   `protobuf:"bytes,200,opt,name=domain_policy_changed,json=domainPolicyChanged,proto3" json:"domain_policy_changed,omitempty"`
	DomainName                string   `protobuf:"bytes,201,opt,name=domain_name,json=domainName,proto3" json:"domain_name,omitempty"`
	DsName                    string   `protobuf:"bytes,202,opt,name=ds_name,json=dsName,proto3" json:"ds_name,omitempty"`
	DsType                    string   `protobuf:"bytes,203,opt,name=ds_type,json=dsType,proto3" json:"ds_type,omitempty"`
	AccountName               string   `protobuf:"bytes,204,opt,name=account_name,json=accountName,proto3" json:"account_name,omitempty"`
	Ip                        string   `protobuf:"bytes,205,opt,name=ip,proto3" json:"ip,omitempty"`
	EnterExePath              string   `protobuf:"bytes,206,opt,name=enter_exe_path,json=enterExePath,proto3" json:"enter_exe_path,omitempty"`
	EnterCmdline              string   `protobuf:"bytes,207,opt,name=enter_cmdline,json=enterCmdline,proto3" json:"enter_cmdline,omitempty"`
	ContainerId               string   `protobuf:"bytes,208,opt,name=container_id,json=containerId,proto3" json:"container_id,omitempty"`
	ContainerName             string   `protobuf:"bytes,209,opt,name=container_name,json=containerName,proto3" json:"container_name,omitempty"`
	IsContainer               int64    `protobuf:"varint,210,opt,name=is_container,json=isContainer,proto3" json:"is_container,omitempty"`
	RaspType                  string   `protobuf:"bytes,211,opt,name=rasp_type,json=raspType,proto3" json:"rasp_type,omitempty"`
	Payload                   string   `protobuf:"bytes,212,opt,name=payload,proto3" json:"payload,omitempty"`
	Method                    string   `protobuf:"bytes,213,opt,name=method,proto3" json:"method,omitempty"`
	RequestUrl                string   `protobuf:"bytes,214,opt,name=request_url,json=requestUrl,proto3" json:"request_url,omitempty"`
	RequestUri                string   `protobuf:"bytes,215,opt,name=request_uri,json=requestUri,proto3" json:"request_uri,omitempty"`
	ContentType               string   `protobuf:"bytes,216,opt,name=content_type,json=contentType,proto3" json:"content_type,omitempty"`
	RequestParameters         string   `protobuf:"bytes,217,opt,name=request_parameters,json=requestParameters,proto3" json:"request_parameters,omitempty"`
	RequestHeader             string   `protobuf:"bytes,218,opt,name=request_header,json=requestHeader,proto3" json:"request_header,omitempty"`
	RequestQuery              string   `protobuf:"bytes,219,opt,name=request_query,json=requestQuery,proto3" json:"request_query,omitempty"`
	RequestBody               string   `protobuf:"bytes,220,opt,name=request_body,json=requestBody,proto3" json:"request_body,omitempty"`
	XXX_NoUnkeyedLiteral      struct{} `json:"-"`
	XXX_unrecognized          []byte   `json:"-"`
	XXX_sizecache             int32    `json:"-"`
}
