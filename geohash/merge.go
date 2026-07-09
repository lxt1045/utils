package geohash

import (
	"math"
	"sort"
)

// 本文件实现"将多条无序曲线拼接成一条闭合环(多边形边界)"的功能。
//
// 场景：一个区域的边界被拆成了若干条曲线(polyline)分别存储，
// 这些曲线之间没有先后顺序，方向(正向/反向)也不确定。相邻曲线在衔接处
// 往往共享同一个端点(因精度问题略有偏差)；更棘手的是——同一段边界可能被
// 「数字化了两次」：两条曲线在中段近似重合(横向 < tolerance)，但顶点是
// 任意重采样(顶点数/位置都不同)。需要把它们拼回一条闭合环，并消除重叠。
//
// 算法：容差 snap-rounding + 边去重 + 环游走(等价 JTS SnapRoundingNoder +
// LineDissolver + ring walk)：
//  1. 顶点聚类：把彼此距离 <= tolerance 的顶点并成同一个「节点(node)」。
//     这样"关节抖动"和"重复端点"都归并到同一节点。
//  2. 逐曲线 noding：把每条线段在「附近节点」处打断(按投影参数 t 排序)，
//     得到该曲线的节点序列。任意重采样的两份数字化会经过相同的共享节点，
//     从而产生相同的节点边序列。
//  3. 边去重：相邻节点间的无向边放进集合自动去重——重复数字化的重叠段
//     产生相同的边，塌成一条(消除重叠)。
//  4. 环游走：沿邻接边走一圈得到单环顶点(方向不保证)。
//
// distMeters 返回两坐标点的距离(米)。
// 用 Haversine(球面大圆)，全球范围稳定，避免等距圆柱近似在高纬度的偏差。
func distMeters(a, b Coords) float64 {
	return DistHaversine(a, b)
}

// reverseCoords 返回逆序副本，不修改入参。
func reverseCoords(c []Coords) []Coords {
	out := make([]Coords, len(c))
	for i := range c {
		out[i] = c[len(c)-1-i]
	}
	return out
}

// precisionCellSpanM[h] 是「纬度 70° 处、每轴 h bit 的 geohash 网格」单个 cell 的
// 较短边长(米)，h 从 0 到 31 预先算好，供 tolerancePrecisionBits 查表。
//
// 为什么固定 70°：纬度越高 cos 越小、经向 cell 越窄，用一个足够高的参考纬度取
// 最保守(最窄)的 cell 边长即可，省去逐点求实际最大纬度。70° 覆盖了绝大多数陆地
// 数据；即便数据在更高纬，网格只是偏细一点，不影响 9 邻域搜索的完备性(只会更保守)。
var precisionCellSpanM = func() (t [32]float64) {
	const mPerDeg = 111320.0  // 每纬度约 111.32km
	const cos70 = 0.342020143 // cos(70°)
	// 较短轴(经向，cos70*360 < 180)在整幅网格上的跨度(米)。
	full := mPerDeg * math.Min(180.0, cos70*360.0)
	for h := range t {
		t[h] = full / float64(uint64(1)<<uint(h))
	}
	return
}()

// tolerancePrecisionBits 选一个 geohash 精度 bits(每轴 h bit，总 bits=2h)，使网格
// cell 的较短边 >= tolerance——从而任意两个距离 <= tolerance 的点必落在同一 cell 或
// 其 8 邻居内，9 格搜索即完备(不漏点)。
//
// 直接查预先算好的 precisionCellSpanM 表：取满足 span[h] >= tolerance 的最大 h
// (h 越大 cell 越小)，clamp 到 [1,31]。
func tolerancePrecisionBits(tolerance float64) uint {
	if tolerance <= 0 {
		return 62 // 退化输入：最高精度
	}
	h := 1
	for h < 31 && precisionCellSpanM[h+1] >= tolerance {
		h++
	}
	return uint(2 * h)
}

