//go:build !windows && cgo

package geos_test

import (
	"math"
	"testing"

	"geos"

	"github.com/lxt1045/utils/geohash"
	"github.com/peterstace/simplefeatures/geom"
)

// curvesToMultiLineWKT 用 simplefeatures 的 geom 把多条坐标曲线构造成一个
// MULTILINESTRING 几何，并输出其 WKT。simplefeatures 的坐标是 (X,Y)=(Lng,Lat)。
func curvesToMultiLineWKT(t *testing.T, curves [][]geohash.Coords) string {
	t.Helper()
	lines := make([]geom.LineString, 0, len(curves))
	for _, cv := range curves {
		xys := make([]float64, 0, len(cv)*2)
		for _, c := range cv {
			xys = append(xys, c.Lng, c.Lat)
		}
		ls := geom.NewLineString(geom.NewSequence(xys, geom.DimXY))
		lines = append(lines, ls)
	}
	return geom.NewMultiLineString(lines).AsText()
}

// wktToRing 用 simplefeatures 的 geom 解析 GEOSLineMerge 输出的 WKT，
// 取出其顶点序列并转回 geohash.Coords。要求结果是单条 LINESTRING
// (即所有曲线被成功缝合成一条线)；否则返回 ok=false。
// 若线首尾在 tolerance 内闭合，则去掉重复的尾点，与 MergeCurves 的约定一致。
func wktToRing(t *testing.T, wkt string, tolerance float64) (ring []geohash.Coords, ok bool) {
	t.Helper()
	g, err := geom.UnmarshalWKT(wkt)
	if err != nil {
		t.Fatalf("geom.UnmarshalWKT(%q) 失败: %v", wkt, err)
	}
	if g.Type() != geom.TypeLineString {
		return nil, false // 未缝合成单条线(仍是 MultiLineString)
	}
	seq := g.MustAsLineString().Coordinates()
	n := seq.Length()
	ring = make([]geohash.Coords, n)
	for i := 0; i < n; i++ {
		xy := seq.GetXY(i)
		ring[i] = geohash.Coords{Lat: xy.Y, Lng: xy.X}
	}
	// 去掉与首点重合的尾点(闭合环)。
	if len(ring) > 1 && geohash.DistHaversine(ring[0], ring[len(ring)-1]) <= tolerance {
		ring = ring[:len(ring)-1]
	}
	return ring, true
}

// ringsMatch 判断两条闭合环是否表示同一多边形：顶点数相同，且存在某个
// 旋转偏移(可含反向)使得逐点距离都 <= tolerance。返回最大顶点误差(米)。
func ringsMatch(a, b []geohash.Coords, tolerance float64) (maxErr float64, matched bool) {
	if len(a) != len(b) || len(a) == 0 {
		return math.Inf(1), false
	}
	n := len(a)
	try := func(idx func(i, off int) int) (float64, bool) {
		best := math.Inf(1)
		for off := 0; off < n; off++ {
			cur, ok := 0.0, true
			for i := 0; i < n; i++ {
				d := geohash.DistHaversine(a[i], b[idx(i, off)])
				if d > tolerance {
					ok = false
					break
				}
				if d > cur {
					cur = d
				}
			}
			if ok && cur < best {
				best = cur
			}
		}
		return best, !math.IsInf(best, 1)
	}
	// 正向
	if e, ok := try(func(i, off int) int { return (i + off) % n }); ok {
		return e, true
	}
	// 反向
	if e, ok := try(func(i, off int) int { return ((off - i)%n + n) % n }); ok {
		return e, true
	}
	return math.Inf(1), false
}

