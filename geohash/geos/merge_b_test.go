//go:build !windows && cgo

package geos_test

import (
	"math"
	"math/rand"
	"testing"

	"geos"

	"github.com/lxt1045/utils/geohash"
	"github.com/peterstace/simplefeatures/geom"
)

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

// curvesToWKT 把多条坐标曲线构造成 MULTILINESTRING 的 WKT(用 simplefeatures)。
func curvesToWKT(curves [][]geohash.Coords) string {
	lines := make([]geom.LineString, 0, len(curves))
	for _, cv := range curves {
		xys := make([]float64, 0, len(cv)*2)
		for _, c := range cv {
			xys = append(xys, c.Lng, c.Lat)
		}
		lines = append(lines, geom.NewLineString(geom.NewSequence(xys, geom.DimXY)))
	}
	return geom.NewMultiLineString(lines).AsText()
}

// wktLineStringToCoords 把 GEOSLineMerge 输出的单条 LineString 转回 geohash.Coords，
// 并去掉与首点重合的尾点(闭合环)，得到与 MergeCurves 同约定的顶点序列，便于求面积。
func wktLineStringToCoords(ls geom.LineString) []geohash.Coords {
	seq := ls.Coordinates()
	n := seq.Length()
	ring := make([]geohash.Coords, n)
	for i := 0; i < n; i++ {
		xy := seq.GetXY(i)
		ring[i] = geohash.Coords{Lat: xy.Y, Lng: xy.X}
	}
	if len(ring) > 1 && ring[0] == ring[len(ring)-1] {
		ring = ring[:len(ring)-1]
	}
	return ring
}

// jitterEndpoints 在既有曲线集的基础上派生出“模糊重合”版本：深拷贝每条曲线，
// 只对其两个端点(即与相邻段的衔接点)各注入一个 < tolerance 的独立抖动，
// 段内部点保持不变。这样模糊集与精确集除“衔接点是否精确相等”外完全一致
// (同样的段划分、同样的方向、同样的顺序)，对比才有说服力。
func jitterEndpoints(curves [][]geohash.Coords, jitterDeg float64, seed int64) [][]geohash.Coords {
	rng := rand.New(rand.NewSource(seed))
	out := make([][]geohash.Coords, len(curves))
	for i, cv := range curves {
		seg := make([]geohash.Coords, len(cv))
		copy(seg, cv)
		perturb := func(idx int) {
			seg[idx] = geohash.Coords{
				Lat: seg[idx].Lat + (rng.Float64()*2-1)*jitterDeg,
				Lng: seg[idx].Lng + (rng.Float64()*2-1)*jitterDeg,
			}
		}
		perturb(0)
		if len(seg) > 1 {
			perturb(len(seg) - 1)
		}
		out[i] = seg
	}
	return out
}

// denseJunctions 在既有曲线集基础上派生出“衔接处密集近似重合”版本：把每条曲线
// 的两个端点各扩成一簇 nJ 个点，簇内每个点都是该端点加一个 < tolerance 的独立
// 抖动(误差范围内)。段内部点保持不变。
//
// 目的：模拟真实数据里“同一个衔接点被多份图幅各自采样了上百次、彼此都在误差内
// 但不完全相等”的极端形态，专门压 MergeCurves 的 dedupConsecutive 去重路径——
// 每条曲线两端各 nJ 个近似重合点都要被逐一距离判定并折叠。
//
// 折叠后每条曲线恢复为 (原端点 1 + 段内 + 原端点 1)，故拼接结果的环顶点数与
// 精确集一致，可用同一个 wantVerts 校验。
func denseJunctions(curves [][]geohash.Coords, nJ int, jitterDeg float64, seed int64) [][]geohash.Coords {
	rng := rand.New(rand.NewSource(seed))
	jitter := func(c geohash.Coords) geohash.Coords {
		return geohash.Coords{
			Lat: c.Lat + (rng.Float64()*2-1)*jitterDeg,
			Lng: c.Lng + (rng.Float64()*2-1)*jitterDeg,
		}
	}
	out := make([][]geohash.Coords, len(curves))
	for i, cv := range curves {
		seg := make([]geohash.Coords, 0, len(cv)-2+2*nJ)
		for k := 0; k < nJ; k++ { // 起点衔接簇：nJ 个围绕 cv[0] 的近似重合点
			seg = append(seg, jitter(cv[0]))
		}
		if len(cv) > 2 { // 段内部点原样保留
			seg = append(seg, cv[1:len(cv)-1]...)
		}
		last := cv[len(cv)-1]
		for k := 0; k < nJ; k++ { // 终点衔接簇：nJ 个围绕 cv[last] 的近似重合点
			seg = append(seg, jitter(last))
		}
		out[i] = seg
	}
	return out
}