// projPointSeg 在局部等距圆柱近似下，求点 p 到线段 [a,b] 的最近距离(米)与
// 投影参数 t(夹在 [0,1]，用于沿线段排序)。tolerance 量级(米)下该近似足够精确。
func projPointSeg(a, b, p Coords) (t, dist float64) {
	const mPerDeg = 111320.0
	kx := mPerDeg * math.Cos(a.Lat*degToRad) // 每度经度的米数(按 a 的纬度)
	bx := (b.Lng - a.Lng) * kx
	by := (b.Lat - a.Lat) * mPerDeg
	px := (p.Lng - a.Lng) * kx
	py := (p.Lat - a.Lat) * mPerDeg
	len2 := bx*bx + by*by
	if len2 == 0 {
		return 0, math.Hypot(px, py)
	}
	t = (px*bx + py*by) / len2
	if t < 0 {
		t = 0
	} else if t > 1 {
		t = 1
	}
	dx := px - t*bx
	dy := py - t*by
	return t, math.Hypot(dx, dy)
}

// Coords  坐标
type CurvePoint struct {
	Coords
	CurveIdx int
	PointIdx int
}

// snapGrid 保存 snap-rounding 用的节点集合与网格索引。
type CurvePointIdx struct {
	shift     uint                    // 32-h：由 32bit 全精度坐标右移得到 cell 下标
	cellIndex map[uint64][]CurvePoint // cell key -> 落在该 cell 的节点 id 列表
}

// snapGrid 保存 snap-rounding 用的节点集合与网格索引。
type snapGrid struct {
	nodes     []Coords         // 每个节点的代表坐标(该簇首个顶点)
	shift     uint             // 32-h：由 32bit 全精度坐标右移得到 cell 下标
	cellIndex map[uint64][]int // cell key -> 落在该 cell 的节点 id 列表

	// indexed noding 用的复用暂存：seenGen[id]==curGen 表示该节点已在当前
	// 线段的近邻扫描里测过，避免重复 projPointSeg。用「代次戳」而非 map，
	// 每条线段只需 curGen++ 即完成 O(1) 重置，跨线段复用同一块内存(零分配)。
	seenGen []int32
	curGen  int32
}

// cellXYKey 把 (cx,cy) cell 下标打包成 uint64 键。
func cellXYKey(cx, cy uint32) uint64 { return uint64(cx)<<32 | uint64(cy) }

// cellXY 返回坐标所在的 cell 下标 (cx,cy)。
func (g *snapGrid) cellXY(p Coords) (cx, cy uint32) {
	x, y := EncodeCoords(p.Lat, p.Lng)
	return x >> g.shift, y >> g.shift
}

// cellXY 返回坐标所在的 cell 下标 (cx,cy)。
func (g *CurvePointIdx) cellXY(p Coords) (cx, cy uint32) {
	x, y := EncodeCoords(p.Lat, p.Lng)
	return x >> g.shift, y >> g.shift
}

// findNode 在 p 所在 cell 及其 8 邻居中，找一个距离 <= tolerance 的已有节点。
func (g *CurvePointIdx) findNode(p Coords, tolerance float64) (point CurvePoint, ok bool) {
	cx, cy := g.cellXY(p)
	for dx := -1; dx <= 1; dx++ {
		for dy := -1; dy <= 1; dy++ {
			ncx, ncy := int(cx)+dx, int(cy)+dy
			if ncx < 0 || ncy < 0 {
				continue
			}
			for _, point = range g.cellIndex[cellXYKey(uint32(ncx), uint32(ncy))] {
				if distMeters(p, point.Coords) <= tolerance {
					ok = true
					return
				}
			}
		}
	}
	return
}