// densifyCurve 把一条折线按 stepM 步长在每条边上线性加密，使相邻点间距 < stepM，
// 满足 MergeCurves2 的「密集点曲线」前提。原始顶点(含首尾)被 bit 级精确保留——
// 这样相邻曲线共享的端点在加密后仍完全相同，GEOS LineMerge 才能缝合。
func densifyCurve(curve []geohash.Coords, stepM float64) []geohash.Coords {
	if len(curve) < 2 {
		return append([]geohash.Coords(nil), curve...)
	}
	out := make([]geohash.Coords, 0, len(curve)*8)
	for i := 0; i+1 < len(curve); i++ {
		u, v := curve[i], curve[i+1]
		d := geohash.DistHaversine(u, v)
		steps := int(d/stepM) + 1
		for k := 0; k < steps; k++ {
			f := float64(k) / float64(steps)
			out = append(out, geohash.Coords{Lat: u.Lat + (v.Lat-u.Lat)*f, Lng: u.Lng + (v.Lng-u.Lng)*f})
		}
	}
	return append(out, curve[len(curve)-1])
}

func TestMergeCurves_CompareWithGEOSLineMerge(t *testing.T) {
	// 构造测试用例：给定已知多边形顶点，切成若干段、打乱顺序与方向。
	// GEOSLineMerge 要求衔接端点“精确重合”才会缝合，因此这里不注入抖动
	// (抖动场景在 tolerance 子测试里单独验证 MergeCurves 的鲁棒性)。
	mkPolygonCurves := func(verts []geohash.Coords, segLens []int, reverseEven bool) [][]geohash.Coords {
		// 闭合：把首点接到末尾。
		closed := append(append([]geohash.Coords(nil), verts...), verts[0])
		curves := [][]geohash.Coords{}
		idx := 0
		for si, ln := range segLens {
			seg := make([]geohash.Coords, 0, ln+1)
			for k := 0; k <= ln; k++ {
				seg = append(seg, closed[idx+k])
			}
			idx += ln
			if reverseEven && si%2 == 0 {
				for i, j := 0, len(seg)-1; i < j; i, j = i+1, j-1 {
					seg[i], seg[j] = seg[j], seg[i]
				}
			}
			curves = append(curves, seg)
		}
		return curves
	}

	regularPolygon := func(cx, cy, r float64, sides int) []geohash.Coords {
		v := make([]geohash.Coords, sides)
		for k := 0; k < sides; k++ {
			ang := 2 * math.Pi * float64(k) / float64(sides)
			v[k] = geohash.Coords{Lat: cy + r*math.Sin(ang), Lng: cx + r*math.Cos(ang)}
		}
		return v
	}

	cases := []struct {
		name     string
		verts    []geohash.Coords
		segLens  []int // 各段包含的边数，之和须等于顶点数
		tolerance float64
	}{
		{
			name: "beijing-square-4curves",
			verts: []geohash.Coords{
				{Lat: 39.90, Lng: 116.30},
				{Lat: 39.90, Lng: 116.50},
				{Lat: 40.00, Lng: 116.50},
				{Lat: 40.00, Lng: 116.30},
			},
			segLens:   []int{1, 1, 1, 1},
			tolerance: 1.0,
		},
		{
			name:      "octagon-3curves-uneven",
			verts:     regularPolygon(116.4, 39.9, 0.1, 8),
			segLens:   []int{3, 3, 2},
			tolerance: 1.0,
		},
		{
			name:      "highlat-hexagon-2curves",
			verts:     regularPolygon(10.0, 60.0, 0.2, 6),
			segLens:   []int{3, 3},
			tolerance: 1.0,
		},
		{
			name:      "pentagon-5curves",
			verts:     regularPolygon(-70.0, -33.0, 0.15, 5),
			segLens:   []int{1, 1, 1, 1, 1},
			tolerance: 1.0,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			curves := mkPolygonCurves(tc.verts, tc.segLens, true)

			// —— 本包 MergeCurves ——
			mine := geohash.MergeCurves(curves, tc.tolerance)

			// —— GEOS 参照：simplefeatures geom 造 WKT → GEOSLineMerge → geom 解析 ——
			inWKT := curvesToMultiLineWKT(t, curves)
			outWKT, err := geos.LineMergeWKT(inWKT)
			if err != nil {
				t.Fatalf("GEOSLineMerge 失败: %v", err)
			}
			geosRing, ok := wktToRing(t, outWKT, tc.tolerance)
			if !ok {
				t.Fatalf("GEOSLineMerge 未能缝合成单条线, 输出=%s", outWKT)
			}

			// —— 顶点数应与原多边形一致 ——
			if len(mine) != len(tc.verts) {
				t.Errorf("MergeCurves 顶点数=%d, 期望=%d", len(mine), len(tc.verts))
			}
			if len(geosRing) != len(tc.verts) {
				t.Errorf("GEOSLineMerge 顶点数=%d, 期望=%d", len(geosRing), len(tc.verts))
			}

			// —— 两者拓扑应一致(允许旋转/反向), 顶点误差应极小 ——
			maxErr, matched := ringsMatch(mine, geosRing, tc.tolerance)
			if !matched {
				t.Fatalf("MergeCurves 与 GEOSLineMerge 结果不一致\n  mine =%+v\n  geos =%+v",
					mine, geosRing)
			}

			// —— 面积对比: 两条环各自求面积, 相对误差 ——
			areaMine := geohash.AreaCoords(mine)
			areaGeos := geohash.AreaCoords(geosRing)
			relArea := math.Abs(areaMine-areaGeos) / areaGeos

			t.Logf("顶点数=%d  最大顶点误差=%.3e m  面积: mine=%.3f geos=%.3f 相对误差=%.3e",
				len(mine), maxErr, areaMine, areaGeos, relArea)

			// 端点精确重合的情况下, 两者应还原出同一组顶点, 误差应在浮点噪声级(< 1e-6 m)。
			if maxErr > 1e-6 {
				t.Errorf("顶点误差过大: %.3e m", maxErr)
			}
			if relArea > 1e-9 {
				t.Errorf("面积相对误差过大: %.3e", relArea)
			}
		})

		// —— MergeCurves2 密集点子测试 ——
		// MergeCurves2 前提:输入是密集点曲线(相邻点间距 < tolerance/2)。把每条曲线
		// 加密后既满足该前提,又因原始顶点被精确保留、共享端点仍完全相同,GEOS LineMerge
		// 照样能缝合。用较大 tolerance/step 控制加密点数(小 tolerance 会产生海量点)。
		t.Run(tc.name+"-MergeCurves2", func(t *testing.T) {
			const tol2 = 100.0
			const stepM = 40.0 // < tol2/2，满足密集点前提
			raw := mkPolygonCurves(tc.verts, tc.segLens, true)
			curves := make([][]geohash.Coords, len(raw))
			for i, c := range raw {
				curves[i] = densifyCurve(c, stepM)
			}

			// —— 本包 MergeCurves2 ——
			mine := geohash.MergeCurves2(curves, tol2)
			if len(mine) < len(tc.verts) {
				t.Fatalf("MergeCurves2 顶点数=%d < 原多边形顶点数=%d", len(mine), len(tc.verts))
			}

			// —— GEOS 参照:同一批密集曲线 → GEOSLineMerge → 单条线 ——
			inWKT := curvesToMultiLineWKT(t, curves)
			outWKT, err := geos.LineMergeWKT(inWKT)
			if err != nil {
				t.Fatalf("GEOSLineMerge 失败: %v", err)
			}
			geosRing, ok := wktToRing(t, outWKT, tol2)
			if !ok {
				t.Fatalf("GEOSLineMerge 未能缝合成单条线, 输出前缀=%.120s", outWKT)
			}

			// 密集环顶点数多且两者重采样点未必逐点对齐,用面积做拓扑一致性度量。
			areaMine := geohash.AreaCoords(mine)
			areaGeos := geohash.AreaCoords(geosRing)
			relArea := math.Abs(areaMine-areaGeos) / areaGeos
			t.Logf("顶点数: mine=%d geos=%d  面积: mine=%.3f geos=%.3f 相对误差=%.3e",
				len(mine), len(geosRing), areaMine, areaGeos, relArea)
			if relArea > 1e-5 {
				t.Errorf("MergeCurves2 与 GEOSLineMerge 面积相对误差过大: %.3e", relArea)
			}
		})
	}
}

