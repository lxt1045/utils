package geohash_test

import (
	"fmt"
	"math"
	"math/rand"
	"testing"

	"github.com/lxt1045/utils/geohash"
)

// addOverlapPath 在既有曲线集基础上派生「中段重叠」数据：把边界上某一条曲线的
// 前 nOverlap 个顶点**精确复制**成一条新曲线追加进去——于是这段边界被「数字化了
// 两次」，产生恰好 nOverlap 个重合点(与曲线总数无关，便于横向对比数据量)。
//
// 取自单条曲线的连续前缀而非跨曲线摊平：buildScrambledRingCurves 会打乱曲线顺序，
// 跨曲线取点会拼出空间上不连续的路径、破坏单环拓扑；而单条曲线本身是边界上一段
// 连续子路径，其前缀正是一段干净的「中段」。每条曲线 1024 点，取 200 恒够。
//
// 用精确复制(顶点完全相同)而非任意重采样：任意重采样的正确性已由 geohash 包的
// TestMergeCurves_MidSegmentOverlap 覆盖；而 GEOS 的 UnaryUnion 只能消除精确
// 重合的重复边，用精确复制才能让 GEOS 公平地参与对比。反向复制，触发方向不定的重叠。
func addOverlapPath(curves [][]geohash.Coords, nOverlap int) [][]geohash.Coords {
	src := curves[0]
	if nOverlap > len(src) {
		nOverlap = len(src)
	}
	dup := make([]geohash.Coords, nOverlap)
	// 反向复制(与源曲线方向相反)，制造方向不定的重叠。
	for i := range dup {
		dup[i] = src[nOverlap-1-i]
	}
	out := make([][]geohash.Coords, len(curves), len(curves)+1)
	copy(out, curves)
	return append(out, dup)
}

// reverseCoordsB 返回逆序副本(基准测试自用；geohash.reverseCoords 未导出)。
func reverseCoordsB(c []geohash.Coords) []geohash.Coords {
	out := make([]geohash.Coords, len(c))
	for i := range c {
		out[i] = c[len(c)-1-i]
	}
	return out
}

// buildScrambledRingCurves 构造压测数据集，与 geohash 包中同名基准的构造一致：
// 取一个大圆环边界，切成 nCurves 段、每段 ptsPerCurve 个坐标；相邻段在衔接处
// 本应共享顶点，这里让两侧各注入一个 jitterDeg 的独立抖动，制造“近似重合但不
// 完全相等”的衔接点(闭合环共 2*nCurves 个)。最后随机反转段方向并打乱段顺序。
//
// jitterDeg=0 时衔接点“精确重合”，用于喂给 GEOSLineMerge(它要求端点严格相等
// 才会缝合)；jitterDeg>0 时用于压测 MergeCurves 的容差去重路径。
//
// 尺度约束：段内相邻采样点间距必须 > tolerance(否则被 dedupConsecutive 折叠)，
// 衔接抖动必须 < tolerance(否则断链)。radiusDeg=1° 时段内间距约 6.8m。
func buildScrambledRingCurves(nCurves, ptsPerCurve int, radiusDeg, jitterDeg float64, seed int64) [][]geohash.Coords {
	rng := rand.New(rand.NewSource(seed))
	const cx, cy = 116.4, 39.9 // 环心(经度, 纬度)

	edgesPerCurve := ptsPerCurve - 1
	totalVerts := nCurves * edgesPerCurve
	angleStep := 2 * math.Pi / float64(totalVerts)

	vertexAt := func(k int) geohash.Coords {
		ang := float64(k) * angleStep
		return geohash.Coords{
			Lat: cy + radiusDeg*math.Sin(ang),
			Lng: cx + radiusDeg*math.Cos(ang),
		}
	}
	jitter := func(c geohash.Coords) geohash.Coords {
		if jitterDeg == 0 {
			return c
		}
		return geohash.Coords{
			Lat: c.Lat + (rng.Float64()*2-1)*jitterDeg,
			Lng: c.Lng + (rng.Float64()*2-1)*jitterDeg,
		}
	}

	curves := make([][]geohash.Coords, nCurves)
	for s := range nCurves {
		startV := s * edgesPerCurve
		seg := make([]geohash.Coords, ptsPerCurve)
		for k := range ptsPerCurve {
			v := vertexAt((startV + k) % totalVerts)
			if k == 0 || k == ptsPerCurve-1 {
				v = jitter(v) // 仅两端注入抖动，制造与相邻段的“近似重合”衔接
			}
			seg[k] = v
		}
		curves[s] = seg
	}

	for s := range curves {
		if rng.Intn(2) == 0 {
			curves[s] = reverseCoordsB(curves[s])
		}
	}
	for i := len(curves) - 1; i > 0; i-- {
		j := rng.Intn(i + 1)
		curves[i], curves[j] = curves[j], curves[i]
	}
	return curves
}