// findNode 在 p 所在 cell 及其 8 邻居中，找一个距离 <= tolerance 的已有节点。
func (g *snapGrid) findNode(p Coords, tolerance float64) (id int, ok bool) {
	cx, cy := g.cellXY(p)
	for dx := -1; dx <= 1; dx++ {
		for dy := -1; dy <= 1; dy++ {
			ncx, ncy := int(cx)+dx, int(cy)+dy
			if ncx < 0 || ncy < 0 {
				continue
			}
			for _, id := range g.cellIndex[cellXYKey(uint32(ncx), uint32(ncy))] {
				if distMeters(p, g.nodes[id]) <= tolerance {
					return id, true
				}
			}
		}
	}
	return 0, false
}

// buildSnapGrid 把所有(已内部去重的)曲线顶点聚类成节点：距离 <= tolerance/2 的
// 顶点归并为同一节点。返回节点集合与网格索引。baseline / indexed 共用同一套
// 聚类，保证两者节点一致，结果可逐点比对。
func buildCurvePointIdx(curves [][]Coords, tolerance float64, bits uint) *CurvePointIdx {
	g := &CurvePointIdx{
		shift:     32 - bits/2,
		cellIndex: make(map[uint64][]CurvePoint),
	}
	tolerance = tolerance / 2
	for id, c := range curves {
		lastCoords := c[0]
		for i, p := range c {
			if i == 0 || distMeters(lastCoords, p) < tolerance {
				continue
			}
			// 存入上一个点， 如果当前曲线只有一个距离大于 tolerance / 2 的点，则不存
			cx, cy := g.cellXY(lastCoords)
			key := cellXYKey(cx, cy)
			g.cellIndex[key] = append(g.cellIndex[key], CurvePoint{Coords: p, CurveIdx: id, PointIdx: i - 1})
			if i == len(c)-1 {
				cx, cy := g.cellXY(p)
				key := cellXYKey(cx, cy)
				g.cellIndex[key] = append(g.cellIndex[key], CurvePoint{Coords: p, CurveIdx: id, PointIdx: i})
			}
			lastCoords = p
		}
	}
	return g
}

// buildSnapGrid 把所有(已内部去重的)曲线顶点聚类成节点：距离 <= tolerance/2 的
// 顶点归并为同一节点(保证曲线点间的距离<tolerance误差不放大,也保证索引中的点减少)。返回节点集合与网格索引。baseline / indexed 共用同一套
// 聚类，保证两者节点一致，结果可逐点比对。
func buildSnapGrid(cleaned [][]Coords, tolerance float64, bits uint) *snapGrid {
	g := &snapGrid{
		shift:     32 - bits/2,
		cellIndex: make(map[uint64][]int),
	}
	for _, c := range cleaned {
		for _, p := range c {
			if _, ok := g.findNode(p, tolerance); ok {
				continue // 已属于某个既有节点
			}
			id := len(g.nodes)
			g.nodes = append(g.nodes, p)
			cx, cy := g.cellXY(p)
			key := cellXYKey(cx, cy)
			g.cellIndex[key] = append(g.cellIndex[key], id)
		}
	}
	g.seenGen = make([]int32, len(g.nodes))
	return g
}

// nodeOnSeg 是排序用的中间结构：线段上的一个节点及其投影参数 t。
type nodeOnSeg struct {
	id int
	t  float64
}

// sortSegNodes 把候选节点按投影 t 排序并抽取 id。
func sortSegNodes(cand []nodeOnSeg) []int {
	sort.Slice(cand, func(i, j int) bool { return cand[i].t < cand[j].t })
	out := make([]int, len(cand))
	for i := range cand {
		out[i] = cand[i].id
	}
	return out
}

