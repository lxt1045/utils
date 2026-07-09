package geohash

import (
	"math"
	"testing"
)

// 本文件验证 MergeCurves(拼接无序曲线成闭合环)的正确性。
//
// 背景：MergeCurves 在功能上等价于 JTS 的 LineMerger / GEOS 的 GEOSLineMerge /
// PostGIS 的 ST_LineMerge——把一组无序、方向不定、端点共享的折线缝合成连续线串
// (进而闭合成环)。这些参照实现都不是纯 Go(Java / C++ / GEOS+CGO)，无法在本仓库
// 内直接调用做逐点比对。
//
// 但 MergeCurves 属于"拓扑重建":给定某个已知多边形的边界被打散后的曲线集合，
// 正确的输出就应当还原出这个多边形。因此可以用"重建环所围面积"作为误差度量,
// 并借助本仓库已有的两个独立参照库交叉验证:
//   - paulmach/orb        : 球面角盈公式(orbArea, 见 area_compare_test.go)
//   - tidwall/geodesic    : WGS84 椭球测地真值(geodesicArea, 见 area_compare_test.go)
//
// 结论(由实测得出):在容差(tolerance)大于关节处抖动的前提下,MergeCurves 重建
// 的环与真实边界在拓扑上完全一致,所围面积相对 orb 与椭球真值的误差,与直接用
// 真实边界计算面积的误差完全同量级(即误差只来自面积公式本身的球/椭球近似,
// 拼接过程不引入额外误差)。

// splitRingIntoScrambledCurves 把一个闭合环(顶点列表,不含重复首尾)的边界拆成
// nCurves 段曲线,并故意:打乱段的顺序、随机反转部分段的方向、在关节处注入一个
// 小于 jitterDeg 的微小偏移(模拟真实数据里各图幅端点因精度不完全重合的情况)。
//
// 每条边会被密集重采样，使相邻点间距 < tolerance/2——满足 MergeCurves2 的「密集点
// 曲线」前提(粘接点靠点距离在网格里反查)。denseStepM 是重采样步长(米)。
//
// 返回的曲线集合恰好覆盖整条边界且相邻段共享端点,适合喂给 MergeCurves / MergeCurves2。
func splitRingIntoScrambledCurves(ring []Coords, nCurves int, jitterDeg, denseStepM float64) [][]Coords {
	n := len(ring)
	// 闭合边界的有向顶点序列(把首点再接到末尾,形成 n 条边、n+1 个点)。
	closed := make([]Coords, 0, n+1)
	closed = append(closed, ring...)
	closed = append(closed, ring[0])

	// 把 n 条边尽量均匀地切成 nCurves 段(每段是若干条相连的边)。
	edges := n // 边数
	base := edges / nCurves
	rem := edges % nCurves

	// densifyEdge 在 [u,v] 上按 denseStepM 步长插值(含 u，不含 v)，保证相邻点间距 < step。
	densifyEdge := func(u, v Coords) []Coords {
		d := distMeters(u, v)
		steps := int(d/denseStepM) + 1
		out := make([]Coords, 0, steps)
		for k := 0; k < steps; k++ {
			f := float64(k) / float64(steps)
			out = append(out, Coords{Lat: u.Lat + (v.Lat-u.Lat)*f, Lng: u.Lng + (v.Lng-u.Lng)*f})
		}
		return out
	}

	segments := make([][]Coords, 0, nCurves)
	idx := 0
	for s := range nCurves {
		cnt := base
		if s < rem {
			cnt++ // 前 rem 段各多分一条边
		}
		// 密集重采样该段包含的 cnt 条边，最后补上段终点。
		seg := make([]Coords, 0, cnt*8+1)
		for k := 0; k < cnt; k++ {
			seg = append(seg, densifyEdge(closed[idx+k], closed[idx+k+1])...)
		}
		seg = append(seg, closed[idx+cnt]) // 段终点
		idx += cnt
		segments = append(segments, seg)
	}

	// 在每个关节处注入微小抖动:把每段的两个端点各推移一点点(< jitterDeg)。
	// 用确定性的伪随机(基于索引)保证测试可复现。
	jitter := func(c Coords, seed int) Coords {
		// 取一个 [-1,1] 的确定性因子
		f := math.Sin(float64(seed)*12.9898) * 43758.5453
		f = f - math.Floor(f) // 小数部分 [0,1)
		f = f*2 - 1           // [-1,1)
		return Coords{Lat: c.Lat + f*jitterDeg, Lng: c.Lng + f*jitterDeg}
	}
	for si := range segments {
		seg := segments[si]
		seg[0] = jitter(seg[0], si*7+1)
		seg[len(seg)-1] = jitter(seg[len(seg)-1], si*7+3)
	}

	// 随机反转部分段的方向(确定性:偶数段反转)。
	for si := range segments {
		if si%2 == 0 {
			segments[si] = reverseCoords(segments[si])
		}
	}

	// 打乱段的先后顺序(确定性置换:旋转 + 整体反转)。
	// 旋转和反转对任意 nCurves 都是合法置换,不会像 (i*k)%n 那样在
	// gcd(k,n)!=1 时把多个下标折叠到同一位置而丢段。
	n2 := len(segments)
	rot := n2 / 2
	scrambled := make([][]Coords, n2)
	for si := range segments {
		scrambled[si] = segments[(si+rot)%n2]
	}
	for i, j := 0, n2-1; i < j; i, j = i+1, j-1 {
		scrambled[i], scrambled[j] = scrambled[j], scrambled[i]
	}
	return scrambled
}

