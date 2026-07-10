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

// DistHaversineFast 是 DistHaversine 的高性能近似版本：用等距圆柱(equirectangular)
// 投影把球面距离化成平面勾股，运行时只需 1 次 Cos + 1 次 Sqrt，
// 相比精确 Haversine 省掉 2 次 Sin、1 次 Cos、1 次 Asin。
//
// 原理：把经度差按「中点纬度」的余弦缩放成东西向平面距离，纬度差直接作南北向距离，
//
//	x = Δλ·cos(φm)   (φm = 两点纬度中值)
//	y = Δφ
//	d = R·sqrt(x² + y²)
//
// 对换 p1、p2 时 Δλ、Δφ 只变号，平方后不变，故满足对称性。
//
// 精度：平面近似误差随距离增长，约正比于 (d/R)²。经验量级：
//   - < 1000 km：相对误差 < 0.5%
//   - < 2000 km：相对误差 < 2%
//   - < 3500 km：相对误差 < 5%
//
// 更远(洲际、近对跖)会超过 5%，不适用。适合近/中距离的粗筛、排序、阈值判断等
// 对精度不敏感但要极快的场景；需要全球范围精确距离请用 DistHaversine。
func DistHaversineFast(p1, p2 Coords) float64 {
	dLat := (p2.Lat - p1.Lat) * degToRad
	dLng := (p2.Lng - p1.Lng) * degToRad
	// 跨 180° 经线时取最短经差，避免 Δλ 接近 ±360° 造成巨大误差。
	if dLng > math.Pi {
		dLng -= 2 * math.Pi
	} else if dLng < -math.Pi {
		dLng += 2 * math.Pi
	}
	latMean := (p1.Lat + p2.Lat) * 0.5 * degToRad
	x := dLng * math.Cos(latMean)
	return earthRadiusM * math.Sqrt(x*x+dLat*dLat)
}

// piSq = π²，Bhaskara 余弦近似用的常量。
const piSq = math.Pi * math.Pi

// distShortMaxM 是 DistShort 的适用距离上限(米)；超过即视为「远」并饱和返回该值。
const distShortMaxM = 100.0

// distShortMaxAngle 是 100m 对应的球面角(弧度)。距离 = R·角，故角差超过它必 >100m。
const distShortMaxAngle = distShortMaxM / earthRadiusM

// DistShort 为 100m 以内的近距离两点专门优化：在 DistHaversineFast 的等距圆柱投影
// 基础上，把唯一的 math.Cos 调用换成 Bhaskara I 余弦有理近似，去掉了 libm 调用，
// 只剩几次乘加 + 1 次除 + 1 次 Sqrt，比 DistHaversineFast 更快。
//
//	cos(φ) ≈ (π² - 4φ²) / (π² + φ²)   φ∈[-π/2, π/2]
//
// 该近似在整个纬度区间(含两极)相对误差 < 0.2%，且 100m 尺度下投影展平误差可忽略，
// 故总误差远小于 10%。φ 由 lat·degToRad 得到，天然落在 [-π/2, π/2]，无需 clamp。
//
// 超界饱和：本函数只对 ≤100m 有意义，故先用角差快速判界——一旦两点距离必然 >100m，
// 立即返回 distShortMaxM(100.0) 作为「远」哨兵值，省掉 Sqrt。调用方若只需判断
// 「是否在 100m 内」，可直接比较返回值是否 < 100。更远的精确距离请用
// DistHaversineFast 或 DistHaversine。
func DistShort(p1, p2 Coords) float64 {
	dLat := (p2.Lat - p1.Lat) * degToRad
	dLng := (p2.Lng - p1.Lng) * degToRad
	// 跨 180° 经线时取最短经差(近距离仅在 ±180° 附近才触发，分支几乎不命中)。
	if dLng > math.Pi {
		dLng -= 2 * math.Pi
	} else if dLng < -math.Pi {
		dLng += 2 * math.Pi
	}
	// 南北向角差已给出距离下界(R·|dLat|)：仅凭它超界就能廉价判「远」，跳过 cos 近似。
	if dLat > distShortMaxAngle || dLat < -distShortMaxAngle {
		return distShortMaxM
	}
	phi := (p1.Lat + p2.Lat) * 0.5 * degToRad
	phiSq := phi * phi
	cosLat := (piSq - 4*phiSq) / (piSq + phiSq) // Bhaskara I 余弦近似
	x := dLng * cosLat
	// 合成角差平方超过阈值即 >100m，饱和返回，省掉 Sqrt。
	d2 := x*x + dLat*dLat
	if d2 > distShortMaxAngle*distShortMaxAngle {
		return distShortMaxM
	}
	return earthRadiusM * math.Sqrt(d2)
}