// TestMergeCurves_ToleranceRobustness 验证 MergeCurves 在“衔接端点有微小抖动”
// (真实数据常见)时仍能缝合成环，而 GEOSLineMerge 因要求端点精确重合会失败——
// 这正是 MergeCurves 相对裸 GEOSLineMerge 的实用价值。抖动去除后两者面积应一致。
func TestMergeCurves_ToleranceRobustness(t *testing.T) {
	verts := []geohash.Coords{
		{Lat: 40.0, Lng: 116.0},
		{Lat: 40.0, Lng: 116.1},
		{Lat: 40.1, Lng: 116.1},
		{Lat: 40.1, Lng: 116.0},
	}
	const jitter = 5e-6 // ≈ 0.5m 量级
	curves := [][]geohash.Coords{
		{verts[0], verts[1]},
		{{Lat: verts[1].Lat + jitter, Lng: verts[1].Lng}, verts[2]},
		{{Lat: verts[2].Lat, Lng: verts[2].Lng + jitter}, verts[3]},
		{{Lat: verts[3].Lat - jitter, Lng: verts[3].Lng}, verts[0]},
	}

	// MergeCurves 用 5m 容差应能缝合成 4 顶点环。
	mine := geohash.MergeCurves(curves, 5.0)
	if len(mine) != 4 {
		t.Fatalf("MergeCurves 抖动场景应得到 4 顶点环, 实际=%d: %+v", len(mine), mine)
	}

	// GEOSLineMerge 对同样(未对齐)的输入：端点不精确重合, 无法缝合成单条线。
	inWKT := curvesToMultiLineWKT(t, curves)
	outWKT, err := geos.LineMergeWKT(inWKT)
	if err != nil {
		t.Fatalf("GEOSLineMerge 失败: %v", err)
	}
	g, err := geom.UnmarshalWKT(outWKT)
	if err != nil {
		t.Fatalf("解析 GEOS 输出失败: %v", err)
	}
	t.Logf("抖动输入下 GEOSLineMerge 输出类型=%s (期望仍为 MultiLineString, 说明未缝合)", g.Type())

	// 面积健全性：MergeCurves 结果面积应接近"真实无抖动多边形"面积。
	areaMine := geohash.AreaCoords(mine)
	areaTruth := geohash.AreaCoords(verts)
	rel := math.Abs(areaMine-areaTruth) / areaTruth
	t.Logf("MergeCurves 面积=%.3f  真值=%.3f  相对误差=%.4e", areaMine, areaTruth, rel)
	if rel > 1e-3 {
		t.Errorf("抖动去除后面积相对误差过大: %.4e", rel)
	}

	// —— MergeCurves2 抖动子测试:密集化后同样能缝合,与 MergeCurves 面积一致 ——
	const tol2 = 5.0
	const stepM = 2.0 // < tol2/2，满足密集点前提
	curvesDense := make([][]geohash.Coords, len(curves))
	for i, c := range curves {
		curvesDense[i] = densifyCurve(c, stepM)
	}
	mine2 := geohash.MergeCurves2(curvesDense, tol2)
	if len(mine2) < 4 {
		t.Fatalf("MergeCurves2 抖动场景应得到至少 4 顶点环, 实际=%d: %+v", len(mine2), mine2)
	}
	areaMine2 := geohash.AreaCoords(mine2)
	rel2 := math.Abs(areaMine2-areaTruth) / areaTruth
	t.Logf("MergeCurves2(密集) 顶点数=%d 面积=%.3f  相对误差=%.4e", len(mine2), areaMine2, rel2)
	if rel2 > 1e-3 {
		t.Errorf("MergeCurves2 抖动去除后面积相对误差过大: %.4e", rel2)
	}
}
