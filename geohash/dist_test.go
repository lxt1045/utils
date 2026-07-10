package geohash

import (
	"math"
	"math/rand"
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

// DistHaversineFast 对称性：交换两点，距离不变。
func TestDistHaversineFast_Symmetric(t *testing.T) {
	a := Coords{Lat: 39.90, Lng: 116.30}
	b := Coords{Lat: 40.50, Lng: 117.10}
	if d1, d2 := DistHaversineFast(a, b), DistHaversineFast(b, a); math.Abs(d1-d2) > 1e-9 {
		t.Fatalf("not symmetric: %.9f vs %.9f", d1, d2)
	}
}

// DistHaversineFast 的适用域：|纬度| ≤ 70° 且距离 ≤ 1000km 时，相对精确 Haversine
// 的误差应 ≤ 5%。此域覆盖了几乎所有实际的近/中距离查询。
//
// 说明：等距圆柱投影把球面拍平，误差随「纬度」和「距离」增长；在两极附近(|lat|>70°)
// 经线剧烈收敛，即便两点很近，把 Δλ 按 cos 缩放也会严重失真——这是投影本身的极限，
// 非本函数缺陷。故适用域排除极区；需要全球精确距离请用 DistHaversine。
func TestDistHaversineFast_ErrorBudget(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	const (
		n        = 500000
		maxLatOK = 70.0
		maxDist  = 1000e3
	)
	var maxRel float64
	for range n {
		a := Coords{Lat: rng.Float64()*2*maxLatOK - maxLatOK, Lng: rng.Float64()*360 - 180}
		b := Coords{Lat: rng.Float64()*2*maxLatOK - maxLatOK, Lng: rng.Float64()*360 - 180}
		exact := DistHaversine(a, b)
		if exact < 1000 || exact > maxDist {
			continue
		}
		rel := math.Abs(exact-DistHaversineFast(a, b)) / exact
		if rel > maxRel {
			maxRel = rel
		}
	}
	t.Logf("|lat|≤%.0f°, dist≤%.0fkm: 最大相对误差=%.4f", maxLatOK, maxDist/1000, maxRel)
	if maxRel > 0.05 {
		t.Errorf("最大相对误差 %.4f 超过 5%% 预算", maxRel)
	}
}

// 跨 180° 经线：北京(116E) 到 檀香山(157W) 应取最短经差，不因 Δλ≈273° 而爆炸。
func TestDistHaversineFast_Antimeridian(t *testing.T) {
	a := Coords{Lat: 21, Lng: 157}
	b := Coords{Lat: 21, Lng: -157}
	got := DistHaversineFast(a, b)
	exact := DistHaversine(a, b)
	if rel := math.Abs(got-exact) / exact; rel > 0.05 {
		t.Fatalf("antimeridian fast=%.1f exact=%.1f rel=%.4f", got, exact, rel)
	}
}

// DistShort 对称性：交换两点，距离不变。
func TestDistShort_Symmetric(t *testing.T) {
	a := Coords{Lat: 39.90000, Lng: 116.30000}
	b := Coords{Lat: 39.90050, Lng: 116.30060}
	if d1, d2 := DistShort(a, b), DistShort(b, a); math.Abs(d1-d2) > 1e-9 {
		t.Fatalf("not symmetric: %.9f vs %.9f", d1, d2)
	}
}

// DistShort 在 100m 以内、全纬度范围(含两极)相对精确 Haversine 的误差应 < 10%。
func TestDistShort_ErrorBudget(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	const n = 500000
	var maxRel float64
	for range n {
		lat := rng.Float64()*180 - 90
		lng := rng.Float64()*360 - 180
		a := Coords{Lat: lat, Lng: lng}
		// 在 a 周围 ~±100m 内随机取 b：1m≈1e-5°，取 ±0.001° 覆盖到约 110m。
		b := Coords{Lat: lat + (rng.Float64()*2-1)*0.001, Lng: lng + (rng.Float64()*2-1)*0.001}
		exact := DistHaversine(a, b)
		if exact < 1 || exact > 100 { // 只考察 1m~100m 区间
			continue
		}
		if rel := math.Abs(exact-DistShort(a, b)) / exact; rel > maxRel {
			maxRel = rel
		}
	}
	t.Logf("dist≤100m(全纬度): 最大相对误差=%.4f", maxRel)
	if maxRel > 0.10 {
		t.Errorf("最大相对误差 %.4f 超过 10%% 预算", maxRel)
	}
}

// BenchmarkDistHaversine 对比精确版、快速近似版与近距离版的耗时。
func BenchmarkDistHaversine(b *testing.B) {
	p1 := Coords{Lat: 39.9042, Lng: 116.4074}
	p2 := Coords{Lat: 31.2304, Lng: 121.4737}
	b.Run("Haversine", func(b *testing.B) {
		for range b.N {
			_ = DistHaversine(p1, p2)
		}
	})
	b.Run("HaversineFast", func(b *testing.B) {
		for range b.N {
			_ = DistHaversineFast(p1, p2)
		}
	})
	// 近距离版用相隔约 100m 的两点，贴合其适用场景。
	s1 := Coords{Lat: 39.90000, Lng: 116.40000}
	s2 := Coords{Lat: 39.90050, Lng: 116.40060}
	b.Run("Short", func(b *testing.B) {
		for range b.N {
			_ = DistShort(s1, s2)
		}
	})
}
