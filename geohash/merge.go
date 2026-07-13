package geohash

import (
	"context"
	"math"
	"sort"

	"github.com/lxt1045/utils/log"
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

// key：把 (CurveIdx, PointIdx) 打包成 uint64 唯一键，用于跨 cell 去重同一个点
// (一个点可能因边界原因被登记多次，需保证每个物理点只进 pts 一次)。
func (cp CurvePoint) key() uint64 {
	return uint64(uint32(cp.CurveIdx))<<32 | uint64(uint32(cp.PointIdx))
}

// snapGrid 保存 snap-rounding 用的节点集合与网格索引。
type CurvePointIdx struct {
	shift         uint                    // 32-h：由 32bit 全精度坐标右移得到 cell 下标
	cellIndex     map[uint64][]CurvePoint // cell key -> 落在该 cell 的节点 id 列表
	countAllPoint int
	countCurve    int

	junctionsUsed map[uint64]bool // 同一个点只处理一次
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
func (g *CurvePointIdx) findNode(p Coords, tolerance float64, id int, onlyOne bool) (result []CurvePoint) {
	cx, cy := g.cellXY(p)
	seenCurves := make([]bool, g.countCurve)
	seenCurves[id] = true

	for dx := -1; dx <= 1; dx++ {
		for dy := -1; dy <= 1; dy++ {
			ncx, ncy := int(cx)+dx, int(cy)+dy
			if ncx < 0 || ncy < 0 {
				continue
			}
			for _, q := range g.cellIndex[cellXYKey(uint32(ncx), uint32(ncy))] {
				if seenCurves[q.CurveIdx] {
					continue
				}
				if DistShort(p, q.Coords) <= tolerance {
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
		shift:         32 - bits/2,
		cellIndex:     make(map[uint64][]CurvePoint),
		junctionsUsed: make(map[uint64]bool),
		countCurve:    len(curves),
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

// Ring 表示 MergeCurves3 找到的一条拼接链：顶点序列与是否闭合。
// IsClosed=true 时首尾在 tolerance 内相接(可求面积)；false 时是断裂的开放链
// (重复数字化的重叠曲线、或拓扑断裂的残段)，调用方通常跳过不计面积。
type Ring struct {
	Coords   []Coords
	IsClosed bool
}

// unionFind 把「跨曲线重合的粘接点」并成同一个 junction 节点(带路径减半)。
type unionFind struct {
	matchPoints []int // 粘接点对: 初始化时 idx:idx, 扫描到时 idx: matchIdx; 多条曲线(超过2条)的共同点: 碰撞处理,链式处理
}

func newUnionFind(n int) *unionFind {
	p := make([]int, n)
	for i := range p {
		p[i] = i
	}
	return &unionFind{matchPoints: p}
}

func (u *unionFind) find(x int) int {
	for u.matchPoints[x] != x {
		u.matchPoints[x] = u.matchPoints[u.matchPoints[x]] // 解决碰撞的方式: 链式处理
		x = u.matchPoints[x]
	}
	return x
}

func (u *unionFind) union(a, b int) {
	ra, rb := u.find(a), u.find(b)
	if ra != rb {
		u.matchPoints[ra] = rb
	}
}

// unionFind 把「跨曲线重合的粘接点」并成同一个 junction 节点(带路径减半)。
type unionFind2 struct {
	matchPoints map[int]*int // 粘接点对: 初始化时 idx:idx, 扫描到时 idx: matchIdx; 多条曲线(超过2条)的共同点: 碰撞处理,链式处理
}

func newUnionFind2(n int) *unionFind2 {
	return &unionFind2{matchPoints: make(map[int]*int)}
}

func (u *unionFind2) find(x int) int {
	p := u.matchPoints[x]
	if p != nil {
		return *p
	}
	return x
}

func (u *unionFind2) union(a, b int) {
	p := u.matchPoints[a]
	q := u.matchPoints[b]
	if p == nil {
		if q == nil {
			p = &b
			u.matchPoints[a] = p
			u.matchPoints[b] = p
		} else {
			*q = b
			u.matchPoints[a] = q
		}
	} else if q == nil {
		*p = b
		u.matchPoints[b] = p
	} else {
		*p = b
		*q = b
	}
	// ra, rb := u.find(a), u.find(b)
	// if ra != rb {
	// 	u.matchPoints[ra] = rb
	// }
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
//
// 与 MergeCurves4 的选型(基准 50~800 条曲线,见 TestMergeCurves4/BenchmarkMergeCurves3vs4)：
// 两者正确性等价(闭合主环面积逐点一致,误差 < 1e-7)。差异在重叠段处理与规模表现——
//   - MergeCurves3(本函数,建图+单次走边)：重叠段靠「无序节点对」边去重,恒还原出 1 条
//     主闭合环,输出干净;内存随规模线性、稳定。重叠为主且规模大(≥200 条)时更省内存、更快。
//   - MergeCurves4(DFS+分支去重)：干净数据全面更快更省(耗时 0.7~0.8x、内存低至 0.54x)；
//     但重叠数据会残留 3~4 条退化闭合环(不影响主环,取最大环即可),大规模比本函数慢 ~1.35x、
//     内存 ~1.5x。
// 经验法则：干净/近干净数据首选 MergeCurves4;重叠为主且规模大用 MergeCurves3。
func MergeCurves3(curves [][]Coords, tolerance float64) []Ring {
	if len(curves) == 0 {
		return nil
	}

	// bits：由 tolerance 反推的 geohash 每轴精度(总 bits=2h)，保证网格 cell 较短边
	// >= tolerance，使得 9 邻域搜索对「距离 <= tolerance 的点对」完备(不漏点)。
	bits := tolerancePrecisionBits(tolerance)
	// g：曲线点的网格索引。buildCurvePointIdx 会对每条曲线做「密集点稀化」——沿曲线
	// 每隔约 tolerance/2 才保留一个点建索引，既降低点数又不丢失粘接判定所需的分辨率。
	// g.cellIndex: cell key -> 落在该 cell 的稀化点列表；g.countAllPoint: 稀化点总数。
	g := buildCurvePointIdx(curves, tolerance, bits)
	if len(g.cellIndex) == 0 {
		return nil
	}

	// 1) 把各 cell 里的稀化点摊平成全局数组 PointsIdx(下标即 gid)。buildCurvePointIdx
	//    对每个物理点只 add 一次，故无需再按 key 去重。
	var PointsIdx []CurvePoint
	for _, list := range g.cellIndex {
		PointsIdx = append(PointsIdx, list...)
	}

	// 2) 纯 cell 吸附：node = 掩码后的 geohash cell(确定性吸附，落进同一 cell 即同一 node)。
	//    相比旧版「findNode 邻域扫描 + 并查集两两聚类」，这里不做邻域搜索、不建并查集：
	//    两份「独立重采样的重叠曲线」只要落进同一串 cell 就得到*相同的 node 序列*，第 4 步
	//    的 edgeKey 去重才能把重叠段真正塌成一条边——避免旧版因两两聚类的簇边界不对称而
	//    残留大量平行小边、把主环顶点数撑到虚高。
	//    代价：恰好处于 cell 边界两侧、相距 <tolerance 的点可能被分到相邻 node(tolerance 级
	//    残留)，符合容差语义；cell 边长 >= tolerance，精确重合/近似重合的点绝大多数落同一 cell。
	//    junction：被 >=2 条*不同*曲线触及的 cell 即粘接/交叉处；仅 junction 参与建图，
	//    曲线内部点(只被本曲线触及)不入图。
	cellNode := make(map[uint64]int) // cellKey -> node id
	var nodeCurve []int              // node -> 首个触及它的曲线(-1 表示尚未登记)
	var nodeIsJunc []bool            // node 是否 junction
	pointNode := make([]int, len(PointsIdx))
	for idx, p := range PointsIdx {
		cx, cy := g.cellXY(p.Coords)
		ck := cellXYKey(cx, cy)
		nd, ok := cellNode[ck]
		if !ok {
			nd = len(nodeCurve)
			cellNode[ck] = nd
			nodeCurve = append(nodeCurve, -1)
			nodeIsJunc = append(nodeIsJunc, false)
		}
		pointNode[idx] = nd
		switch {
		case nodeCurve[nd] == -1:
			nodeCurve[nd] = p.CurveIdx // 首次触及：记下曲线
		case nodeCurve[nd] != p.CurveIdx:
			nodeIsJunc[nd] = true // 被第二条不同曲线触及：确认是 junction
		}
	}

	// 3) 按曲线归拢 junction 点(接合点、交叉点)，并沿曲线走向(pointIdx 升序)排好序。
	//    perCurve[i]：第 i 条曲线上所有 junction 点，排序后相邻两个之间正好夹着一段
	//    「不含其它 junction 的子路径」——这就是下一步要抽出的图的边。
	type CurveJunctionNode struct {
		pointIdx, node int // 曲线上一个 junction 的 {沿线位置 pointIdx, 所属 node id(=cell)}。
	}
	curveJunctionNodes := make([][]CurveJunctionNode, len(curves)) // 每条曲线上所有的 junction 点(接合点、交叉点)
	for idx, ponit := range PointsIdx {
		if !nodeIsJunc[pointNode[idx]] {
			continue // 曲线内部点不入图
		}
		curveJunctionNodes[ponit.CurveIdx] = append(curveJunctionNodes[ponit.CurveIdx], CurveJunctionNode{
			pointIdx: ponit.PointIdx,
			node:     pointNode[idx],
		})
	}
	for i := range curveJunctionNodes {
		sort.Slice(curveJunctionNodes[i], func(a, b int) bool {
			return curveJunctionNodes[i][a].pointIdx < curveJunctionNodes[i][b].pointIdx
		})
	}

	// 4) 建图：每条曲线上相邻两个 junction 之间的子路径 => 一条无向边(携带原始密集坐标)。
	//    edge.u/v：两端 junction 的节点 idx；edge.coords：从 u 到 v 的完整密集坐标(含两端)。
	//    edgeKey：以「无序节点对」为键去重——同一段边界被数字化两次会产生端点相同的两条
	//    边，只保留一条，重叠段就此塌成单边被消除。
	var edges []Edge
	edgeKey := make(map[uint64]bool) // 无序节点对键 -> 是否已建过边
	for curveIdx, junctionNodes := range curveJunctionNodes {
		for j := 0; j+1 < len(junctionNodes); j++ {
			a, b := junctionNodes[j], junctionNodes[j+1]
			if a.node == b.node {
				continue // 两个 junction 同簇：退化自环，跳过
			}
			ku, kv := a.node, b.node
			if ku > kv { // 归一成无序对(小的在前)，让 u->v 与 v->u 命中同一键
				ku, kv = kv, ku
			}
			ek := uint64(uint32(ku))<<32 | uint64(uint32(kv))
			if edgeKey[ek] {
				continue // 重叠段：相同节点对只保留一条
			}
			edgeKey[ek] = true
			// 截取该子路径的原始密集坐标(左闭右闭)，独立拷贝一份，避免与源切片别名。
			coords := append([]Coords(nil), curves[curveIdx][a.pointIdx:b.pointIdx+1]...)
			edges = append(edges, Edge{start: a.node, end: b.node, coords: coords})
		}
	}
	if len(edges) == 0 {
		return nil
	}

	// 5) 沿边走链，把图还原成一条条链/环。
	//    half：从某个节点出发的一条「半边」——通向 to、对应第 edgeID 条边。
	//    adj：邻接表，每条无向边在两端各挂一条半边。
	//    used：边是否已被走过(每条边只用一次)。
	type half struct{ to, edgeID int }
	adj := make(map[int][]half)
	for id, e := range edges {
		adj[e.start] = append(adj[e.start], half{e.end, id})
		adj[e.end] = append(adj[e.end], half{e.start, id})
	}
	used := make([]bool, len(edges))
	// step：从 cur 出发挑一条尚未用过的邻接边，返回对端节点与边号；无可走边则 ok=false。
	step := func(cur int) (to, edgeID int, ok bool) {
		for _, h := range adj[cur] {
			if !used[h.edgeID] {
				return h.to, h.edgeID, true
			}
		}
		return 0, 0, false
	}
	// appendEdge：把一条边的坐标按「from->对端」的方向追加进 ring。
	// edge.coords 存的是 u->v 方向；若本次是从 v 进入(from!=u)，需逆序追加。
	// skipFirst：跳过该边的起点，避免和上一段的尾点重复(相邻边共享 junction 顶点)。
	appendEdge := func(ring []Coords, e Edge, from int, skipFirst bool) []Coords {
		if e.start == from { // 正向：u->v
			if skipFirst {
				return append(ring, e.coords[1:]...)
			}
			return append(ring, e.coords...)
		}
		for k := len(e.coords) - 1; k >= 0; k-- { // 逆向：v->u
			if skipFirst && k == len(e.coords)-1 {
				continue
			}
			ring = append(ring, e.coords[k])
		}
		return ring
	}

	var rings []Ring
	// ring：走链时复用的坐标缓冲，预分配到稀化点总数上界，每条链游走前 ring[:0] 清空复用。
	var ring []Coords = make([]Coords, g.countAllPoint)
	for id := range edges {
		if used[id] {
			continue // 该边已被并入某条链
		}
		start := edges[id].start // 以这条边的一端为链起点
		cur := start
		ring = ring[:0]
		first, closed := true, false // first：是否第一段(第一段不跳起点)；closed：是否走回起点
		for {
			to, eid, ok := step(cur)
			if !ok {
				break // 走到断头：这是一条开放链(非闭合)
			}
			used[eid] = true
			ring = appendEdge(ring, edges[eid], cur, !first)
			first = false
			cur = to
			if cur == start {
				closed = true // 走回起点：闭合环
				break
			}
		}
		// 闭合环的尾点与首点是同一 junction，若二者在 tolerance 内则去掉重复尾点。
		if closed && len(ring) > 1 && distMeters(ring[0], ring[len(ring)-1]) <= tolerance {
			ring = ring[:len(ring)-1]
		}

		// 用精确大小拷贝落盘：ring 本身是复用缓冲，下一轮会被覆盖，必须复制一份独立切片。
		rings = append(rings, Ring{Coords: append(ring[:0:0], ring...), IsClosed: closed})
	}
	return rings
}

// 边
type Edge struct {
	curveIdx int // 曲线idx
	start    int
	end      int      // 两端 junction 的节点 idx
	coords   []Coords // 从 u 到 v 的完整密集坐标(含两端)
}
type RingEdge struct {
	Edges    []Edge
	IsClosed bool
}

type Matche struct {
	pointIdx int          // 曲线 a 的idx
	matches  []CurvePoint // 曲线n 的idx在这里
}
type Stack struct {
	edges         []Edge // 曲线的边的顺序
	curvesScanned []bool // 已经扫描过的曲线

	curves [][]Coords // 所有曲线

	// 当前正在处理的点
	CurveIdx      int
	startPointIdx int
}

func (s Stack) Clone() (out Stack) {
	out = s
	out.curvesScanned = append(s.curvesScanned[:0:0], s.curvesScanned...)
	return
}

// MergeCurves4 的嵌套部分
func mergeCurves(ctx context.Context, stack Stack, tolerance float64, g *CurvePointIdx) (edges []*RingEdge) {
	if stack.curvesScanned[stack.CurveIdx] {
		if len(stack.edges) <= 1 {
			return
		}
		var lastPointIdx int
		for lastPointIdx = range stack.edges {
			if stack.edges[lastPointIdx].curveIdx == stack.CurveIdx {
				break
			}
		}
		if stack.edges[lastPointIdx].curveIdx != stack.CurveIdx {
			log.Ctx(ctx).Warn().Msgf("数据错误, 已经扫描过的曲线没有出现在栈里")
			return
		}

		edges = append(edges, &RingEdge{
			Edges: append(append(stack.edges[:0:0], stack.edges[lastPointIdx+1:]...), Edge{
				curveIdx: stack.CurveIdx,
				start:    stack.startPointIdx,
				end:      stack.edges[lastPointIdx].end, //
			}),
			IsClosed: true,
		})
		return // 曲线已扫过，说明死循环或拓扑断裂
	}

	stack.curvesScanned[stack.CurveIdx] = true
	currentCurve := stack.curves[stack.CurveIdx]

	// 扫顺序：优先尝试终点、起点、实在不行再扫中段。; 简洁: 把复杂扫顺序写道一个循环里
	none := true
	lastStored, step := currentCurve[0], tolerance/2
	userCurves := make([]bool, g.countCurve) // 同一条曲线的交叉线只使用一次
	for idx := range currentCurve {
		if idx != 0 && idx != len(currentCurve)-1 && DistShort(lastStored, currentCurve[idx]) < step {
			continue
		}
		lastStored = currentCurve[idx]
		if DistShort(currentCurve[idx], currentCurve[stack.startPointIdx]) <= tolerance {
			continue
		}
		matches := g.findNode(currentCurve[idx], tolerance, stack.CurveIdx, false)
		if len(matches) > 0 {
			for i, match := range matches {
				k := match.key()
				if userCurves[match.CurveIdx] || g.junctionsUsed[k] {
					continue
				}
				g.junctionsUsed[k] = true

				userCurves[match.CurveIdx] = true

				none = false
				s0 := stack
				if idx != len(currentCurve)-1 || i != len(matches)-1 {
					s0 = s0.Clone() // 不是最后一个粘接点，就都需要 Clone 一份, 避免相互污染
				}
				s0.edges = append(s0.edges, Edge{
					curveIdx: stack.CurveIdx,
					start:    stack.startPointIdx,
					end:      idx,
				})
				s0.startPointIdx = match.PointIdx
				s0.CurveIdx = match.CurveIdx

				edges1 := mergeCurves(ctx, s0, tolerance, g)
				edges = append(edges, edges1...)
			}
		}
	}
	// 这是一条开放的曲线
	if none {
		edges = append(edges, &RingEdge{
			Edges: append(append(stack.edges[:0:0], stack.edges...), Edge{
				curveIdx: stack.CurveIdx,
				start:    stack.startPointIdx,
				end:      len(stack.curves[stack.CurveIdx]) - 1, // 开放的曲线
			}),
			IsClosed: false,
		})
		return
	}
	return
}

// MergeCurves4 与 MergeCurves3 等价：把多条无序曲线按粘接点拼接，返回所有闭合环。
// 二者结果正确性一致(干净数据面积逐点相同；重叠数据面积误差 < 1e-7)。
//
// 算法差异：MergeCurves3 用「junction 图 + 单次走边」(建无向边并按节点对去重叠，
// 再线性走链)；MergeCurves4 用「DFS 递归 + 分支回溯」，配合 junctionsUsed(全局粘接
// 点去重)与 userCurves(同曲线只接一次)抑制分支爆炸，末尾只保留闭合环(IsClosed=true)。
//
// 选型建议(基准见 BenchmarkMergeCurves3vs4，50~800 曲线)：
//   - 干净数据(无重复数字化)：优先 MergeCurves4——耗时约 0.7~0.8x、内存约 0.6~0.8x，全面更省。
//   - 重叠为主且规模大(≥500 曲线)：优先 MergeCurves3——它靠边去重使重叠段恒塌成 1 环；
//     MergeCurves4 的 DFS 对重叠段会残留 3~4 个退化闭合环，大规模下慢约 1.3x、内存约 1.5x。
//
// 前提与 MergeCurves3 相同：密集点曲线(相邻点间距 < tolerance/2)、粘接点对齐。
func MergeCurves4(curves [][]Coords, tolerance float64) (rings []Ring) {
	if len(curves) == 0 {
		return nil
	}

	bits := tolerancePrecisionBits(tolerance)
	g := buildCurvePointIdx(curves, tolerance, bits)
	if len(g.cellIndex) == 0 {
		return nil
	}

	s0 := Stack{
		edges:         nil,
		curvesScanned: make([]bool, len(curves)), // 已经扫描过的曲线
		curves:        curves,

		// 当前正在处理的点
		CurveIdx:      0,
		startPointIdx: 0,
	}

	// TODO: 未用的曲线，也要走一遍
	edges := mergeCurves(context.TODO(), s0, tolerance, g)
	// ring：走链时复用的坐标缓冲，预分配到稀化点总数上界，每条链游走前 ring[:0] 清空复用。
	var ring []Coords = make([]Coords, g.countAllPoint)
	for _, edge := range edges {
		if !edge.IsClosed {
			continue
		}
		ring = ring[:0]
		for _, r := range edge.Edges {
			if r.start > r.end {
				for ; r.start > r.end; r.start-- {
					ring = append(ring, curves[r.curveIdx][r.start])
				}
			} else {
				ring = append(ring, curves[r.curveIdx][r.start:r.end]...)
			}
		}
		rings = append(rings, Ring{
			IsClosed: edge.IsClosed,
			Coords:   append(ring[:0:0], ring...),
		})
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
