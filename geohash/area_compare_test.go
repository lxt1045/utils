package geohash

import (
	"math"
	"testing"

	"github.com/paulmach/orb"
	"github.com/paulmach/orb/geo"
	"github.com/tidwall/geodesic"
)

// orbArea 用 paulmach/orb 的 geo.Area 计算面积(平方米)。
// orb 内部同样是球面角盈公式，预期与 AreaCoords 几乎完全一致。
func orbArea(ring []Coords) float64 {
	pts := make([]orb.Point, 0, len(ring)+1)
	for _, c := range ring {
		pts = append(pts, orb.Point{c.Lng, c.Lat}) // orb.Point 是 {Lon, Lat}
	}
	// orb.Ring 要求首尾闭合
	pts = append(pts, pts[0])
	return geo.Area(orb.Polygon{orb.Ring(pts)})
}

// geodesicArea 用 tidwall/geodesic 的椭球测地算法计算面积(平方米)。
// 这是 WGS84 椭球上的真值参照(Karney 方法，毫米级)。
func geodesicArea(ring []Coords) float64 {
	poly := geodesic.WGS84.PolygonInit(false)
	for _, c := range ring {
		poly.AddPoint(c.Lat, c.Lng)
	}
	var area, perimeter float64
	poly.Compute(false, true, &area, &perimeter)
	return math.Abs(area)
}

// TestArea_CompareWithLibraries 对比 AreaCoords 与 orb、geodesic 的结果。
//
// 关键结论(由实测得出)：
//   - AreaCoords vs orb.geo.Area：两者都是球面角盈公式，但球半径不同——
//     orb 用 WGS84 赤道半径 6378137m，本包用等面积球半径 6371007m。
//     面积 ∝ R²，故差异恒为 (6378137/6371007)²-1 ≈ 0.224%，与顶点位置无关。
//   - AreaCoords vs geodesic(椭球真值)：球近似椭球的固有误差，随纬度增大，
//     实测最大约 0.55%(60°N)。因等面积球半径专为面积近似设计，本包实现
//     反而比 orb(赤道半径)更贴近椭球真值。
func TestArea_CompareWithLibraries(t *testing.T) {
	cases := []struct {
		name string
		ring []Coords
	}{
		{
			name: "beijing-block", // 北京城区一小块
			ring: []Coords{
				{Lat: 39.900, Lng: 116.300},
				{Lat: 39.900, Lng: 116.500},
				{Lat: 40.000, Lng: 116.500},
				{Lat: 40.000, Lng: 116.300},
			},
		},
		{
			name: "equator-1deg", // 赤道附近 1°×1°
			ring: []Coords{
				{Lat: 0, Lng: 0},
				{Lat: 0, Lng: 1},
				{Lat: 1, Lng: 1},
				{Lat: 1, Lng: 0},
			},
		},
		{
			name: "high-latitude", // 高纬度(60°N)方块
			ring: []Coords{
				{Lat: 60.0, Lng: 10.0},
				{Lat: 60.0, Lng: 12.0},
				{Lat: 61.0, Lng: 12.0},
				{Lat: 61.0, Lng: 10.0},
			},
		},
		{
			name: "large-region", // 跨度较大的区域
			ring: []Coords{
				{Lat: 30.0, Lng: 100.0},
				{Lat: 30.0, Lng: 120.0},
				{Lat: 45.0, Lng: 120.0},
				{Lat: 45.0, Lng: 100.0},
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			mine := AreaCoords(tc.ring)
			orbA := orbArea(tc.ring)
			geoA := geodesicArea(tc.ring)

			relOrb := math.Abs(mine-orbA) / geoA    // AreaCoords vs orb
			relGeo := math.Abs(mine-geoA) / geoA    // AreaCoords vs 椭球真值
			relOrbGeo := math.Abs(orbA-geoA) / geoA // orb vs 椭球真值

			t.Logf("AreaCoords=%.2f m^2  orb=%.2f  geodesic(真值)=%.2f", mine, orbA, geoA)
			t.Logf("AreaCoords→orb=%.4f%%  AreaCoords→真值=%.4f%%  orb→真值=%.4f%%",
				relOrb*100, relGeo*100, relOrbGeo*100)

			// 与 orb 的差异源于球半径不同(等面积球 vs 赤道半径)，恒约 0.224%
			if relOrb > 3e-3 {
				t.Errorf("AreaCoords 与 orb 差异过大: relErr=%.6f", relOrb)
			}
			// 本包(等面积球)相对椭球真值的球近似误差应在 0.6% 内(高纬度最大)
			if relGeo > 6e-3 {
				t.Errorf("AreaCoords 与 geodesic 椭球真值差异过大: relErr=%.6f", relGeo)
			}
			// orb(赤道半径)相对椭球真值误差更大，应在 0.8% 内
			if relOrbGeo > 8e-3 {
				t.Errorf("orb 与 geodesic 椭球真值差异过大: relErr=%.6f", relOrbGeo)
			}
		})
	}
}

// TestAreaGeoInt_CompareWithGeodesic 验证 geohash 整型版与椭球真值也在可接受范围内。
func TestAreaGeoInt_CompareWithGeodesic(t *testing.T) {
	ring := []Coords{
		{Lat: 39.900, Lng: 116.300},
		{Lat: 39.900, Lng: 116.500},
		{Lat: 40.000, Lng: 116.500},
		{Lat: 40.000, Lng: 116.300},
	}
	geos := make([]uint64, len(ring))
	for i, c := range ring {
		geos[i] = EncodeInt(c.Lat, c.Lng)
	}

	got := AreaGeoInt(geos)
	want := geodesicArea(ring)
	rel := math.Abs(got-want) / want
	t.Logf("AreaGeoInt=%.2f m^2  geodesic(真值)=%.2f  relErr=%.4f%%", got, want, rel*100)
	if rel > 5e-3 {
		t.Errorf("AreaGeoInt 与 geodesic 真值差异过大: relErr=%.6f", rel)
	}
}
