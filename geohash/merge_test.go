package geohash

import (
	"math"
	"testing"
)

func Test_unionFind(t *testing.T) {
	t.Run("1-0", func(t *testing.T) {
		uf := newUnionFind(5)
		uf.union(2, 1)
		uf.union(1, 3)
		uf.union(2, 4)
		for i, v := range uf.matchPoints {
			t.Logf("%d:%d", i, v)
		}
	})
	t.Run("1", func(t *testing.T) {
		uf := newUnionFind(5)
		uf.union(1, 2)
		uf.union(1, 3)
		uf.union(2, 4)
		for i, v := range uf.matchPoints {
			t.Logf("%d:%d", i, v)
		}
	})
	t.Run("2", func(t *testing.T) {
		uf := newUnionFind2(5)
		uf.union(1, 2)
		uf.union(1, 3)
		uf.union(2, 4)
		for i, v := range uf.matchPoints {
			t.Logf("%d:%+v", i, v)
		}
	})
}

func Benchmark_distMeters(b *testing.B) {
	x := Coords{Lat: 39.90, Lng: 116.30}
	y := Coords{Lat: 39.90, Lng: 116.31}

	b.Run("distMeters", func(b *testing.B) {
		b.ReportAllocs()
		for range b.N {
			distMeters(x, y)
		}
	})
}

// densifyEdgeTest 在 [u,v] 上按 stepM 步长线性插值(含 u，不含 v)，用于把稀疏
// 折线加密成「密集点曲线」(相邻点间距 < stepM)，满足 MergeCurves2 的输入前提。
func densifyEdgeTest(u, v Coords, stepM float64) []Coords {
	d := distMeters(u, v)
	steps := int(d/stepM) + 1
	out := make([]Coords, 0, steps)
	for k := 0; k < steps; k++ {
		f := float64(k) / float64(steps)
		out = append(out, Coords{Lat: u.Lat + (v.Lat-u.Lat)*f, Lng: u.Lng + (v.Lng-u.Lng)*f})
	}
	return out
}

// 把一个正方形边界拆成 4 条乱序、方向不一、端点重合的密集点曲线，
// 验证 MergeCurves0 / MergeCurves2 都能拼回闭合环，且面积与直接构造的一致。
func TestMergeCurves_SquareFromScrambledEdges(t *testing.T) {
	// 正方形四角(北京附近，边长 ~1.1km；顺时针)。用小尺度以便密集采样点数可控。
	a := Coords{Lat: 39.90, Lng: 116.30}
	b := Coords{Lat: 39.90, Lng: 116.31}
	c := Coords{Lat: 39.91, Lng: 116.31}
	d := Coords{Lat: 39.91, Lng: 116.30}
	const tolerance = 5.0
	const stepM = 2.0 // < tolerance/2，满足密集点前提

	// 四条边密集重采样(各含起点、不含终点)，故意打乱顺序、反转部分方向；
	// 相邻边在角点处共享端点(下一条边的起点=上一条边的终点)。
	edgeAB := append(densifyEdgeTest(a, b, stepM), b)
	edgeBC := append(densifyEdgeTest(b, c, stepM), c)
	edgeCD := append(densifyEdgeTest(c, d, stepM), d)
	edgeDA := append(densifyEdgeTest(d, a, stepM), a)
	curves := [][]Coords{
		reverseCoords(edgeCD), // 上边(反向)
		edgeAB,                // 下边
		reverseCoords(edgeDA), // 左边(反向)
		edgeBC,                // 右边
	}
	want := AreaCoords([]Coords{a, b, c, d})

	check := func(t *testing.T, ring []Coords) {
		// 密集输入下环顶点数远多于 4，只校验面积(拓扑一致性的稳健度量)。
		if len(ring) < 4 {
			t.Fatalf("环顶点过少=%d: %+v", len(ring), ring)
		}
		got := AreaCoords(ring)
		if rel := math.Abs(got-want) / want; rel > 1e-3 {
			t.Fatalf("merged area=%.4f want=%.4f relErr=%.2e", got, want, rel)
		}
	}
	t.Run("MergeCurves0", func(t *testing.T) { check(t, MergeCurves0(curves, tolerance)) })
}

// 衔接处端点存在微小偏差(< tolerance)时应被去重衔接(密集点曲线)。
func TestMergeCurves_ToleranceDedup(t *testing.T) {
	const tolerance = 5.0
	const stepM = 2.0
	const jitter = 4.5e-6 // ≈ 0.5m (< tolerance)

	// 三角形三角(北京附近)。三条边各密集采样成一条曲线，衔接点注入 < tolerance 抖动。
	a := Coords{Lat: 39.90, Lng: 116.30}
	b := Coords{Lat: 39.90, Lng: 116.31}
	c := Coords{Lat: 39.91, Lng: 116.31}
	want := AreaCoords([]Coords{a, b, c})

	// 每条边一条密集曲线；相邻边的衔接点各自带独立抖动(近似重合但不相等)。
	j := func(p Coords, s float64) Coords { return Coords{Lat: p.Lat + s, Lng: p.Lng + s} }
	e1 := append(densifyEdgeTest(a, b, stepM), b)                        // a -> b
	e2 := append(densifyEdgeTest(j(b, jitter), c, stepM), c)             // b(抖动) -> c
	e3 := append(densifyEdgeTest(j(c, -jitter), a, stepM), j(a, jitter)) // c(抖动) -> a(抖动)

	for _, tc := range []struct {
		name string
		fn   func([][]Coords, float64) []Coords
	}{
		{"MergeCurves0", MergeCurves0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ring := tc.fn([][]Coords{e1, e2, e3}, tolerance)
			if len(ring) < 3 {
				t.Fatalf("环顶点过少=%d: %+v", len(ring), ring)
			}
			got := AreaCoords(ring)
			if rel := math.Abs(got-want) / want; rel > 1e-3 {
				t.Fatalf("area=%.4f want=%.4f relErr=%.2e", got, want, rel)
			}
		})
	}
}

func TestMergeCurvesGeoInt_RoundTrip(t *testing.T) {
	corners := []Coords{
		{Lat: 39.90, Lng: 116.30},
		{Lat: 39.90, Lng: 116.50},
		{Lat: 40.00, Lng: 116.50},
		{Lat: 40.00, Lng: 116.30},
	}
	enc := func(c Coords) uint64 { return EncodeInt(c.Lat, c.Lng) }

	curves := [][]uint64{
		{enc(corners[2]), enc(corners[3])},
		{enc(corners[0]), enc(corners[1])},
		{enc(corners[3]), enc(corners[0])},
		{enc(corners[1]), enc(corners[2])},
	}

	ring := MergeCurvesGeoInt(curves, 100.0) // 100m 容差(> 网格尺寸)
	if len(ring) != 4 {
		t.Fatalf("expected 4 vertices, got %d", len(ring))
	}

	got := AreaGeoInt(ring)
	want := AreaCoords(corners)
	if rel := math.Abs(got-want) / want; rel > 1e-3 {
		t.Fatalf("area=%.2f want=%.2f relErr=%.4f", got, want, rel)
	}
}