// BenchmarkMerge 在同一个基准函数里用 b.Run() 压测三个子基准：
//   - MergeCurves      : 本包实现，精确重合衔接(与 GEOS 同输入)
//   - MergeCurvesFuzzy : 本包实现，模糊重合衔接(端点带 < tolerance 抖动)
//   - LineMergeWKT     : GEOS 参照实现(GEOSLineMerge 的 cgo 封装)
//
// 数据规模：100 条曲线 × 每条 1024 个坐标。两套数据集：
//   - 精确重合(jitterDeg=0)：GEOSLineMerge 要求端点严格相等才会缝合，注入抖动它
//     就拼不成单条线；对 MergeCurves 是容差判定的特例，仍走完打乱/反转/去重全流程。
//     用它让 MergeCurves 与 GEOS 吃完全相同的输入，公平对比。
//   - 模糊重合(jitterDeg>0)：衔接点带 < tolerance 的抖动，是真实图幅数据的常见形态，
//     只有 MergeCurves 的容差去重能处理，体现其相对裸 GEOSLineMerge 的实用价值。
//
// 公共部分(计时外)完成：构造两套曲线、构造喂给 GEOSLineMerge 的 WKT、正确性校验。
// 尤其 WKT 构造是把纯 Go 几何交给 C 库前的数据处理，放在公共部分以免影响基准。
func BenchmarkMerge(b *testing.B) {
	const (
		nCurves     = 100
		ptsPerCurve = 1024
		tolerance   = 1.0
		radiusDeg   = 1.0
	)

	const jitterDeg = 1.5e-6 // ~0.17m (< tolerance)，衔接点抖动量

	// —— 公共数据准备(不计入任何子基准的计时) ——
	// 精确重合数据集：MergeCurves 与 GEOSLineMerge 共用(GEOS 要求端点严格相等)。
	curves := buildScrambledRingCurves(nCurves, ptsPerCurve, radiusDeg, 0 /*精确重合*/, 42)
	inWKT := curvesToWKT(curves) // LineMergeWKT 的输入，提前构造好
	// 模糊重合数据集：在精确数据集基础上，仅对每条曲线的两个端点(衔接点)注入
	// < tolerance 的抖动。除此之外与 curves 完全相同(同样的分段、顺序、方向、
	// 段内点)，两者唯一差别就是衔接处的亚容差偏移——对比才有说服力。
	curvesFuzzy := jitterEndpoints(curves, jitterDeg, 43)
	// 衔接处密集近似重合数据集：在精确集基础上，把每条曲线两端各扩成一簇 nDenseJ 个
	// 误差范围内的近似重合点(每条曲线两端合计 2*nDenseJ 个衔接点)。专压去重路径。
	const nDenseJ = 128 // 每端 100+ 个衔接点
	curvesDense := denseJunctions(curves, nDenseJ, jitterDeg, 44)

	// 校验各条路径都能正确把所有段拼接成完整闭合环，并各自求拼接后环的面积。
	wantVerts := nCurves * (ptsPerCurve - 1)
	ringExact := geohash.MergeCurves(curves, tolerance)
	if len(ringExact) != wantVerts {
		b.Fatalf("MergeCurves(精确) 拼接未成完整环：得到 %d 顶点，期望 %d", len(ringExact), wantVerts)
	}
	ringFuzzy := geohash.MergeCurves(curvesFuzzy, tolerance)
	if len(ringFuzzy) != wantVerts {
		b.Fatalf("MergeCurves(模糊) 拼接未成完整环：得到 %d 顶点，期望 %d", len(ringFuzzy), wantVerts)
	}
	// 密集衔接集：每端一簇近似重合点会被 dedupConsecutive 折叠回单点，环顶点数仍为 wantVerts。
	ringDense := geohash.MergeCurves(curvesDense, tolerance)
	if len(ringDense) != wantVerts {
		b.Fatalf("MergeCurves(密集衔接) 拼接未成完整环：得到 %d 顶点，期望 %d", len(ringDense), wantVerts)
	}
	outWKT, err := geos.LineMergeWKT(inWKT)
	if err != nil {
		b.Fatalf("LineMergeWKT 失败: %v", err)
	}
	g, err := geom.UnmarshalWKT(outWKT)
	if err != nil {
		b.Fatalf("解析 GEOS 输出失败: %v", err)
	}
	if g.Type() != geom.TypeLineString {
		b.Fatalf("GEOSLineMerge 未缝合成单条线，输出类型=%s", g.Type())
	}
	ringGEOS := wktLineStringToCoords(g.MustAsLineString())

	// —— 各数据集拼接后环的面积(单位 m²)。GEOS 精确重合结果作为面积基准。——
	areaExact := geohash.AreaCoords(ringExact)
	areaFuzzy := geohash.AreaCoords(ringFuzzy)
	areaDense := geohash.AreaCoords(ringDense)
	areaGEOS := geohash.AreaCoords(ringGEOS)

	relErr := func(a, ref float64) float64 { return math.Abs(a-ref) / ref }
	b.Logf("共用输入: %d 条曲线 × %d 点；输出环顶点 %d；精确重合 WKT 长度=%d 字节",
		nCurves, ptsPerCurve, wantVerts, len(inWKT))
	b.Logf("拼接后环面积(m^2): GEOS(基准)=%.3f  精确=%.3f  模糊=%.3f  密集=%.3f",
		areaGEOS, areaExact, areaFuzzy, areaDense)
	b.Logf("相对 GEOS 基准的面积误差: 精确=%.3e  模糊=%.3e  密集=%.3e",
		relErr(areaExact, areaGEOS), relErr(areaFuzzy, areaGEOS), relErr(areaDense, areaGEOS))

	/*
	   ┌──────────────────┬─────────────────────┬──────────┬──────────┐
	   │      子基准      │      衔接形态       │   耗时   │ 内存/op  │
	   ├──────────────────┼─────────────────────┼──────────┼──────────┤
	   │ MergeCurves      │ 精确重合(每端1点)   │ ~5.75 ms │ 10.56 MB │
	   ├──────────────────┼─────────────────────┼──────────┼──────────┤
	   │ MergeCurvesFuzzy │ 亚容差抖动(每端1点) │ ~5.76 ms │ 10.56 MB │
	   ├──────────────────┼─────────────────────┼──────────┼──────────┤
	   │ MergeCurvesDense │ 每端128个近似重合点 │ ~6.91 ms │ 10.97 MB │
	   ├──────────────────┼─────────────────────┼──────────┼──────────┤
	   │ LineMergeWKT     │ GEOS 参照           │ ~38.1 ms │ 3.46 MB  │
	   └──────────────────┴─────────────────────┴──────────┴──────────┘
	 //*/

	// —— 子基准 1：本包 MergeCurves(纯 Go，精确重合衔接) ——
	b.Run("MergeCurves", func(b *testing.B) {
		b.ReportAllocs()
		for range b.N {
			_ = geohash.MergeCurves(curves, tolerance)
		}
	})

	// —— 子基准 2：本包 MergeCurves(纯 Go，模糊重合衔接) ——
	// 衔接端点带 < tolerance 的抖动，走容差去重路径(GEOSLineMerge 无法处理这种输入)。
	b.Run("MergeCurvesFuzzy", func(b *testing.B) {
		b.ReportAllocs()
		for range b.N {
			_ = geohash.MergeCurves(curvesFuzzy, tolerance)
		}
	})

	// —— 子基准 3：本包 MergeCurves(纯 Go，密集衔接簇) ——
	// 每条曲线两端各有 100+ 个近似重合的衔接点(全部在 tolerance 内)，
	// 重点压测 dedupConsecutive 的容差折叠与衔接去重路径。
	b.Run("MergeCurvesDense", func(b *testing.B) {
		b.ReportAllocs()
		for range b.N {
			_ = geohash.MergeCurves(curvesDense, tolerance)
		}
	})

	// —— 子基准 4：GEOS 参照 LineMergeWKT(cgo；计时含 WKT 解析+合并+序列化) ——
	b.Run("LineMergeWKT", func(b *testing.B) {
		b.ReportAllocs()
		for range b.N {
			if _, err := geos.LineMergeWKT(inWKT); err != nil {
				b.Fatal(err)
			}
		}
	})
}
