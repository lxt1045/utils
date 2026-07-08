package geohash

import "math"

// 本文件实现"将多条无序曲线拼接成一条闭合环(多边形边界)"的功能。
//
// 场景：一个区域的边界被拆成了若干条曲线(polyline)分别存储，
// 这些曲线之间没有先后顺序，方向(正向/反向)也不确定，相邻曲线
// 在衔接处往往共享同一个端点(甚至因精度问题略有偏差)。
// 需要把它们首尾相接地拼回一条闭合环，并在衔接处去重。

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

// dedupConsecutive 折叠"相邻且距离 <= tolerance"的重复点，返回新切片。
// 用于清理单条曲线内部由采样/量化产生的冗余点。
func dedupConsecutive(pts []Coords, tolerance float64) []Coords {
	if len(pts) == 0 {
		return nil
	}
	out := make([]Coords, 0, len(pts))
	out = append(out, pts[0])
	for i := 1; i < len(pts); i++ {
		if distMeters(out[len(out)-1], pts[i]) > tolerance {
			out = append(out, pts[i])
		}
	}
	return out
}

// MergeCurves 将多条无序曲线拼接成一条闭合环，返回环上的有序顶点。
//
// curves    : 每条曲线是一串有序坐标点(WGS84，度)；曲线之间无先后顺序，
//             方向也不确定(函数会按需反转)。
// tolerance : 可接受误差(米)。用于两处判定：
//             1) 单条曲线内相邻重复点的折叠；
//             2) 曲线端点之间是否"重合"从而可以衔接/去重。
//
// 返回：拼接后的闭合环顶点列表(不重复首尾点，最后一点到第一点为隐含闭合边)。
// 该结果可直接传入 AreaCoords 求面积。
//
// 算法(贪心链式拼接，类似 JTS LineMerger)：
//  1. 先对每条曲线做内部去重；
//  2. 取一条曲线作为初始链，记录链的首端 head 与尾端 tail；
//  3. 反复在剩余曲线中寻找某个端点与 head 或 tail 距离 <= tolerance 的曲线，
//     必要时反转后接到链的对应端，并丢弃重合的那个衔接点(去重)；
//  4. 直到没有曲线可再衔接；
//  5. 若首尾两端也在 tolerance 内，则丢弃重复的尾点，得到闭合环。
//
// 注意：若输入曲线并不能全部连成单一环(存在断裂或多个独立环)，
// 本函数只返回从初始曲线出发能连通的那一条链。
func MergeCurves(curves [][]Coords, tolerance float64) []Coords {
	cleaned := make([][]Coords, 0, len(curves))
	for _, c := range curves {
		if d := dedupConsecutive(c, tolerance); len(d) > 0 {
			cleaned = append(cleaned, d)
		}
	}
	if len(cleaned) == 0 {
		return nil
	}

	used := make([]bool, len(cleaned))
	chain := append([]Coords(nil), cleaned[0]...)
	used[0] = true

	for {
		head, tail := chain[0], chain[len(chain)-1]
		found := false
		for i, c := range cleaned {
			if used[i] {
				continue
			}
			c0, cn := c[0], c[len(c)-1]
			switch {
			case distMeters(tail, c0) <= tolerance:
				// tail 接 c 的起点：丢弃 c0(与 tail 重合)后追加到尾部
				chain = append(chain, c[1:]...)
			case distMeters(tail, cn) <= tolerance:
				// tail 接 c 的终点：反转 c，丢弃其首点(原 cn)后追加
				chain = append(chain, reverseCoords(c)[1:]...)
			case distMeters(head, cn) <= tolerance:
				// c 的终点接 head：丢弃 cn 后整体前置
				chain = append(append([]Coords(nil), c[:len(c)-1]...), chain...)
			case distMeters(head, c0) <= tolerance:
				// c 的起点接 head：反转 c，丢弃其尾点(原 c0)后前置
				chain = append(append([]Coords(nil), reverseCoords(c)[:len(c)-1]...), chain...)
			default:
				continue
			}
			used[i] = true
			found = true
			break
		}
		if !found {
			break
		}
	}

	// 闭合：首尾重合则丢弃重复的尾点，返回不含重复首尾的环。
	if len(chain) > 1 && distMeters(chain[0], chain[len(chain)-1]) <= tolerance {
		chain = chain[:len(chain)-1]
	}
	return chain
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

// tolerancePrecisionBits 依 tolerance(米)选一个 geohash 精度 bits(偶数，latBits==lngBits)，
// 使每个网格 cell 的经、纬向边长都 >= tolerance——从而任意两个距离 <= tolerance 的点
// 必定落在同一 cell 或其 8 个邻居 cell 内，9 格搜索即完备(不漏点)。
//
// 参考纬度取数据中 |lat| 的最大值：纬度越高 cos 越小、经向 cell 越窄，用它最保守。
func tolerancePrecisionBits(cleaned [][]Coords, tolerance float64) uint {
	if tolerance <= 0 {
		return 52 // 退化输入：给一个较高精度即可
	}
	maxAbsLat := 0.0
	for _, c := range cleaned {
		for _, p := range c {
			if a := math.Abs(p.Lat); a > maxAbsLat {
				maxAbsLat = a
			}
		}
	}
	if maxAbsLat > 89 {
		maxAbsLat = 89 // 防止 cos 过小导致 bits 过低
	}
	cosPhi := math.Cos(maxAbsLat * degToRad)

	const mPerDeg = 111320.0 // 每纬度约 111.32km
	// 单个 cell(latBits==lngBits==h)的边长(米)：
	//   纬向 = mPerDeg * 180   / 2^h
	//   经向 = mPerDeg * cosPhi*360 / 2^h
	// 取较小者 >= tolerance  =>  2^h <= mPerDeg*min(180, cosPhi*360)/tolerance
	minSpanDeg := math.Min(180.0, cosPhi*360.0)
	ratio := mPerDeg * minSpanDeg / tolerance

	h := 0
	if ratio >= 1 {
		h = int(math.Floor(math.Log2(ratio)))
	}
	if h < 1 {
		h = 1
	}
	if h > 31 {
		h = 31
	}
	return uint(2 * h)
}

// MergeCurvesIndexed 是 MergeCurves 的加速版本：功能与结果和 MergeCurves 一致
// (把无序、方向不定、端点近似重合的曲线拼成一条闭合环)，但把原来对每条链端
// O(曲线数) 的线性扫描，换成基于 geohash 网格索引的近邻查找，整体从约 O(曲线数²)
// 降到约 O(曲线数)。
//
// 算法：
//  1. 每条曲线先做内部去重(dedupConsecutive)。
//  2. 只把每条曲线的两个「端点」编码成 geohash 网格 cell 建索引——拼接只发生在
//     端点处，曲线内部点不可能成为衔接点，无需入索引(这也是相对“索引所有点”
//     更省的地方)。网格精度由 tolerancePrecisionBits 依 tolerance 选取，保证
//     cell 边长 >= tolerance。
//  3. 拼接时，对链的 tail / head 各取「所在 cell + 8 邻居(NeighborsInt)」共 9 个
//     cell，从索引取候选端点，再用 distMeters 精确校验 <= tolerance 才衔接。
//  4. 其余逻辑(按需反转、衔接处去重、末尾闭合)与 MergeCurves 相同。
//
// 结果与 MergeCurves 表示同一条环(顶点序列可能因遍历顺序不同而整体旋转或反向)。
func MergeCurvesIndexed(curves [][]Coords, tolerance float64) []Coords {
	cleaned := make([][]Coords, 0, len(curves))
	for _, c := range curves {
		if d := dedupConsecutive(c, tolerance); len(d) > 0 {
			cleaned = append(cleaned, d)
		}
	}
	if len(cleaned) == 0 {
		return nil
	}
	if len(cleaned) == 1 {
		chain := append([]Coords(nil), cleaned[0]...)
		if len(chain) > 1 && distMeters(chain[0], chain[len(chain)-1]) <= tolerance {
			chain = chain[:len(chain)-1]
		}
		return chain
	}

	bits := tolerancePrecisionBits(cleaned, tolerance)
	mask := uint64(math.MaxUint64) << (64 - bits)
	cellOf := func(p Coords) uint64 {
		return EncodeInt(p.Lat, p.Lng) & mask
	}

	// 端点索引：cell -> 落在该 cell 的端点引用列表。
	type endRef struct {
		curve int  // 曲线下标
		tail  bool // false=起点 c[0]；true=终点 c[len-1]
	}
	index := make(map[uint64][]endRef, len(cleaned)*2)
	for ci, c := range cleaned {
		index[cellOf(c[0])] = append(index[cellOf(c[0])], endRef{ci, false})
		last := len(c) - 1
		index[cellOf(c[last])] = append(index[cellOf(c[last])], endRef{ci, true})
	}

	used := make([]bool, len(cleaned))

	// findNear 在 query 点附近的 9 个 cell 中，找一条未使用曲线、且距离 <= tolerance
	// 的端点。返回曲线下标、该端点是否为终点、是否找到。
	findNear := func(query Coords) (ci int, tail bool, ok bool) {
		check := func(cell uint64) (int, bool, bool) {
			for _, ref := range index[cell] {
				if used[ref.curve] {
					continue
				}
				c := cleaned[ref.curve]
				ep := c[0]
				if ref.tail {
					ep = c[len(c)-1]
				}
				if distMeters(query, ep) <= tolerance {
					return ref.curve, ref.tail, true
				}
			}
			return 0, false, false
		}

		base := cellOf(query)
		if ci, tail, ok = check(base); ok { // 先查自身 cell
			return
		}
		for _, cell := range NeighborsInt(base, bits) { // 再查 8 邻居
			if ci, tail, ok = check(cell); ok {
				return
			}
		}
		return 0, false, false
	}

	chain := append([]Coords(nil), cleaned[0]...)
	used[0] = true

	for {
		// 先尝试在尾部衔接。
		if ci, isTail, ok := findNear(chain[len(chain)-1]); ok {
			c := cleaned[ci]
			if !isTail {
				chain = append(chain, c[1:]...) // tail 接 c 起点：丢弃 c0
			} else {
				chain = append(chain, reverseCoords(c)[1:]...) // tail 接 c 终点：反转后丢弃原 cn
			}
			used[ci] = true
			continue
		}
		// 再尝试在头部衔接。
		if ci, isTail, ok := findNear(chain[0]); ok {
			c := cleaned[ci]
			if isTail {
				chain = append(append([]Coords(nil), c[:len(c)-1]...), chain...) // c 终点接 head：丢弃 cn 后前置
			} else {
				chain = append(append([]Coords(nil), reverseCoords(c)[:len(c)-1]...), chain...) // c 起点接 head：反转后丢弃原 c0 前置
			}
			used[ci] = true
			continue
		}
		break
	}

	// 闭合：首尾重合则丢弃重复的尾点。
	if len(chain) > 1 && distMeters(chain[0], chain[len(chain)-1]) <= tolerance {
		chain = chain[:len(chain)-1]
	}
	return chain
}
