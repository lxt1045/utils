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
	shift         uint                    // 32-h：由 32bit 全精度坐标右移得到 cell 下标
	cellIndex     map[uint64][]CurvePoint // cell key -> 落在该 cell 的节点 id 列表
	countAllPoint int
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

// findNode 在 p 所在 cell 及其 8 邻居中，找距离 <= tolerance、且所属曲线
// 尚未被使用(curvesScanned[CurveIdx]==false)的已索引点。
//
// onlyOne=true: 返回第一个命中点(单结果，用于 MergeCurves2 的确定性单链游走)。
// onlyOne=false: 返回所有命中点，每条曲线去重后只保留一个点(用于 MergeCurves3 的多分支探索)。
//
// 用 curvesScanned 而非单个 excludeCurve：不仅排除「p 自己所在的曲线」，还排除
// 所有已消费的曲线。这样能正确处理「同一段边界被数字化两次」的重叠——一条曲线被
// 走过后即标记 scanned，其重复副本(或已用过的邻接曲线)的点会被跳过，findNode 会
// 在同一 cell 里继续扫描，返回真正未用的粘接曲线，而不是停在一个已用曲线的点上。
func (g *CurvePointIdx) findNode(p Coords, tolerance float64, curvesScanned []bool, onlyOne bool) []CurvePoint {
	cx, cy := g.cellXY(p)
	var result []CurvePoint
	seenCurves := make(map[int]bool) // 每条曲线只保留第一个匹配点

	for dx := -1; dx <= 1; dx++ {
		for dy := -1; dy <= 1; dy++ {
			ncx, ncy := int(cx)+dx, int(cy)+dy
			if ncx < 0 || ncy < 0 {
				continue
			}
			for _, q := range g.cellIndex[cellXYKey(uint32(ncx), uint32(ncy))] {
				if curvesScanned[q.CurveIdx] || seenCurves[q.CurveIdx] {
					continue
				}
				if distMeters(p, q.Coords) <= tolerance {
					result = append(result, q)
					seenCurves[q.CurveIdx] = true
					if onlyOne {
						return result // 立即返回第一个命中
					}
				}
			}
		}
	}
	return result
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

// buildCurvePointIdx 把曲线上的点(按 tolerance/2 步长稀化后)建入 geohash 网格索引。
//
// 前提:输入曲线是「密集点曲线」，相邻点间距 < tolerance/2。粘接点可能出现在曲线
// 中段任意位置(不限于端点)，所以要索引整条曲线的点(而非只端点)，供 findNode 反查。
//
// 稀化规则:沿曲线遍历，只在「当前点与上一个已索引点距离 >= tolerance/2」时存入。
// 用 tolerance/2(而非 tolerance)作步长，保证索引点沿曲线间距 < tolerance——这样另一条
// 曲线上任意与本曲线重合(< tolerance)的查询点，都能在 tolerance 内命中本曲线的某个
// 索引点(步长放大到 tolerance 则可能漏检)。每个点存它自己的坐标/cell/下标。
func buildCurvePointIdx(curves [][]Coords, tolerance float64, bits uint) *CurvePointIdx {
	g := &CurvePointIdx{
		shift:     32 - bits/2,
		cellIndex: make(map[uint64][]CurvePoint),
	}
	step := tolerance / 2
	_ = step
	add := func(p Coords, curveIdx, pointIdx int) {
		cx, cy := g.cellXY(p)
		key := cellXYKey(cx, cy)
		g.cellIndex[key] = append(g.cellIndex[key], CurvePoint{Coords: p, CurveIdx: curveIdx, PointIdx: pointIdx})
	}
	for id, c := range curves {
		g.countAllPoint += len(c)
		if len(c) == 0 {
			continue
		}
		add(c[0], id, 0) // 首点必存
		lastStored := c[0]
		for i := 1; i < len(c); i++ {
			if DistShort(lastStored, c[i]) >= step {
				add(c[i], id, i)
				lastStored = c[i]
			}
		}
		// 末点必存(端点是最常见的粘接点，且末点可能因步长被跳过)。
		if last := len(c) - 1; last > 0 && lastStored != c[last] {
			add(c[last], id, last)
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
	steps := max(nx, ny)
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

// MergeCurves2 将多条无序曲线拼接成一条闭合环，返回环上有序顶点(不含重复首尾点)。
//
// 前提:输入是「密集点曲线」(相邻点间距 < tolerance/2)，曲线间通过粘接点连接，粘接点
// 可能出现在曲线的端点或中段任意位置。本函数沿曲线逐点查找与其他曲线重合(< tolerance)
// 的粘接点，在粘接点处切开当前曲线、接上另一条曲线，直到回到起点形成闭合环。
//
// 与 MergeCurves(snap-rounding)的区别:MergeCurves 对所有顶点做 noding(沿线段 DDA)，
// 能处理任意重采样的中段重叠。本函数只查索引点(稀化到 ~tolerance/2 间距)，更快但只能
// 处理「粘接点对齐」的干净数据(不处理任意重采样的重叠段)。
//
// 若输入曲线不能全部连成单一闭合环(有曲线用不上、或首尾不闭合)，返回 nil。
func MergeCurves2(curves [][]Coords, tolerance float64) (closeCrv []Coords) {
	if len(curves) == 0 {
		return nil
	}

	bits := tolerancePrecisionBits(tolerance)
	g := buildCurvePointIdx(curves, tolerance, bits)
	if len(g.cellIndex) == 0 {
		return nil
	}

	const rootCurveIdx = 0 // 从 curve[0] 的某个端点开始，找一个能匹配到其他曲线的粘接点作根。
	var (
		curvesScanned = make([]bool, len(curves))
		rootPointIdx  = -1
		currentPoint  CurvePoint
	)

	// 根查找期间临时标记 root 已扫，避免 findNode 命中 curve[0] 自己的点；查完复位，
	// 让主循环闭环时能重新匹配回 root。
	curvesScanned[rootCurveIdx] = true
	// 扫顺序：优先尝试终点、起点、实在不行再扫中段。; 简洁: 把复杂扫顺序写道一个循环里
	for i, l := -1, len(curves[rootCurveIdx]); i < l-1; i++ {
		// idx := (i + l - 1) % l
		idx := i
		if i < 0 {
			idx = 0
		} else if i == 0 {
			idx = l - 1
		}
		if matches := g.findNode(curves[rootCurveIdx][idx], tolerance, curvesScanned, true); len(matches) > 0 {
			rootPointIdx = idx
			currentPoint = matches[0]
			break
		}
	}
	curvesScanned[rootCurveIdx] = false // 复位：root 尚未真正走完，闭环时要能匹配回它
	if rootPointIdx < 0 {
		return nil // curve[0] 完全孤立
	}
	// 主循环:沿当前曲线走到粘接点，切换到连接的曲线，直到回到 curve[0]。
	for {
		ci := currentPoint.CurveIdx
		if ci == rootCurveIdx && len(closeCrv) > 0 {
			// 回到起点曲线，闭环。把起点曲线剩余段接上。
			c0 := curves[rootCurveIdx]
			if rootPointIdx < currentPoint.PointIdx {
				// 剩余段:[rootPointIdx, currentPoint.PointIdx] 逆序(含 rootPointIdx)
				for i := currentPoint.PointIdx; i >= rootPointIdx; i-- {
					closeCrv = append(closeCrv, c0[i])
				}
			} else {
				// 剩余段:[currentPoint.PointIdx, rootPointIdx] 顺序(含 rootPointIdx)
				closeCrv = append(closeCrv, c0[currentPoint.PointIdx:rootPointIdx+1]...)
			}
			curvesScanned[rootCurveIdx] = true // 闭环成功，root 也算用上，供末尾"全部用上"检查
			break
		}

		if curvesScanned[ci] {
			return nil // 曲线已扫过，说明死循环或拓扑断裂
		}
		curvesScanned[ci] = true
		currentCurve := curves[ci]

		var nextPoint CurvePoint
		found := false

		// 扫顺序：优先尝试终点、起点、实在不行再扫中段。; 简洁: 把复杂扫顺序写道一个循环里
		for i, l := -1, len(currentCurve); i < l-1; i++ {
			// idx := (i + l - 1) % l
			idx := i
			if i < 0 {
				idx = 0
			} else if i == 0 {
				idx = l - 1
			}
			if distMeters(currentCurve[idx], currentPoint.Coords) <= tolerance {
				continue
			}
			if matches := g.findNode(currentCurve[idx], tolerance, curvesScanned, true); len(matches) > 0 {
				cp := matches[0]
				// 含粘接点端点(idx)：下一条曲线会从其匹配点 cp 再走一遍，故每个关节
				// 产生一对 near-重合点(<tolerance)。它们对 shoelace 面积贡献 ~0，却能把
				// 真实拐点表示精确(与 GEOS 逐点一致)。最后统一丢弃一个闭合尾点。
				if idx < currentPoint.PointIdx {
					for j := currentPoint.PointIdx; j >= idx; j-- {
						closeCrv = append(closeCrv, currentCurve[j])
					}
				} else {
					closeCrv = append(closeCrv, currentCurve[currentPoint.PointIdx:idx+1]...)
				}
				nextPoint = cp
				found = true
				break
			}

			/*
					// 1. 优先尝试 head (curve[0])
					head := currentCurve[0]
					if distMeters(head, currentPoint.Coords) > tolerance {
						if cp, ok := g.findNode(head, tolerance, ci); ok && !curvesScanned[cp.CurveIdx]{
							for i := currentPoint.PointIdx; i >= 0; i-- {
								closeCrv = append(closeCrv, currentCurve[i])
							}
							nextPoint = cp
							found = true
						}
					}

					// 2. 再尝试 tail (curve[len-1])
					if !found {
						tailIdx := len(currentCurve) - 1
						tail := currentCurve[tailIdx]
						if distMeters(tail, currentPoint.Coords) > tolerance {
							if cp, ok := g.findNode(tail, tolerance, ci); ok && !curvesScanned[cp.CurveIdx] {
								closeCrv = append(closeCrv, currentCurve[currentPoint.PointIdx:tailIdx+1]...)
								nextPoint = cp
								found = true
							}
						}
					}

					// 3. 兜底:扫中段 (curve[1:len-1])
					if !found {
						for i := 1; i < len(currentCurve)-1; i++ {
							if distMeters(currentCurve[i], currentPoint.Coords) <= tolerance {
								continue
							}
							if cp, ok := g.findNode(currentCurve[i], tolerance, ci); ok && !curvesScanned[cp.CurveIdx] {
								if i < currentPoint.PointIdx {
									for j := currentPoint.PointIdx; j >= i; j-- {
										closeCrv = append(closeCrv, currentCurve[j])
									}
								} else {
									closeCrv = append(closeCrv, currentCurve[currentPoint.PointIdx:i+1]...)
								}
								nextPoint = cp
								found = true
								break
							}
						}
					}
			//*/
		}
		if !found {
			return nil
		}
		currentPoint = nextPoint
	}

	// 所有曲线都必须被用上
	for _, scanned := range curvesScanned {
		if !scanned {
			return nil
		}
	}

	// 首尾必须闭合(每个接缝有重复点,故末点与首点 near-重合是正常的)
	if len(closeCrv) < 4 || distMeters(closeCrv[0], closeCrv[len(closeCrv)-1]) > tolerance {
		return nil
	}
	return closeCrv[:len(closeCrv)-1] // 丢弃与首点重合的尾点
}

// Ring 表示 MergeCurves3 找到的一条拼接链：顶点序列与是否闭合。
// IsClosed=true 时首尾在 tolerance 内相接(可求面积)；false 时是断裂的开放链
// (重复数字化的重叠曲线、或拓扑断裂的残段)，调用方通常跳过不计面积。
type Ring struct {
	Coords   []Coords
	IsClosed bool
}

// unionFind 把「跨曲线重合的粘接点」并成同一个 junction 节点(带路径减半)。
type unionFind struct{ parent []int }

func newUnionFind(n int) *unionFind {
	p := make([]int, n)
	for i := range p {
		p[i] = i
	}
	return &unionFind{parent: p}
}

func (u *unionFind) find(x int) int {
	for u.parent[x] != x {
		u.parent[x] = u.parent[u.parent[x]]
		x = u.parent[x]
	}
	return x
}

func (u *unionFind) union(a, b int) {
	ra, rb := u.find(a), u.find(b)
	if ra != rb {
		u.parent[ra] = rb
	}
}

// MergeCurves3 把多条无序曲线按粘接点拼接，返回找到的所有环(闭合环 + 开放链)。
//
// 与 MergeCurves2 的区别：MergeCurves2 走贪心单链，要求所有曲线连成单一闭合环，
// 任何多余/重叠曲线都会污染主环或导致整体 nil。MergeCurves3 改成「junction 图 +
// 走链」：先把跨曲线重合的粘接点并成 junction 节点，junction 之间的曲线子路径作为
// 无向边并去重(重复数字化的重叠段产生相同节点对的边，塌成一条被消除)，再沿未用边
// 走链——回到起点为闭合环(IsClosed=true)，走到断头为开放链(false)。
//
// 好处：重复数字化的重叠曲线被边去重自然消除，不再污染真实边界的主环，能稳定还原
// 出正确的闭合环。调用方按 IsClosed 过滤，只对闭合环求面积(见测试)。
//
// 前提与 MergeCurves2 相同：密集点曲线(相邻点间距 < tolerance/2)、粘接点对齐。
func MergeCurves3(curves [][]Coords, tolerance float64) []Ring {
	if len(curves) == 0 {
		return nil
	}

	bits := tolerancePrecisionBits(tolerance)
	g := buildCurvePointIdx(curves, tolerance, bits)
	if len(g.cellIndex) == 0 {
		return nil
	}

	// 1) 收集所有已索引的稀化点，给每个点一个全局 id。
	keyOf := func(cp CurvePoint) uint64 {
		return uint64(uint32(cp.CurveIdx))<<32 | uint64(uint32(cp.PointIdx))
	}
	ptID := make(map[uint64]int) // 每个线的点顶一个唯一id?
	var pts []CurvePoint
	for _, list := range g.cellIndex {
		for _, cp := range list {
			k := keyOf(cp)
			if _, ok := ptID[k]; !ok {
				ptID[k] = len(pts)
				pts = append(pts, cp)
			}
		}
	}

	// 2) 对每个点 findNode(onlyOne=false) 找出 tolerance 内其他曲线的点，用并查集把
	//    这些跨曲线重合的点并成同一个 junction 簇。查询时临时排除自身曲线。
	uf := newUnionFind(len(pts))
	isJunction := make([]bool, len(pts))
	scanned := make([]bool, len(curves))
	for gi, cp := range pts {
		scanned[cp.CurveIdx] = true
		matches := g.findNode(cp.Coords, tolerance, scanned, false)
		scanned[cp.CurveIdx] = false
		for _, m := range matches {
			mi := ptID[keyOf(m)]
			uf.union(gi, mi)
			isJunction[gi] = true
			isJunction[mi] = true
		}
	}

	// 3) 每条曲线按 pointIdx 顺序收集它的 junction 点(簇根作节点 id)。
	type cj struct{ pointIdx, node int }
	perCurve := make([][]cj, len(curves))
	for gi, cp := range pts {
		if !isJunction[gi] {
			continue
		}
		perCurve[cp.CurveIdx] = append(perCurve[cp.CurveIdx], cj{cp.PointIdx, uf.find(gi)})
	}
	for i := range perCurve {
		sort.Slice(perCurve[i], func(a, b int) bool {
			return perCurve[i][a].pointIdx < perCurve[i][b].pointIdx
		})
	}

	// 4) 相邻 junction 之间的曲线子路径构成一条无向边(携带原始密集坐标)。相同节点对
	//    的边只保留一条——重复数字化的重叠段就此塌成一条被消除。
	type edge struct {
		u, v   int
		coords []Coords // 从 u 到 v 的坐标(含两端)
	}
	var edges []edge
	edgeKey := make(map[uint64]bool)
	for i, cjs := range perCurve {
		for j := 0; j+1 < len(cjs); j++ {
			a, b := cjs[j], cjs[j+1]
			if a.node == b.node {
				continue // 同簇自环
			}
			ku, kv := a.node, b.node
			if ku > kv {
				ku, kv = kv, ku
			}
			ek := uint64(uint32(ku))<<32 | uint64(uint32(kv))
			if edgeKey[ek] {
				continue // 重叠段：相同节点对只保留一条
			}
			edgeKey[ek] = true
			coords := append([]Coords(nil), curves[i][a.pointIdx:b.pointIdx+1]...)
			edges = append(edges, edge{u: a.node, v: b.node, coords: coords})
		}
	}
	if len(edges) == 0 {
		return nil
	}

	// 5) 邻接表 + 走链：每次从一条未用边出发，沿未用边走到回起点(闭合环)或断头(开放链)。
	type half struct{ to, edgeID int }
	adj := make(map[int][]half)
	for id, e := range edges {
		adj[e.u] = append(adj[e.u], half{e.v, id})
		adj[e.v] = append(adj[e.v], half{e.u, id})
	}
	used := make([]bool, len(edges))
	step := func(cur int) (to, edgeID int, ok bool) {
		for _, h := range adj[cur] {
			if !used[h.edgeID] {
				return h.to, h.edgeID, true
			}
		}
		return 0, 0, false
	}
	// 把 edge 坐标按 from->to 方向追加(skipFirst 避免与上一段尾点重复)。
	appendEdge := func(ring []Coords, e edge, from int, skipFirst bool) []Coords {
		if e.u == from {
			if skipFirst {
				return append(ring, e.coords[1:]...)
			}
			return append(ring, e.coords...)
		}
		for k := len(e.coords) - 1; k >= 0; k-- {
			if skipFirst && k == len(e.coords)-1 {
				continue
			}
			ring = append(ring, e.coords[k])
		}
		return ring
	}

	var rings []Ring
	var ring []Coords = make([]Coords, g.countAllPoint)
	for id := range edges {
		if used[id] {
			continue
		}
		start := edges[id].u
		cur := start
		// var ring []Coords
		ring = ring[:0]
		first, closed := true, false
		for {
			to, eid, ok := step(cur)
			if !ok {
				break // 断头：开放链
			}
			used[eid] = true
			ring = appendEdge(ring, edges[eid], cur, !first)
			first = false
			cur = to
			if cur == start {
				closed = true
				break
			}
		}
		if closed && len(ring) > 1 && distMeters(ring[0], ring[len(ring)-1]) <= tolerance {
			ring = ring[:len(ring)-1] // 丢弃与首点重合的尾点
		}

		// rings = append(rings, Ring{Coords: ring, IsClosed: closed})
		rings = append(rings, Ring{Coords: append(ring[:0:0], ring...), IsClosed: closed})
	}
	return rings
}

func MergeCurves4(curves [][]Coords, tolerance float64) []Ring {
	if len(curves) == 0 {
		return nil
	}

	bits := tolerancePrecisionBits(tolerance)
	g := buildCurvePointIdx(curves, tolerance, bits)
	if len(g.cellIndex) == 0 {
		return nil
	}

	// 1) 收集所有已索引的稀化点，给每个点一个全局 id。
	keyOf := func(cp CurvePoint) uint64 {
		return uint64(uint32(cp.CurveIdx))<<32 | uint64(uint32(cp.PointIdx))
	}
	ptID := make(map[uint64]int) // 每个线的点顶一个唯一id?
	var pts []CurvePoint
	for _, list := range g.cellIndex {
		for _, cp := range list {
			k := keyOf(cp)
			if _, ok := ptID[k]; !ok {
				ptID[k] = len(pts)
				pts = append(pts, cp)
			}
		}
	}

	// 2) 对每个点 findNode(onlyOne=false) 找出 tolerance 内其他曲线的点，用并查集把
	//    这些跨曲线重合的点并成同一个 junction 簇。查询时临时排除自身曲线。
	uf := newUnionFind(len(pts))
	isJunction := make([]bool, len(pts))
	scanned := make([]bool, len(curves))
	for gi, cp := range pts {
		scanned[cp.CurveIdx] = true
		matches := g.findNode(cp.Coords, tolerance, scanned, false)
		scanned[cp.CurveIdx] = false
		for _, m := range matches {
			mi := ptID[keyOf(m)]
			uf.union(gi, mi)
			isJunction[gi] = true
			isJunction[mi] = true
		}
	}

	// 3) 每条曲线按 pointIdx 顺序收集它的 junction 点(簇根作节点 id)。
	type cj struct{ pointIdx, node int }
	perCurve := make([][]cj, len(curves))
	for gi, cp := range pts {
		if !isJunction[gi] {
			continue
		}
		perCurve[cp.CurveIdx] = append(perCurve[cp.CurveIdx], cj{cp.PointIdx, uf.find(gi)})
	}
	for i := range perCurve {
		sort.Slice(perCurve[i], func(a, b int) bool {
			return perCurve[i][a].pointIdx < perCurve[i][b].pointIdx
		})
	}

	// 4) 相邻 junction 之间的曲线子路径构成一条无向边(携带原始密集坐标)。相同节点对
	//    的边只保留一条——重复数字化的重叠段就此塌成一条被消除。
	type edge struct {
		u, v   int
		coords []Coords // 从 u 到 v 的坐标(含两端)
	}
	var edges []edge
	edgeKey := make(map[uint64]bool)
	for i, cjs := range perCurve {
		for j := 0; j+1 < len(cjs); j++ {
			a, b := cjs[j], cjs[j+1]
			if a.node == b.node {
				continue // 同簇自环
			}
			ku, kv := a.node, b.node
			if ku > kv {
				ku, kv = kv, ku
			}
			ek := uint64(uint32(ku))<<32 | uint64(uint32(kv))
			if edgeKey[ek] {
				continue // 重叠段：相同节点对只保留一条
			}
			edgeKey[ek] = true
			coords := append([]Coords(nil), curves[i][a.pointIdx:b.pointIdx+1]...)
			edges = append(edges, edge{u: a.node, v: b.node, coords: coords})
		}
	}
	if len(edges) == 0 {
		return nil
	}

	// 5) 邻接表 + 走链：每次从一条未用边出发，沿未用边走到回起点(闭合环)或断头(开放链)。
	type half struct{ to, edgeID int }
	adj := make(map[int][]half)
	for id, e := range edges {
		adj[e.u] = append(adj[e.u], half{e.v, id})
		adj[e.v] = append(adj[e.v], half{e.u, id})
	}
	used := make([]bool, len(edges))
	step := func(cur int) (to, edgeID int, ok bool) {
		for _, h := range adj[cur] {
			if !used[h.edgeID] {
				return h.to, h.edgeID, true
			}
		}
		return 0, 0, false
	}
	// 把 edge 坐标按 from->to 方向追加(skipFirst 避免与上一段尾点重复)。
	appendEdge := func(ring []Coords, e edge, from int, skipFirst bool) []Coords {
		if e.u == from {
			if skipFirst {
				return append(ring, e.coords[1:]...)
			}
			return append(ring, e.coords...)
		}
		for k := len(e.coords) - 1; k >= 0; k-- {
			if skipFirst && k == len(e.coords)-1 {
				continue
			}
			ring = append(ring, e.coords[k])
		}
		return ring
	}

	var rings []Ring
	var ring []Coords = make([]Coords, g.countAllPoint)
	for id := range edges {
		if used[id] {
			continue
		}
		start := edges[id].u
		cur := start
		// var ring []Coords
		ring = ring[:0]
		first, closed := true, false
		for {
			to, eid, ok := step(cur)
			if !ok {
				break // 断头：开放链
			}
			used[eid] = true
			ring = appendEdge(ring, edges[eid], cur, !first)
			first = false
			cur = to
			if cur == start {
				closed = true
				break
			}
		}
		if closed && len(ring) > 1 && distMeters(ring[0], ring[len(ring)-1]) <= tolerance {
			ring = ring[:len(ring)-1] // 丢弃与首点重合的尾点
		}

		// rings = append(rings, Ring{Coords: ring, IsClosed: closed})
		rings = append(rings, Ring{Coords: append(ring[:0:0], ring...), IsClosed: closed})
	}
	return rings
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