// BenchmarkMergeOverlap 压测「中段重叠(边界段被数字化两次)」场景下两种实现随
// 数据量(曲线数)增长的耗时对比，并在公共部分对比各自还原环的面积误差。
//
// 数据：每条曲线固定 1024 点，用 addOverlapPath 追加一条精确复制的 200 点子路径
// 形成中段重叠(重合点数固定 200，与曲线数无关，便于横向比较)。曲线数取
// 50/100/200/500 四档，即总点数约 5万/10万/20万/51万。两种实现都应把重叠段塌成
// 一条、还原出同一条环，面积与「无重叠原始边界」一致。
//
// 三方：
//   - MergeCurves               : 本包实现(geohash 网格 snap-rounding noding，近 O(V))
//   - MergeCurves2              : 本包快速实现(仅适用密集点+粘接点对齐；本数据集超出其
//     适用范围，一并测以观察其在不适配场景下的表现)
//   - GEOS UnaryUnion+LineMerge : C 库 noding 参照(计时含 WKT 往返)
//
// 每档公共部分(计时外)：构造数据、跑一遍各实现求面积、算相对误差并打印。
func BenchmarkMergeOverlap(b *testing.B) {
	const (
		ptsPerCurve = 1024 * 8
		nOverlap    = 200 // 固定 200 个重合点
		tolerance   = 1.0
		radiusDeg   = 0.01 // 半径 ~1.11km、周长 ~7km；相邻点间距随曲线数在 ~0.14m(50 条)到 ~0.014m(500 条)间，全档 < tolerance/2 = 0.5m，满足 MergeCurves2 密集点前提
	)

	for _, nCurves := range []int{50, 100, 200, 500} {
		// —— 公共数据准备(不计入计时) ——
		base := buildScrambledRingCurves(nCurves, ptsPerCurve, radiusDeg, 0 /*精确重合*/, 42)
		overlap := addOverlapPath(base, nOverlap)

		// 无重叠真值面积：用本包对 base 还原环再求面积。
		baseRing := geohash.MergeCurves(base, tolerance)
		wantArea := geohash.AreaCoords(baseRing)

		// 两种实现在重叠数据上的还原环 + 面积。
		ringMine := geohash.MergeCurves(overlap, tolerance)

		areaMine := geohash.AreaCoords(ringMine)

		// MergeCurves2 前提是「密集点曲线 + 粘接点对齐」。本数据集已调密集(相邻点
		// ~0.14~0.014m < tolerance/2)，满足密集前提；但仍含中段重叠(addOverlapPath 追加的
		// 精确复制曲线)，多出的重叠曲线无法纳入单环游走——预期仍还原不出正确环。
		// 一并计入对比，如实反映其在「密集但含重叠」场景下的边界行为。
		ringMine2 := geohash.MergeCurves2(overlap, tolerance)
		areaMine2 := geohash.AreaCoords(ringMine2)

		// MergeCurves3 分区遍历：返回所有链(闭合环 + 开放链)。重复数字化的重叠曲线会
		// 自成一条独立链，不污染主环。只对闭合环(IsClosed=true)求面积，取最大的一条作为
		// 真实边界环(重叠曲线形成的退化环面积远小于主环)。
		rings3 := geohash.MergeCurves3(overlap, tolerance)
		areaMine3, closedCnt, mainVerts := 0.0, 0, 0
		for _, r := range rings3 {
			if !r.IsClosed {
				continue
			}
			closedCnt++
			if a := geohash.AreaCoords(r.Coords); a > areaMine3 {
				areaMine3, mainVerts = a, len(r.Coords)
			}
		}

		// MergeCurves4 与 MergeCurves3 算法相同，仅在环游走阶段用复用 scratch 缓冲 +
		// 精确大小拷贝替代逐环 append 增长，减少内存分配。结果应与 MergeCurves3 一致。
		rings4 := geohash.MergeCurves4(overlap, tolerance)
		areaMine4 := 0.0
		for _, r := range rings4 {
			if !r.IsClosed {
				continue
			}
			if a := geohash.AreaCoords(r.Coords); a > areaMine4 {
				areaMine4 = a
			}
		}

		relErr := func(a float64) float64 { return math.Abs(a-wantArea) / wantArea }
		b.Logf("[%d 条 ×%d 点 +%d 重合] 环顶点: 真值=%d MergeCurves=%d MergeCurves2=%d MergeCurves3(主环)=%d(闭合环%d/共%d链); 面积误差: MergeCurves=%.3e MergeCurves2=%.3e MergeCurves3=%.3e MergeCurves4=%.3e",
			nCurves, ptsPerCurve, nOverlap, len(baseRing), len(ringMine), len(ringMine2), mainVerts, closedCnt, len(rings3), relErr(areaMine), relErr(areaMine2), relErr(areaMine3), relErr(areaMine4))

		b.Run(fmt.Sprintf("MergeCurves/%d", nCurves), func(b *testing.B) {
			b.ReportAllocs()
			for range b.N {
				_ = geohash.MergeCurves(overlap, tolerance)
			}
		})
		b.Run(fmt.Sprintf("MergeCurves2/%d", nCurves), func(b *testing.B) {
			b.ReportAllocs()
			for range b.N {
				_ = geohash.MergeCurves2(overlap, tolerance)
			}
		})
		b.Run(fmt.Sprintf("MergeCurves3/%d", nCurves), func(b *testing.B) {
			b.ReportAllocs()
			for range b.N {
				_ = geohash.MergeCurves3(overlap, tolerance)
			}
		})
		b.Run(fmt.Sprintf("MergeCurves4/%d", nCurves), func(b *testing.B) {
			b.ReportAllocs()
			for range b.N {
				_ = geohash.MergeCurves4(overlap, tolerance)
			}
		})
	}
}

