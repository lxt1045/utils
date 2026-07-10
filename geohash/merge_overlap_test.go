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
