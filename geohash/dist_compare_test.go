package geohash

import (
	"math"
	"testing"

	"github.com/paulmach/orb"
	"github.com/paulmach/orb/geo"
	"github.com/tidwall/geodesic"
)

// orbDist 用 paulmach/orb 的 geo.DistanceHaversine 计算两点距离(米)。
// 与本包 DistHaversine 同为 Haversine 公式，差异只来自球半径取值。
func orbDist(a, b Coords) float64 {
	return geo.DistanceHaversine(orb.Point{a.Lng, a.Lat}, orb.Point{b.Lng, b.Lat})
}

// geodesicDist 用 tidwall/geodesic 的椭球测地反算求两点距离(米)。
// 这是 WGS84 椭球上的真值参照(Karney 方法，毫米级)。
func geodesicDist(a, b Coords) float64 {
	var s12 float64
	geodesic.WGS84.Inverse(a.Lat, a.Lng, b.Lat, b.Lng, &s12, nil, nil)
	return s12
}

// TestDistHaversine_CompareWithLibraries 对比 DistHaversine 与 orb、geodesic。
//
// 关键结论(由实测得出)：
//   - DistHaversine vs orb.DistanceHaversine：两者都是 Haversine 公式，但球半径
//     不同——orb 用 WGS84 赤道半径 6378137m，本包用等面积球半径 6371007m。
//     距离 ∝ R，故差异恒为 6378137/6371007-1 ≈ 0.1119%，与距离和位置无关。
//   - DistHaversine vs geodesic(椭球真值)：球近似椭球的固有误差，实测 < 0.4%，
//     且本包(等面积球)通常比 orb(赤道半径)更贴近椭球真值。
func TestDistHaversine_CompareWithLibraries(t *testing.T) {
	cases := []struct {
		name string
		a, b Coords
	}{
		{
			name: "short-city", // 城区内短距离(~200m)
			a:    Coords{Lat: 39.9000, Lng: 116.3000},
			b:    Coords{Lat: 39.9010, Lng: 116.3020},
		},
		{
			name: "beijing-shanghai", // 北京-上海(~1067km)
			a:    Coords{Lat: 39.9042, Lng: 116.4074},
			b:    Coords{Lat: 31.2304, Lng: 121.4737},
		},
		{
			name: "east-west-midlat", // 中纬度纯东西向长距离
			a:    Coords{Lat: 45.0, Lng: 100.0},
			b:    Coords{Lat: 45.0, Lng: 120.0},
		},
		{
			name: "north-south", // 纯南北向(经线)长距离
			a:    Coords{Lat: 10.0, Lng: 50.0},
			b:    Coords{Lat: 40.0, Lng: 50.0},
		},
		{
			name: "high-latitude", // 高纬度长距离
			a:    Coords{Lat: 60.0, Lng: 10.0},
			b:    Coords{Lat: 62.0, Lng: 30.0},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			mine := DistHaversine(tc.a, tc.b)
			orbD := orbDist(tc.a, tc.b)
			geoD := geodesicDist(tc.a, tc.b)

			relOrb := math.Abs(mine-orbD) / geoD
			relGeo := math.Abs(mine-geoD) / geoD

			t.Logf("DistHaversine=%.3f m  orb=%.3f  geodesic(真值)=%.3f", mine, orbD, geoD)
			t.Logf("mine→orb=%.6f%%  mine→真值=%.4f%%  orb→真值=%.4f%%",
				relOrb*100, relGeo*100, math.Abs(orbD-geoD)/geoD*100)

			// 与 orb 的差异源于球半径不同：orb 用 WGS84 赤道半径 6378137m，
			// 本包用等面积球半径 6371007m。距离 ∝ R，故差异恒为
			// 6378137/6371007-1 ≈ 0.1119%，与距离和位置无关。
			if relOrb > 2e-3 {
				t.Errorf("DistHaversine 与 orb 差异过大: relErr=%.6f", relOrb)
			}
			// 与椭球真值的球近似误差应在 0.6% 内
			if relGeo > 6e-3 {
				t.Errorf("DistHaversine 与 geodesic 椭球真值差异过大: relErr=%.6f", relGeo)
			}
		})
	}
}