/*
go test -benchmem -run=^$ -bench ^BenchmarkMergeOverlapPprof$ github.com/lxt1045/utils/geohash -count=1 -v -cpuprofile cpu.prof
go test -benchmem -run=^$ -bench ^BenchmarkMergeOverlapPprof$ github.com/lxt1045/utils/geohash -c -o test.bin
go tool pprof ./test.bin cpu.prof
*/
func BenchmarkMergeOverlapPprof(b *testing.B) {
	const (
		ptsPerCurve = 1024 * 8
		nOverlap    = 200 // 固定 200 个重合点
		tolerance   = 1.0
		radiusDeg   = 0.01 // 半径 ~1.11km、周长 ~7km；相邻点间距随曲线数在 ~0.14m(50 条)到 ~0.014m(500 条)间，全档 < tolerance/2 = 0.5m，满足 MergeCurves2 密集点前提
	)

	for _, nCurves := range []int{50, 100, 200, 500} {
		// —— 公共数据准备(不计入计时) ——
		base := buildScrambledRingCurves(nCurves, ptsPerCurve, radiusDeg, 0 /*精确重合*/, 42)
		overlap := addOverlapPath(base, nOverlap)

		b.Run(fmt.Sprintf("MergeCurves3/%d", nCurves), func(b *testing.B) {
			b.ReportAllocs()
			for range b.N {
				_ = geohash.MergeCurves3(overlap, tolerance)
			}
		})
	}
}
