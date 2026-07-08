package geohash

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
