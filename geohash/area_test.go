package geohash

import (
	"math"
	"testing"
)

// 用一个跨赤道、跨本初子午线的 1°×1° 方块做基准。
// 该方块在赤道附近，理论面积约为 (地球周长/360)^2。
func TestAreaCoords_OneDegreeSquare(t *testing.T) {
	square := []Coords{
		{Lat: 0, Lng: 0},
		{Lat: 0, Lng: 1},
		{Lat: 1, Lng: 1},
		{Lat: 1, Lng: 0},
	}

	got := AreaCoords(square)

	// 1° 经线弧长 ≈ 111195 m (等面积球)，赤道附近 1°×1° ≈ 111195^2
	want := 111195.0 * 111195.0
	relErr := math.Abs(got-want) / want
	if relErr > 0.01 {
		t.Fatalf("area=%.2f m^2, want≈%.2f m^2, relErr=%.4f", got, want, relErr)
	}
}

// 顶点顺序反转(顺/逆时针)不应改变面积，且结果非负。
func TestAreaCoords_WindingOrder(t *testing.T) {
	cw := []Coords{
		{Lat: 0, Lng: 0},
		{Lat: 0, Lng: 1},
		{Lat: 1, Lng: 1},
		{Lat: 1, Lng: 0},
	}
	ccw := []Coords{
		{Lat: 1, Lng: 0},
		{Lat: 1, Lng: 1},
		{Lat: 0, Lng: 1},
		{Lat: 0, Lng: 0},
	}

	a1 := AreaCoords(cw)
	a2 := AreaCoords(ccw)
	if math.Abs(a1-a2) > 1e-6 {
		t.Fatalf("winding order changed area: cw=%.6f ccw=%.6f", a1, a2)
	}
	if a1 < 0 || a2 < 0 {
		t.Fatalf("area must be non-negative: %.6f", a1)
	}
}

// 显式首尾相接的输入与不相接的输入应得到相同结果。
func TestAreaCoords_ClosedVsOpenRing(t *testing.T) {
	open := []Coords{
		{Lat: 0, Lng: 0},
		{Lat: 0, Lng: 1},
		{Lat: 1, Lng: 1},
		{Lat: 1, Lng: 0},
	}
	closed := append(append([]Coords{}, open...), open[0])

	if a, b := AreaCoords(open), AreaCoords(closed); math.Abs(a-b) > 1e-6 {
		t.Fatalf("open=%.6f closed=%.6f", a, b)
	}
}

func TestAreaCoords_Degenerate(t *testing.T) {
	if a := AreaCoords(nil); a != 0 {
		t.Fatalf("nil should be 0, got %f", a)
	}
	if a := AreaCoords([]Coords{{Lat: 1, Lng: 1}, {Lat: 2, Lng: 2}}); a != 0 {
		t.Fatalf("2 points should be 0, got %f", a)
	}
}

// geohash 版本应与浮点版本接近(误差来自网格量化)。
func TestAreaGeoInt_MatchesCoords(t *testing.T) {
	coords := []Coords{
		{Lat: 39.900, Lng: 116.300},
		{Lat: 39.900, Lng: 116.500},
		{Lat: 40.000, Lng: 116.500},
		{Lat: 40.000, Lng: 116.300},
	}

	geos := make([]uint64, len(coords))
	for i, c := range coords {
		geos[i] = EncodeInt(c.Lat, c.Lng)
	}

	want := AreaCoords(coords)
	got := AreaGeoInt(geos)

	// 32bit/轴 分辨率下量化误差极小
	relErr := math.Abs(got-want) / want
	if relErr > 1e-3 {
		t.Fatalf("geo area=%.2f, coord area=%.2f, relErr=%.6f", got, want, relErr)
	}
}