// TestMergeCurves_CompareAreaWithLibraries 验证:把已知多边形边界打散后再 MergeCurves
// 重建,重建环的面积与"真实边界面积"在 orb / 椭球真值下误差同量级,拼接不引入额外误差。
func TestMergeCurves_CompareAreaWithLibraries(t *testing.T) {
	cases := []struct {
		name    string
		ring    []Coords
		nCurves int
	}{
		{
			name: "beijing-block-4curves",
			ring: []Coords{
				{Lat: 39.900, Lng: 116.300},
				{Lat: 39.900, Lng: 116.500},
				{Lat: 40.000, Lng: 116.500},
				{Lat: 40.000, Lng: 116.300},
			},
			nCurves: 4,
		},
		{
			name: "octagon-3curves", // 8 顶点八边形,拆成 3 段(段边数不均匀)
			ring: func() []Coords {
				c := []Coords{}
				cx, cy, r := 116.4, 39.9, 0.1
				for k := range 8 {
					ang := 2 * math.Pi * float64(k) / 8
					c = append(c, Coords{Lat: cy + r*math.Sin(ang), Lng: cx + r*math.Cos(ang)})
				}
				return c
			}(),
			nCurves: 3,
		},
		{
			name: "highlat-hexagon-5curves",
			ring: func() []Coords {
				c := []Coords{}
				cx, cy, r := 10.0, 60.0, 0.2
				for k := range 6 {
					ang := 2 * math.Pi * float64(k) / 6
					c = append(c, Coords{Lat: cy + r*math.Sin(ang), Lng: cx + r*math.Cos(ang)})
				}
				return c
			}(),
			nCurves: 5,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// 关节抖动约 0.5m(≈ 4.5e-6 度),容差取 5m 以覆盖抖动。
			const jitterDeg = 4.5e-6
			const tolerance = 5.0
			const denseStepM = 2.0 // < tolerance/2,满足 MergeCurves2 密集点前提

			curves := splitRingIntoScrambledCurves(tc.ring, tc.nCurves, jitterDeg, denseStepM)
			merged := MergeCurves(curves, tolerance)

			// MergeCurves 用 snap-rounding 聚类密集点,合并后顶点数会明显少于密集
			// 输入(tolerance 内的点塌成单节点)。跳过顶点数检查,只校验面积拓扑。
			if len(merged) < len(tc.ring) {
				t.Fatalf("重建环顶点数=%d < 原环=%d, 拼接拓扑断裂: %+v",
					len(merged), len(tc.ring), merged)
			}

			// 三种面积:本包 / orb / 椭球真值,均基于重建环。
			mineMerged := AreaCoords(merged)
			orbMerged := orbArea(merged)
			geoMerged := geodesicArea(merged)

			// 参照:直接用"真实边界"算的面积(同样三种库)。
			mineTruth := AreaCoords(tc.ring)
			geoTruth := geodesicArea(tc.ring)

			// (1) 拼接前后本包面积应几乎相等(仅关节抖动带来的极小差异)。
			relMerge := math.Abs(mineMerged-mineTruth) / mineTruth
			// (2) 重建环相对椭球真值(用真实边界算)的误差。
			relGeo := math.Abs(mineMerged-geoTruth) / geoTruth
			// (3) 重建环上 本包 vs orb 的差异(应恒约 0.224%,球半径差异)。
			relOrb := math.Abs(mineMerged-orbMerged) / geoMerged

			t.Logf("重建环: AreaCoords=%.3f  orb=%.3f  geodesic=%.3f", mineMerged, orbMerged, geoMerged)
			t.Logf("真实边界: AreaCoords=%.3f  geodesic=%.3f", mineTruth, geoTruth)
			t.Logf("拼接引入误差(本包 前后)=%.6f%%  重建→椭球真值=%.4f%%  重建 本包→orb=%.4f%%",
				relMerge*100, relGeo*100, relOrb*100)

			// 拼接本身不应引入可观误差:关节抖动 ~0.5m 对城市街区/公里级多边形
			// 面积的相对影响应远小于 0.1%。
			if relMerge > 1e-3 {
				t.Errorf("拼接前后面积差异过大, 说明重建引入了额外误差: relErr=%.6f", relMerge)
			}
			// 重建环相对椭球真值的误差,应与 area_compare_test 同量级(< 0.6%)。
			if relGeo > 6e-3 {
				t.Errorf("重建环相对椭球真值误差过大: relErr=%.6f", relGeo)
			}
			// 重建环上 本包 vs orb 的差异仍是球半径之差,应 < 0.3%。
			if relOrb > 3e-3 {
				t.Errorf("重建环 本包 vs orb 差异过大: relErr=%.6f", relOrb)
			}
		})

		t.Run(tc.name+"-MergeCurves2", func(t *testing.T) {
			// 关节抖动约 0.5m(≈ 4.5e-6 度),容差取 5m 以覆盖抖动。
			const jitterDeg = 4.5e-6
			const tolerance = 5.0
			const denseStepM = 2.0 // < tolerance/2，满足 MergeCurves2 的密集点前提

			curves := splitRingIntoScrambledCurves(tc.ring, tc.nCurves, jitterDeg, denseStepM)
			merged := MergeCurves2(curves, tolerance)

			// 密集重采样后重建环含大量顶点，不再等于原环顶点数；用面积校验拓扑正确性。
			if len(merged) < len(tc.ring) {
				t.Fatalf("重建环顶点数=%d, 过少(至少应有原环 %d 个拐点)", len(merged), len(tc.ring))
			}

			// 三种面积:本包 / orb / 椭球真值,均基于重建环。
			mineMerged := AreaCoords(merged)
			orbMerged := orbArea(merged)
			geoMerged := geodesicArea(merged)

			// 参照:直接用"真实边界"算的面积(同样三种库)。
			mineTruth := AreaCoords(tc.ring)
			geoTruth := geodesicArea(tc.ring)

			// (1) 拼接前后本包面积应几乎相等(仅关节抖动带来的极小差异)。
			relMerge := math.Abs(mineMerged-mineTruth) / mineTruth
			// (2) 重建环相对椭球真值(用真实边界算)的误差。
			relGeo := math.Abs(mineMerged-geoTruth) / geoTruth
			// (3) 重建环上 本包 vs orb 的差异(应恒约 0.224%,球半径差异)。
			relOrb := math.Abs(mineMerged-orbMerged) / geoMerged

			t.Logf("重建环: AreaCoords=%.3f  orb=%.3f  geodesic=%.3f", mineMerged, orbMerged, geoMerged)
			t.Logf("真实边界: AreaCoords=%.3f  geodesic=%.3f", mineTruth, geoTruth)
			t.Logf("拼接引入误差(本包 前后)=%.6f%%  重建→椭球真值=%.4f%%  重建 本包→orb=%.4f%%",
				relMerge*100, relGeo*100, relOrb*100)

			// 拼接本身不应引入可观误差:关节抖动 ~0.5m 对城市街区/公里级多边形
			// 面积的相对影响应远小于 0.1%。
			if relMerge > 1e-3 {
				t.Errorf("拼接前后面积差异过大, 说明重建引入了额外误差: relErr=%.6f", relMerge)
			}
			// 重建环相对椭球真值的误差,应与 area_compare_test 同量级(< 0.6%)。
			if relGeo > 6e-3 {
				t.Errorf("重建环相对椭球真值误差过大: relErr=%.6f", relGeo)
			}
			// 重建环上 本包 vs orb 的差异仍是球半径之差,应 < 0.3%。
			if relOrb > 3e-3 {
				t.Errorf("重建环 本包 vs orb 差异过大: relErr=%.6f", relOrb)
			}
		})
	}
}