// segNodesIndexed 沿线段 [a,b] 逐个整数 cell 步进，只在扫过的 cell 及其 8 邻居里
// 取候选节点，再用 dist <= tolerance 过滤、按 t 排序。cell 边长 >= tolerance，故
// 任意与线段距离 <= tolerance 的节点必落在扫过 cell 的 3x3 邻域内(不漏点)；
// 复杂度约 O(线段跨越的 cell 数) => 整体近 O(顶点数)。
//
// 实现要点(相对朴素 DDA 的两处零分配优化)：
//   - 整数 cell 步进(沿主轴逐格)，不做浮点过采样，步数 = 主轴 cell 跨度 + 1；
//   - 用 g.seenGen 代次戳去重节点，避免 per-segment 分配 map；每条线段 curGen++
//     即完成 O(1) 重置。同一 cell 被邻域重复覆盖时，节点靠 seenGen 跳过，不重复投影。
func (g *snapGrid) segNodesIndexed(a, b Coords, tolerance float64) []int {
	xa, ya := EncodeCoords(a.Lat, a.Lng)
	xb, yb := EncodeCoords(b.Lat, b.Lng)
	cxa, cya := int(xa>>g.shift), int(ya>>g.shift)
	cxb, cyb := int(xb>>g.shift), int(yb>>g.shift)

	g.curGen++
	gen := g.curGen
	cand := make([]nodeOnSeg, 0, 8)

	addCell := func(cx, cy int) {
		for dx := -1; dx <= 1; dx++ {
			for dy := -1; dy <= 1; dy++ {
				ncx, ncy := cx+dx, cy+dy
				if ncx < 0 || ncy < 0 {
					continue
				}
				for _, id := range g.cellIndex[cellXYKey(uint32(ncx), uint32(ncy))] {
					if g.seenGen[id] == gen {
						continue // 本线段已测过该节点
					}
					g.seenGen[id] = gen
					t, d := projPointSeg(a, b, g.nodes[id])
					if d <= tolerance {
						cand = append(cand, nodeOnSeg{id, t})
					}
				}
			}
		}
	}

	// 沿主轴(cell 跨度更大的那条轴)整数步进，minor 轴按比例取整。
	dcx, dcy := cxb-cxa, cyb-cya
	nx, ny := dcx, dcy
	if nx < 0 {
		nx = -nx
	}
	if ny < 0 {
		ny = -ny
	}
	steps := nx
	if ny > steps {
		steps = ny
	}
	if steps == 0 {
		addCell(cxa, cya) // 同一 cell
		return sortSegNodes(cand)
	}
	for s := 0; s <= steps; s++ {
		f := float64(s) / float64(steps)
		cx := cxa + int(math.Round(float64(dcx)*f))
		cy := cya + int(math.Round(float64(dcy)*f))
		addCell(cx, cy)
	}
	return sortSegNodes(cand)
}

// curveNodeSeq 把一条曲线映射成节点序列：逐线段取附近节点(indexed noding)按 t
// 排序,顺次拼接并折叠相邻相同节点。
func curveNodeSeq(curve []Coords, g *snapGrid, tolerance float64) []int {
	if len(curve) == 1 {
		return nil // 单点曲线:退化处理
	}
	seq := make([]int, 0, len(curve))
	for i := 0; i+1 < len(curve); i++ {
		ids := g.segNodesIndexed(curve[i], curve[i+1], tolerance)
		for _, id := range ids {
			if len(seq) == 0 || seq[len(seq)-1] != id {
				seq = append(seq, id)
			}
		}
	}
	return seq
}

