package geohash

import (
	"math"
	"testing"
)

// TestMergeCurves_MidSegmentOverlap 验证「同一段边界被数字化两次、且两次重采样
// 不对齐」时，snap-rounding 能消除重叠、还原单环，面积与无重叠版一致。
//
// 关键：重叠段特意取一条**非共线的折线(带尖角的 bump)**——若重叠未被消除、被
// 重复遍历，环会自交/腰折，面积会明显偏离。共线的重叠段无法暴露这种错误(共线冗余
// 点不改变面积)，故这里必须用非共线段。
//
// 机制说明：两套数字化虽顶点不对齐，但都贴着同一条真实折线(彼此 < tolerance)。
// noding 时，一套的顶点会作为“热像素”被插入到另一套的线段上(反之亦然)，于是两套
// 得到**相同的节点序列**，边去重把它们塌成一条。这正是 snap-rounding 的核心。
func TestMergeCurves_MidSegmentOverlap(t *testing.T) {
	// 多边形：左边界 D→A、底部一段带尖角的 bump A→P→B、右边界 B→C、顶 C→D。
	// 顶点(Lat,Lng)。tolerance 取 50m，各顶点间距均 >> tolerance。
	A := Coords{Lat: 39.900, Lng: 116.300}
	P := Coords{Lat: 39.880, Lng: 116.350} // bump 尖角(向南凸出)，令 A→P→B 非共线
	B := Coords{Lat: 39.900, Lng: 116.400}
	C := Coords{Lat: 40.000, Lng: 116.400}
	D := Coords{Lat: 40.000, Lng: 116.300}
	const tolerance = 50.0

	// 参照面积：干净多边形 A,P,B,C,D(bump 只走一次)。
	wantArea := AreaCoords([]Coords{A, P, B, C, D})

	// 在直线段上按参数插值取一点(用于制造两套不同的重采样，且严格落在段上)。
	lerp := func(u, v Coords, f float64) Coords {
		return Coords{Lat: u.Lat + (v.Lat-u.Lat)*f, Lng: u.Lng + (v.Lng-u.Lng)*f}
	}
	// 数字化 1：bump A→P→B，A→P 上取 2 个中间点、P→B 上取 1 个。
	dig1 := []Coords{A, lerp(A, P, 0.35), lerp(A, P, 0.70), P, lerp(P, B, 0.5), B}
	// 数字化 2：同一条 bump，但取样点完全不同：A→P 上 1 个、P→B 上 2 个。
	dig2 := []Coords{A, lerp(A, P, 0.55), P, lerp(P, B, 0.33), lerp(P, B, 0.66), B}
	// 其余边界(单独一条曲线)。
	rest := []Coords{B, C, D, A}

	curves := [][]Coords{dig1, rest, dig2} // 顺序打乱

	ring := MergeCurves(curves, tolerance)
	if len(ring) < 5 {
		t.Fatalf("环顶点数=%d, 过少(应至少还原 A,P,B,C,D 五个拐点): %+v", len(ring), ring)
	}
	gotArea := AreaCoords(ring)
	rel := math.Abs(gotArea-wantArea) / wantArea
	t.Logf("环顶点数=%d 面积=%.3f 参照=%.3f 相对误差=%.3e", len(ring), gotArea, wantArea, rel)
	// bump 非共线：若重叠未消除、被重复遍历，环会自交/腰折，面积明显偏离。
	if rel > 1e-3 {
		t.Errorf("面积相对误差过大=%.3e, 疑似重叠未消除(bump 被重复计入)", rel)
	}
}

