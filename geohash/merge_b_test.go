package geohash

import (
	"math"
	"math/rand"
	"testing"
)

// buildScrambledRingCurves 构造用于压测 MergeCurves 的数据集。
//
// 做法：取一个大圆环作为多边形边界，均匀采样出若干顶点，再把边界切成
// nCurves 段，每段 ptsPerCurve 个坐标。相邻两段在衔接处本应共享同一个
// 顶点，但这里让两侧各自独立注入一个 < tolerance 的微小抖动(jitterDeg)，
// 使衔接点“近似重合但不完全相等”——差距落在误差范围内，正是真实图幅
// 数据的常见形态。最后随机反转部分段方向并打乱段顺序。
//
// 近似重合坐标的数量：闭合环有 nCurves 个衔接点，每个衔接点由相邻两段
// 各贡献一个坐标，故近似重合坐标共 2*nCurves 个(本例 100 段 => 200 个)。
//
// 关键尺度约束：
//   - 段内相邻采样点间距必须 > tolerance，否则会被 dedupConsecutive 折叠，
//     1024 个点会缩水；radiusDeg=1° 时相邻点间距约 6.8m，远大于 1m 容差。
//   - 衔接抖动必须 < tolerance，否则两侧端点判不出重合而断链。
func buildScrambledRingCurves(nCurves, ptsPerCurve int, radiusDeg, jitterDeg float64, seed int64) [][]Coords {
	rng := rand.New(rand.NewSource(seed))
	const cx, cy = 116.4, 39.9 // 环心(经度, 纬度)，选在北京附近

	// 每段含两端共 ptsPerCurve 个点，贡献 (ptsPerCurve-1) 条边；段间共享端点，
	// 故闭合环的总边数=总顶点数=nCurves*(ptsPerCurve-1)。
	edgesPerCurve := ptsPerCurve - 1
	totalVerts := nCurves * edgesPerCurve
	angleStep := 2 * math.Pi / float64(totalVerts)

	vertexAt := func(k int) Coords {
		ang := float64(k) * angleStep
		return Coords{
			Lat: cy + radiusDeg*math.Sin(ang),
			Lng: cx + radiusDeg*math.Cos(ang),
		}
	}
	jitter := func(c Coords) Coords {
		return Coords{
			Lat: c.Lat + (rng.Float64()*2-1)*jitterDeg,
			Lng: c.Lng + (rng.Float64()*2-1)*jitterDeg,
		}
	}

	curves := make([][]Coords, nCurves)
	for s := 0; s < nCurves; s++ {
		startV := s * edgesPerCurve
		seg := make([]Coords, ptsPerCurve)
		for k := 0; k < ptsPerCurve; k++ {
			v := vertexAt((startV + k) % totalVerts)
			// 仅两端注入抖动，制造与相邻段“近似重合”的衔接；段内部点保持精确。
			if k == 0 || k == ptsPerCurve-1 {
				v = jitter(v)
			}
			seg[k] = v
		}
		curves[s] = seg
	}

	// 随机反转部分段方向(让 MergeCurves 的四种衔接分支都被触发)。
	for s := range curves {
		if rng.Intn(2) == 0 {
			curves[s] = reverseCoords(curves[s])
		}
	}
	// Fisher–Yates 打乱段先后顺序。
	for i := len(curves) - 1; i > 0; i-- {
		j := rng.Intn(i + 1)
		curves[i], curves[j] = curves[j], curves[i]
	}
	return curves
}

// BenchmarkMergeCurves 压测 MergeCurves：
//   - 100 条曲线
//   - 每条曲线 1024 个坐标
//   - 段间约 200 个近似重合的衔接坐标(唯一差距 < tolerance)
func BenchmarkMergeCurves(b *testing.B) {
	const (
		nCurves     = 100
		ptsPerCurve = 1024
		tolerance   = 1.0    // 米
		radiusDeg   = 1.0    // ~111km 半径；段内相邻点间距 ~6.8m (> tolerance)
		jitterDeg   = 1.5e-6 // ~0.17m (< tolerance)，制造近似重合
	)

	curves := buildScrambledRingCurves(nCurves, ptsPerCurve, radiusDeg, jitterDeg, 42)

	// 先校验：拼接应还原出完整闭合环，顶点数=总边数。
	wantVerts := nCurves * (ptsPerCurve - 1)
	if got := MergeCurves(curves, tolerance); len(got) != wantVerts {
		b.Fatalf("拼接未成完整环：得到 %d 顶点，期望 %d", len(got), wantVerts)
	}
	b.Logf("输入: %d 条曲线 × %d 点；近似重合衔接坐标 %d 个；输出环顶点 %d",
		nCurves, ptsPerCurve, 2*nCurves, wantVerts)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = MergeCurves(curves, tolerance)
	}
}

// BenchmarkMergeCurvesVs2 对比两种实现在「相邻曲线共享端点(带亚容差抖动)」这类
// 干净数据上的耗时：
//   - MergeCurves  : snap-rounding(顶点聚类 + 逐段 DDA noding + 边去重 + 环游走)
//   - MergeCurves2 : 端点链式拼接(只索引端点、双向延伸)
//
// 两者在这类数据上结果一致(都还原同一环)，但 MergeCurves2 不做逐点 noding，
// 预期明显更快。曲线数取 50/100/200/500 档，每条 1024 点。
func BenchmarkMergeCurvesVs2(b *testing.B) {
	const (
		ptsPerCurve = 1024
		tolerance   = 1.0
		radiusDeg   = 1.0
		jitterDeg   = 1.5e-6
	)
	for _, nCurves := range []int{50, 100, 200, 500} {
		curves := buildScrambledRingCurves(nCurves, ptsPerCurve, radiusDeg, jitterDeg, 42)

		// 一致性校验(计时外)：两者顶点数应相同。
		r1 := MergeCurves(curves, tolerance)
		r2 := MergeCurves2(curves, tolerance)
		r3 := MergeCurves3(curves, tolerance)
		b.Logf("[%d 条 ×%d 点] 环顶点: MergeCurves=%d MergeCurves2=%d MergeCurves3=%d",
			nCurves, ptsPerCurve, len(r1), len(r2), len(r3[0].Coords))

		b.Run("MergeCurves/"+itoa(nCurves), func(b *testing.B) {
			b.ReportAllocs()
			for range b.N {
				_ = MergeCurves(curves, tolerance)
			}
		})
		b.Run("MergeCurves2/"+itoa(nCurves), func(b *testing.B) {
			b.ReportAllocs()
			for range b.N {
				_ = MergeCurves2(curves, tolerance)
			}
		})
		b.Run("MergeCurves3/"+itoa(nCurves), func(b *testing.B) {
			b.ReportAllocs()
			for range b.N {
				_ = MergeCurves3(curves, tolerance)
			}
		})
	}
}