// ringFromNodeSeqs 由各曲线的节点序列构建无向边集(自动去重叠)，再游走成单环。
// 返回环上节点的代表坐标(不含重复首尾)。
func ringFromNodeSeqs(seqs [][]int, nodes []Coords, tolerance float64) []Coords {
	type edge struct{ a, b int }
	edgeSet := make(map[edge]struct{})
	adj := make(map[int][]int)
	addEdge := func(u, v int) {
		if u == v {
			return
		}
		if u > v {
			u, v = v, u
		}
		e := edge{u, v}
		if _, ok := edgeSet[e]; ok {
			return // 重复边(重叠段)只保留一条
		}
		edgeSet[e] = struct{}{}
		adj[u] = append(adj[u], v)
		adj[v] = append(adj[v], u)
	}
	for _, seq := range seqs {
		for i := 0; i+1 < len(seq); i++ {
			addEdge(seq[i], seq[i+1])
		}
	}
	if len(edgeSet) == 0 {
		// 没有任何边：可能是单点/单节点输入。
		if len(nodes) > 0 {
			return []Coords{nodes[0]}
		}
		return nil
	}

	// 选起点：优先度为 1 的节点(开放链)，否则任取一条边的端点。
	start := -1
	for n, nb := range adj {
		if len(nb) == 1 {
			if start == -1 || n < start {
				start = n
			}
		}
	}
	if start == -1 {
		for e := range edgeSet {
			start = e.a
			break
		}
	}

	usedEdge := make(map[edge]bool)
	takeEdge := func(u, v int) bool {
		if u > v {
			u, v = v, u
		}
		e := edge{u, v}
		if usedEdge[e] {
			return false
		}
		usedEdge[e] = true
		return true
	}

	ring := make([]Coords, 0, len(nodes))
	cur := start
	ring = append(ring, nodes[cur])
	for {
		next := -1
		for _, nb := range adj[cur] {
			if takeEdge(cur, nb) {
				next = nb
				break
			}
		}
		if next == -1 {
			break // 无未用边可走
		}
		cur = next
		if cur == start {
			break // 回到起点，闭环完成(不重复追加首点)
		}
		ring = append(ring, nodes[cur])
	}

	// 闭合去重约定：首尾在 tolerance 内则丢弃重复尾点。
	if len(ring) > 1 && distMeters(ring[0], ring[len(ring)-1]) <= tolerance {
		ring = ring[:len(ring)-1]
	}
	return ring
}

// MergeCurves 将多条无序曲线拼接成一条闭合环，返回环上的有序顶点(方向不保证，
// 不含重复首尾点)。结果可直接传入 AreaCoords 求面积。
//
// curves    : 每条曲线是一串有序坐标点(WGS84，度)；曲线间无先后顺序、方向不定。
// tolerance : 可接受误差(米)。用于顶点聚类成节点、线段 noding 两处判定。
//
// 本函数用容差 snap-rounding(见文件头算法说明)，能正确处理三种情形：
// 关节抖动、端点近似重合、以及「同一段边界被任意重采样地数字化两次」的中段重叠
// (重叠段会被边去重消除，不会重复计入面积)。noding 的近邻查找用 geohash 网格索引，
// 复杂度约 O(顶点数)。
//
// 注意：若输入曲线不能连成单一环(断裂或多个独立环)，只返回从起点能连通的那条链。
func MergeCurves(curves [][]Coords, tolerance float64) []Coords {
	if len(curves) == 0 {
		return nil
	}

	bits := tolerancePrecisionBits(tolerance)
	g := buildSnapGrid(curves, tolerance, bits)
	if len(g.nodes) == 0 {
		return nil
	}

	seqs := make([][]int, 0, len(curves))
	for _, c := range curves {
		if seq := curveNodeSeq(c, g, tolerance); len(seq) > 0 {
			seqs = append(seqs, seq)
		}
	}
	return ringFromNodeSeqs(seqs, g.nodes, tolerance)
}