// BenchmarkMergeCurvesScaling 在不同顶点数下压测 MergeCurves，展示其近 O(V) 的标度。
//
// 关键：snap-rounding 的 noding 成本主要在沿线段扫格的 DDA，其代价正比于
// 「边界总长 / cell 尺寸」，与顶点数无关。若固定半径(边界总长恒定)只增顶点数，
// DDA 项会是一个与 V 无关的大常数，掩盖真正的标度关系。这里让半径随 V 增长，使
// **每段边长固定为 tolerance 的若干倍**(真实数字化密度)：边界总长 ∝ V、DDA = O(V)。
func BenchmarkMergeCurvesScaling(b *testing.B) {
	const (
		tolerance = 1.0
		segMeters = 8.0 // 每段约 8m (>tolerance 保证节点可区分；~8 cell/段令 DDA∝V)
	)
	for _, nCurves := range []int{50, 200, 800, 3200} {
		// 半径随 V 增长，保持每段边长 ≈ segMeters：边界总长 = segMeters*nCurves。
		radiusDeg := segMeters * float64(nCurves) / (2 * math.Pi) / 111320.0
		curves := buildScrambledRingCurves(nCurves, 2, radiusDeg, 0, 42)

		b.Run(itoa(nCurves), func(b *testing.B) {
			b.ReportAllocs()
			for range b.N {
				_ = MergeCurves(curves, tolerance)
			}
		})
		b.Run(itoa(nCurves), func(b *testing.B) {
			b.ReportAllocs()
			for range b.N {
				_ = MergeCurves3(curves, tolerance)
			}
		})
	}
}

// mainClosedArea 返回一组 Ring 中最大闭合环的面积与闭合环条数。
func mainClosedArea(rings []Ring) (area float64, closedCnt int) {
	for _, r := range rings {
		if !r.IsClosed {
			continue
		}
		closedCnt++
		if a := AreaCoords(r.Coords); a > area {
			area = a
		}
	}
	return area, closedCnt
}

// TestMergeCurves4 在多档曲线数下对比 MergeCurves3 与 MergeCurves4 的等效性：
// 两者都应从同一份「打乱的干净圆环」还原出唯一的闭合主环，且面积逐点一致。
func TestMergeCurves4(t *testing.T) {
	const (
		tolerance = 1.0
		segMeters = 8.0 // 每段约 8m (>tolerance 保证节点可区分；~8 cell/段令 DDA∝V)
	)

	for _, nCurves := range []int{50, 100, 200, 500, 800} {
		// 半径随 V 增长，保持每段边长 ≈ segMeters：边界总长 = segMeters*nCurves。
		radiusDeg := segMeters * float64(nCurves) / (2 * math.Pi) / 111320.0
		curves := buildScrambledRingCurves(nCurves, 2, radiusDeg, 0, 42)

		rings3 := MergeCurves3(curves, tolerance)
		rings4 := MergeCurves4(curves, tolerance)

		area3, closed3 := mainClosedArea(rings3)
		area4, closed4 := mainClosedArea(rings4)

		relDiff := 0.0
		if area3 > 0 {
			relDiff = math.Abs(area3-area4) / area3
		}
		t.Logf("[%d 条] MergeCurves3: %d 链/%d 闭合 主环面积=%.6f | MergeCurves4: %d 链/%d 闭合 主环面积=%.6f | 相对差=%.3e",
			nCurves, len(rings3), closed3, area3, len(rings4), closed4, area4, relDiff)

		if closed3 == 0 {
			t.Errorf("[%d 条] MergeCurves3 未产出闭合环", nCurves)
		}
		if closed4 == 0 {
			t.Errorf("[%d 条] MergeCurves4 未产出闭合环", nCurves)
		}
		if area3 > 0 && area4 > 0 && relDiff > 1e-6 {
			t.Errorf("[%d 条] 两者主环面积差异过大: %.3e", nCurves, relDiff)
		}
	}
}

// addOverlapPath 追加一条「中段重叠」曲线：把 curves[0] 的前 nOverlap 个顶点反向
// 精确复制成一条新曲线。这段边界因此被数字化了两次(方向相反)，用于验证两种实现
// 是否都能把重复段塌成一条、不重复计入面积。
func addOverlapPath(curves [][]Coords, nOverlap int) [][]Coords {
	src := curves[0]
	if nOverlap > len(src) {
		nOverlap = len(src)
	}
	dup := make([]Coords, nOverlap)
	for i := range dup {
		dup[i] = src[nOverlap-1-i]
	}
	out := make([][]Coords, len(curves), len(curves)+1)
	copy(out, curves)
	return append(out, dup)
}

