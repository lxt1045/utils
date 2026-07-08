package geohash

import "math"

// WGS84 椭球参数与等面积球半径。
//
// 计算球面多边形面积时，选用"等面积球半径"(authalic radius)：
// 它是一个与 WGS84 椭球表面积相等的正球体半径，用它做球面近似时
// 面积误差最小。数值由 WGS84 (a=6378137, f=1/298.257223563) 推导而来。
const (
	authalicRadius    = 6371007.1809184751 // 等面积球半径(米)
	authalicRadiusSqr = authalicRadius * authalicRadius
)

// AreaCoords 计算 WGS84 坐标点(闭合曲线)围成的多边形面积，单位：平方米。
//
// 输入是按顺序排列的顶点列表(经纬度，度)，表示一条闭合曲线。
// 首尾点是否显式相接都可以：函数内部按环处理，会自动连接最后一点到第一点。
// 顶点顺序(顺时针/逆时针)不影响结果，返回值始终为非负。
//
// 算法：球面多边形的"球面角盈"(spherical excess)公式，与 paulmach/orb 的
// geo.Area 使用同一方法。它把地球近似为等面积球，对城市/地块级别的区域，
// 相对误差通常在 0.1% ~ 0.5% 量级(主要来自椭球扁率)。若需要毫米级的
// 大地测量精度，应改用椭球测地面积(见 README 中的 geodesic 方案)。
func AreaCoords(coords []Coords) float64 {
	n := len(coords)
	if n < 3 {
		return 0
	}

	var sum float64
	for i := range n {
		p1 := coords[i]
		p2 := coords[(i+1)%n] // 取模实现闭合，兼容首尾相接或不相接两种输入

		lng1 := p1.Lng * math.Pi / 180
		lng2 := p2.Lng * math.Pi / 180
		lat1 := p1.Lat * math.Pi / 180
		lat2 := p2.Lat * math.Pi / 180

		sum += (lng2 - lng1) * (2 + math.Sin(lat1) + math.Sin(lat2))
	}

	area := sum * authalicRadiusSqr / 2
	return math.Abs(area)
}

// AreaGeoInt 计算由 uint64 geohash 列表(闭合曲线)围成的多边形面积，单位：平方米。
//
// 每个 geohash 都是由 WGS84 坐标经 EncodeInt 得到的整型编码；函数先用
// DecodeInt 还原出每个顶点的经纬度，再复用 AreaCoords 完成计算。
//
// 注意：geohash 是有损网格编码，DecodeInt 得到的是所在网格的一个角点，
// 因此顶点会被量化到网格分辨率上，对面积引入与网格大小相关的误差。
// 若追求精度，请保留原始浮点坐标并直接调用 AreaCoords。
func AreaGeoInt(geos []uint64) float64 {
	if len(geos) < 3 {
		return 0
	}

	coords := make([]Coords, len(geos))
	for i, g := range geos {
		lat, lng := DecodeInt(g)
		coords[i] = Coords{Lat: lat, Lng: lng}
	}
	return AreaCoords(coords)
}