func MergeCurves2(curves [][]Coords, tolerance float64) (closeCrv []Coords) {
	if len(curves) == 0 {
		return nil
	}

	bits := tolerancePrecisionBits(tolerance) // 通过查表获取geohash索引的精度
	g := buildCurvePointIdx(curves, tolerance, bits)
	if len(g.cellIndex) <= 2 {
		return nil
	}

	curvesScanned := make([]bool, len(curves)) // 标志已扫描过的曲线, 避免循环重复扫描造成无法退出
	rootPointIdx := 0
	currentCurbePoint, ok := g.findNode(curves[0][rootPointIdx], tolerance) // 从头开始找
	if !ok {
		c := curves[0]
		rootPointIdx = len(c) - 1
		currentCurbePoint, ok = g.findNode(c[rootPointIdx], tolerance) // 从头开始找
		if !ok {
			for i, p := range c[1 : len(c)-1] {
				currentCurbePoint, ok = g.findNode(p, tolerance)
				if ok {
					rootPointIdx = i + 1
					break
				}
			}
			if !ok {
				return nil
			}
		}
	}

OUT:
	for {
		// 回头了，圆满一圈
		if currentCurbePoint.CurveIdx == 0 {
			if rootPointIdx < currentCurbePoint.PointIdx {
				for i := currentCurbePoint.PointIdx - 1; i >= rootPointIdx; i-- {
					closeCrv = append(closeCrv, curves[0][i])
				}
			} else {
				closeCrv = append(closeCrv, curves[0][currentCurbePoint.PointIdx+1:rootPointIdx+1]...)
			}
			break
		}
		curvesScanned[currentCurbePoint.CurveIdx] = true
		currentCurve := curves[currentCurbePoint.CurveIdx]

		head := currentCurve[0]
		if distMeters(head, currentCurbePoint.Coords) > tolerance {
			nextCurbePoint, ok := g.findNode(head, tolerance)
			if ok && !curvesScanned[nextCurbePoint.CurveIdx] {
				for i := currentCurbePoint.PointIdx - 1; i >= 0; i-- {
					closeCrv = append(closeCrv, currentCurve[i])
				}
				currentCurbePoint = nextCurbePoint
				continue
			}
		}
		tail := currentCurve[len(currentCurve)-1]
		if distMeters(tail, currentCurbePoint.Coords) > tolerance {
			nextCurbePoint, ok := g.findNode(tail, tolerance)
			if ok && !curvesScanned[nextCurbePoint.CurveIdx] {
				closeCrv = append(closeCrv, currentCurve[currentCurbePoint.PointIdx+1:]...)
				continue
			}
			currentCurbePoint = nextCurbePoint
		}

		// 兜底: 分两头，从头还是从尾开始？
		for j, p := range currentCurve[1 : len(currentCurve)-1] {
			if distMeters(p, currentCurbePoint.Coords) > tolerance {
				nextCurbePoint, ok := g.findNode(p, tolerance)
				if ok && !curvesScanned[nextCurbePoint.CurveIdx] {
					j++
					if j < currentCurbePoint.PointIdx {
						for i := currentCurbePoint.PointIdx - 1; i >= j; i-- {
							closeCrv = append(closeCrv, currentCurve[i])
						}
					} else {
						closeCrv = append(closeCrv, currentCurve[currentCurbePoint.PointIdx+1:j+1]...)
					}
					currentCurbePoint = nextCurbePoint
					continue OUT
				}
			}
		}
		return nil
	}
	return
}

// MergeCurvesGeoInt 是 MergeCurves 的 geohash 版本。
//
// curves 中每个 uint64 都是由 WGS84 坐标经 EncodeInt 得到的整型编码。
// 函数先用 DecodeInt 还原为坐标，拼接成闭合环后再重新编码为 []uint64。
// tolerance 同样是米。
//
// 注意：geohash 是有损网格编码，DecodeInt 得到的是网格角点，端点会被
// 量化到网格分辨率上。tolerance 应不小于网格尺寸，否则本应重合的端点
// 可能因量化而判为不相接。若追求精度，请保留原始浮点坐标并用 MergeCurves。
func MergeCurvesGeoInt(curves [][]uint64, tolerance float64) []uint64 {
	coordCurves := make([][]Coords, len(curves))
	for i, cv := range curves {
		cc := make([]Coords, len(cv))
		for j, g := range cv {
			lat, lng := DecodeInt(g)
			cc[j] = Coords{Lat: lat, Lng: lng}
		}
		coordCurves[i] = cc
	}

	ring := MergeCurves(coordCurves, tolerance)
	out := make([]uint64, len(ring))
	for i, c := range ring {
		out[i] = EncodeInt(c.Lat, c.Lng)
	}
	return out
}
