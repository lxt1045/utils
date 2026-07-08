package geohash

import (
	"math"
	"math/rand"
	"testing"
)

// ringArea 是测试内部小工具：对闭合环(不含重复首尾)求面积，用于比较两种实现。
func ringAreaEqual(a, b []Coords, relTol float64) bool {
	aa, ab := AreaCoords(a), AreaCoords(b)
	if ab == 0 {
		return aa == 0
	}
	return math.Abs(aa-ab)/ab <= relTol
}

// TestMergeCurvesIndexed_MatchesBaseline 用一批随机打散的多边形边界，验证
// MergeCurvesIndexed 与 MergeCurves 还原出的环：顶点数一致、面积一致。
func TestMergeCurvesIndexed_MatchesBaseline(t *testing.T) {
	type polygon struct {
		cx, cy, r float64
		sides     int
	}
	polys := []polygon{
		{116.4, 39.9, 0.1, 4},
		{116.4, 39.9, 0.1, 8},
		{10.0, 60.0, 0.2, 6},   // 高纬
		{-70.0, -33.0, 0.15, 5}, // 南半球
		{0.05, 0.05, 0.03, 7},   // 近赤道近本初子午线
	}

	rng := rand.New(rand.NewSource(2024))
	for pi, pg := range polys {
		// 生成规则多边形顶点。
		verts := make([]Coords, pg.sides)
		for k := range pg.sides {
			ang := 2 * math.Pi * float64(k) / float64(pg.sides)
			verts[k] = Coords{Lat: pg.cy + pg.r*math.Sin(ang), Lng: pg.cx + pg.r*math.Cos(ang)}
		}
		// 切成每边一段，随机反转方向并打乱顺序，端点精确重合。
		closed := append(append([]Coords(nil), verts...), verts[0])
		curves := make([][]Coords, pg.sides)
		for s := range pg.sides {
			seg := []Coords{closed[s], closed[s+1]}
			if rng.Intn(2) == 0 {
				seg = reverseCoords(seg)
			}
			curves[s] = seg
		}
		rng.Shuffle(len(curves), func(i, j int) { curves[i], curves[j] = curves[j], curves[i] })

		const tolerance = 5.0
		base := MergeCurves(curves, tolerance)
		idx := MergeCurvesIndexed(curves, tolerance)

		if len(base) != len(idx) {
			t.Errorf("poly#%d 顶点数不一致: baseline=%d indexed=%d", pi, len(base), len(idx))
			continue
		}
		if len(idx) != pg.sides {
			t.Errorf("poly#%d indexed 顶点数=%d, 期望=%d", pi, len(idx), pg.sides)
		}
		if !ringAreaEqual(base, idx, 1e-9) {
			t.Errorf("poly#%d 面积不一致: baseline=%.4f indexed=%.4f",
				pi, AreaCoords(base), AreaCoords(idx))
		}
	}
}

// TestMergeCurvesIndexed_ToleranceDedup 衔接处有 < tolerance 的抖动时，索引版
// 仍应正确衔接去重，结果与基线一致。
func TestMergeCurvesIndexed_ToleranceDedup(t *testing.T) {
	c1 := []Coords{{Lat: 0, Lng: 0}, {Lat: 0, Lng: 1}}
	c2 := []Coords{{Lat: 0.0000045, Lng: 1}, {Lat: 1, Lng: 1}} // 起点比 c1 终点偏 ~0.5m
	c3 := []Coords{{Lat: 1, Lng: 1}, {Lat: 1, Lng: 0}}
	c4 := []Coords{{Lat: 1, Lng: 0}, {Lat: 0, Lng: 0}}
	curves := [][]Coords{c3, c1, c4, c2}

	base := MergeCurves(curves, 1.0)
	idx := MergeCurvesIndexed(curves, 1.0)
	if len(base) != len(idx) {
		t.Fatalf("顶点数不一致: baseline=%d indexed=%d", len(base), len(idx))
	}
	if !ringAreaEqual(base, idx, 1e-9) {
		t.Fatalf("面积不一致: baseline=%.6f indexed=%.6f", AreaCoords(base), AreaCoords(idx))
	}
}

// TestMergeCurvesIndexed_Empty 空输入返回 nil。
func TestMergeCurvesIndexed_Empty(t *testing.T) {
	if r := MergeCurvesIndexed(nil, 1.0); r != nil {
		t.Fatalf("nil 输入应返回 nil, 得到 %+v", r)
	}
}

// BenchmarkMergeCurvesIndexedVsBaseline 在不同曲线数下对比两种实现的耗时，
// 直观展示 MergeCurves 的 O(曲线数²) 与 MergeCurvesIndexed 的近 O(曲线数) 差异。
// 每条曲线只有 2 个点(纯端点场景)，把成本集中在“找衔接曲线”上，放大扫描差异。
func BenchmarkMergeCurvesIndexedVsBaseline(b *testing.B) {
	const tolerance = 1.0
	for _, nCurves := range []int{50, 200, 800, 3200} {
		// 每条曲线 2 个点：段内去重成本可忽略，凸显端点匹配的扫描代价。
		curves := buildScrambledRingCurves(nCurves, 2, 1.0, 0, 42)

		b.Run("baseline/"+itoa(nCurves), func(b *testing.B) {
			b.ReportAllocs()
			for range b.N {
				_ = MergeCurves(curves, tolerance)
			}
		})
		b.Run("indexed/"+itoa(nCurves), func(b *testing.B) {
			b.ReportAllocs()
			for range b.N {
				_ = MergeCurvesIndexed(curves, tolerance)
			}
		})
	}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}