// TestMergeCurves4_Overlap 在含「中段重叠(重复数字化)」的数据上，多档对比 MergeCurves3
// 与 MergeCurves4：追加一条精确复制的重叠曲线后，两者是否都能消除重叠、还原出与无重叠
// 版一致的主环面积。这是判定二者「完全等效」的关键——干净数据谁都能过，重叠才见真章。
func TestMergeCurves4_Overlap(t *testing.T) {
	const (
		tolerance   = 1.0
		ptsPerCurve = 64
		nOverlap    = 20
	)

	for _, nCurves := range []int{50, 100, 200, 500, 800} {
		// 半径随 V 增长，保持段内点密集(相邻点间距 < tolerance/2)。
		radiusDeg := 4.0 * float64(nCurves*ptsPerCurve) / (2 * math.Pi) / 111320.0 * 0.25
		base := buildScrambledRingCurves(nCurves, ptsPerCurve, radiusDeg, 0, 42)

		// 无重叠真值面积：对 base 求主环面积。
		baseArea3, _ := mainClosedArea(MergeCurves3(base, tolerance))

		overlap := addOverlapPath(base, nOverlap)
		area3, closed3 := mainClosedArea(MergeCurves3(overlap, tolerance))
		area4, closed4 := mainClosedArea(MergeCurves4(overlap, tolerance))

		rel3, rel4 := 0.0, 0.0
		if baseArea3 > 0 {
			rel3 = math.Abs(area3-baseArea3) / baseArea3
			rel4 = math.Abs(area4-baseArea3) / baseArea3
		}
		t.Logf("[%d 条 +%d 重合] 真值面积=%.3f | MergeCurves3: %d 闭合 面积=%.3f 误差=%.3e | MergeCurves4: %d 闭合 面积=%.3f 误差=%.3e",
			nCurves, nOverlap, baseArea3, closed3, area3, rel3, closed4, area4, rel4)

		if closed4 == 0 {
			t.Errorf("[%d 条] MergeCurves4 在重叠数据上未产出闭合环", nCurves)
		}
		if baseArea3 > 0 && rel4 > 1e-3 {
			t.Errorf("[%d 条] MergeCurves4 主环面积偏离无重叠真值过大: %.3e (疑似重叠未消除)", nCurves, rel4)
		}
	}
}

// BenchmarkMergeCurves3vs4 在多档曲线数下对比 MergeCurves3 与 MergeCurves4 的耗时/分配，
// 干净数据与含中段重叠(重复数字化)两种输入各测一遍。
func BenchmarkMergeCurves3vs4(b *testing.B) {
	const (
		tolerance   = 1.0
		ptsPerCurve = 64
		nOverlap    = 20
	)
	for _, nCurves := range []int{50, 100, 200, 500, 800} {
		radiusDeg := 4.0 * float64(nCurves*ptsPerCurve) / (2 * math.Pi) / 111320.0 * 0.25
		base := buildScrambledRingCurves(nCurves, ptsPerCurve, radiusDeg, 0, 42)
		overlap := addOverlapPath(base, nOverlap)

		b.Run("clean/MergeCurves3/"+itoa(nCurves), func(b *testing.B) {
			b.ReportAllocs()
			for range b.N {
				_ = MergeCurves3(base, tolerance)
			}
		})
		b.Run("clean/MergeCurves4/"+itoa(nCurves), func(b *testing.B) {
			b.ReportAllocs()
			for range b.N {
				_ = MergeCurves4(base, tolerance)
			}
		})
		b.Run("overlap/MergeCurves3/"+itoa(nCurves), func(b *testing.B) {
			b.ReportAllocs()
			for range b.N {
				_ = MergeCurves3(overlap, tolerance)
			}
		})
		b.Run("overlap/MergeCurves4/"+itoa(nCurves), func(b *testing.B) {
			b.ReportAllocs()
			for range b.N {
				_ = MergeCurves4(overlap, tolerance)
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
