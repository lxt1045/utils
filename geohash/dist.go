package geohash

import "math"

// 本文件提供精度优先的球面距离实现：Haversine 公式。
//
// 相比 geo.go 中的 Dist/Dist2(等距圆柱近似,仅适合中低纬度、短距离),
// Haversine 在全球范围(含高纬、跨 180° 经线、对跖点)都稳定，
// 数值上避免了近对跖点时反余弦公式的精度塌陷。
// 地球按等面积球(authalicRadius)近似，对椭球真值相对误差约 0.1%~0.3%。

// 地球平均半径(米)。用等面积球半径,与本包 AreaCoords 保持同一球模型。
const earthRadiusM = authalicRadius

const degToRad = math.Pi / 180

// DistHaversine 返回两 WGS84 坐标点间的大圆距离，单位：米。
//
// 采用 Haversine 公式，全球范围数值稳定，满足对称性 DistHaversine(a,b)==DistHaversine(b,a)。
// 相比等距圆柱近似的 Dist，本函数在高纬度与长距离下精度显著更好。
//
// 优化点：
//   - 只调用 2 次 Sin、2 次 Cos、1 次 Asin、1 次 Sqrt，无多余三角函数；
//   - 纬度余弦用 math.Cos 直接求，避免额外的 lat 转换往返；
//   - 纯标量运算，可被编译器内联，零堆分配。
func DistHaversine(p1, p2 Coords) float64 {
	lat1 := p1.Lat * degToRad
	lat2 := p2.Lat * degToRad
	dLat := (p2.Lat - p1.Lat) * degToRad
	dLng := (p2.Lng - p1.Lng) * degToRad

	sinLat := math.Sin(dLat / 2)
	sinLng := math.Sin(dLng / 2)

	// a = sin²(Δφ/2) + cosφ1·cosφ2·sin²(Δλ/2)
	a := sinLat*sinLat + math.Cos(lat1)*math.Cos(lat2)*sinLng*sinLng
	// 用 Asin 形式(而非 Atan2)：a 已被夹在 [0,1]，少一次除法与分支。
	if a > 1 {
		a = 1 // 浮点误差保护，防止 Sqrt 出现极微小负判
	}
	return 2 * earthRadiusM * math.Asin(math.Sqrt(a))
}
