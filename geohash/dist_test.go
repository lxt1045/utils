package geohash

import (
	"math"
	"testing"
)

// DistHaversine 必须对称：Dist(a,b) == DistHaversine(b,a)。
// 修正前，经度项只用 p1.Lat 求 cos，两点纬度不同时不对称。
func TestDist_Symmetric(t *testing.T) {
	a := Coords{Lat: 39.90, Lng: 116.30}
	b := Coords{Lat: 40.50, Lng: 117.10}

	d1 := DistHaversine(a, b)
	d2 := DistHaversine(b, a)
	if math.Abs(d1-d2) > 1e-9 {
		t.Fatalf("DistHaversine not symmetric: DistHaversine(a,b)=%.9f DistHaversine(b,a)=%.9f", d1, d2)
	}
}

// 纯经线方向(同经度)1°的距离 —— Haversine 球面大圆弧长。
func TestDist_MeridianDegree(t *testing.T) {
	a := Coords{Lat: 39.0, Lng: 116.0}
	b := Coords{Lat: 40.0, Lng: 116.0}

	got := DistHaversine(a, b) / 1000 // m → km
	// 1° 经线弧长 = R·(π/180)，R = earthRadiusM
	want := earthRadiusM * (math.Pi / 180) / 1000 // m → km
	if relErr := math.Abs(got-want) / want; relErr > 1e-9 {
		t.Fatalf("meridian 1° = %.6f km, want %.6f km", got, want)
	}
}

// Haversine 对称性。
func TestDistHaversine_Symmetric(t *testing.T) {
	a := Coords{Lat: 39.90, Lng: 116.30}
	b := Coords{Lat: 40.50, Lng: 117.10}
	if d1, d2 := DistHaversine(a, b), DistHaversine(b, a); math.Abs(d1-d2) > 1e-9 {
		t.Fatalf("not symmetric: %.9f vs %.9f", d1, d2)
	}
}

// 同点距离为 0。
func TestDistHaversine_ZeroAndAntipodal(t *testing.T) {
	p := Coords{Lat: 30, Lng: 120}
	if d := DistHaversine(p, p); d != 0 {
		t.Fatalf("same point should be 0, got %.6f", d)
	}
	// 对跖点距离应约等于半个大圆周长 π·R
	anti := Coords{Lat: -30, Lng: -60}
	got := DistHaversine(p, anti)
	want := math.Pi * earthRadiusM
	if rel := math.Abs(got-want) / want; rel > 1e-9 {
		t.Fatalf("antipodal = %.2f m, want %.2f m", got, want)
	}
}

// 已知距离基准：北京 (39.9042,116.4074) 到 上海 (31.2304,121.4737)
// 大圆距离约 1067 km。
func TestDistHaversine_KnownCity(t *testing.T) {
	beijing := Coords{Lat: 39.9042, Lng: 116.4074}
	shanghai := Coords{Lat: 31.2304, Lng: 121.4737}
	got := DistHaversine(beijing, shanghai) / 1000 // km
	if got < 1050 || got > 1080 {
		t.Fatalf("Beijing-Shanghai = %.2f km, expected ~1067 km", got)
	}
}

// 高纬度对比：等距圆柱近似(DistHaversine)在高纬会偏离，Haversine 更准。
// 此处只验证纯经度差在 60°N 的距离与 cos(60°)·赤道1°弧长 一致(球模型)。
func TestDistHaversine_HighLatitudeLongitude(t *testing.T) {
	a := Coords{Lat: 60, Lng: 10}
	b := Coords{Lat: 60, Lng: 11}
	got := DistHaversine(a, b)
	// 60°N 纬线圈上 1° 经度 ≈ cos(60°) · (2πR/360)
	want := math.Cos(60*degToRad) * (2 * math.Pi * earthRadiusM / 360)
	if rel := math.Abs(got-want) / want; rel > 1e-3 {
		t.Fatalf("60N 1°lng = %.2f m, want ≈ %.2f m (rel=%.4f)", got, want, rel)
	}
}
