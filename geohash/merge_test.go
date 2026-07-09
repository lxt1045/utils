package geohash

import (
	"math"
	"testing"
)

// 把一个正方形边界拆成 4 条乱序、方向不一、端点重合的曲线，
// 验证 MergeCurves 能拼回闭合环，且面积与直接构造的一致。
func TestMergeCurves_SquareFromScrambledEdges(t *testing.T) {
	// 正方形四角 (顺时针)
	a := Coords{Lat: 0, Lng: 0}
	b := Coords{Lat: 0, Lng: 1}
	c := Coords{Lat: 1, Lng: 1}
	d := Coords{Lat: 1, Lng: 0}

	// 四条边，故意打乱顺序并反转部分方向，端点共享
	curves := [][]Coords{
		{c, d}, // 上边(反向)
		{a, b}, // 下边
		{d, a}, // 左边(反向)
		{b, c}, // 右边
	}
	want := AreaCoords([]Coords{a, b, c, d})

	t.Run("MergeCurves", func(t *testing.T) {
		ring := MergeCurves(curves, 1.0) // 1m 容差

		// 闭合环应恰好包含 4 个不重复顶点
		if len(ring) != 4 {
			t.Fatalf("expected 4 vertices, got %d: %+v", len(ring), ring)
		}

		got := AreaCoords(ring)
		if rel := math.Abs(got-want) / want; rel > 1e-9 {
			t.Fatalf("merged area=%.4f want=%.4f relErr=%.2e", got, want, rel)
		}
	})
	t.Run("MergeCurves2", func(t *testing.T) {
		ring := MergeCurves2(curves, 1.0) // 1m 容差

		// 闭合环应恰好包含 4 个不重复顶点
		if len(ring) != 4 {
			t.Fatalf("expected 4 vertices, got %d: %+v", len(ring), ring)
		}

		got := AreaCoords(ring)
		if rel := math.Abs(got-want) / want; rel > 1e-9 {
			t.Fatalf("merged area=%.4f want=%.4f relErr=%.2e", got, want, rel)
		}
	})
}

// 衔接处端点存在微小偏差(< tolerance)时应被去重衔接。
func TestMergeCurves_ToleranceDedup(t *testing.T) {
	// 两条曲线，第二条起点比第一条终点偏了约 0.5m
	c1 := []Coords{{Lat: 0, Lng: 0}, {Lat: 0, Lng: 1}}
	// 0.5m ≈ 0.0000045° 纬度
	c2 := []Coords{{Lat: 0.0000045, Lng: 1}, {Lat: 1, Lng: 1}, {Lat: 0, Lng: 0}}

	ring := MergeCurves([][]Coords{c1, c2}, 1.0)
	// c1(2点) + c2(2点) - 1个重合点 = 3
	if len(ring) != 3 {
		t.Fatalf("expected 3 vertices after dedup, got %d: %+v", len(ring), ring)
	}

	ring = MergeCurves2([][]Coords{c1, c2}, 1.0)
	// c1(2点) + c2(2点) - 1个重合点 = 3
	if len(ring) != 3 {
		t.Fatalf("expected 3 vertices after dedup, got %d: %+v", len(ring), ring)
	}
}

func TestMergeCurvesGeoInt_RoundTrip(t *testing.T) {
	corners := []Coords{
		{Lat: 39.90, Lng: 116.30},
		{Lat: 39.90, Lng: 116.50},
		{Lat: 40.00, Lng: 116.50},
		{Lat: 40.00, Lng: 116.30},
	}
	enc := func(c Coords) uint64 { return EncodeInt(c.Lat, c.Lng) }

	curves := [][]uint64{
		{enc(corners[2]), enc(corners[3])},
		{enc(corners[0]), enc(corners[1])},
		{enc(corners[3]), enc(corners[0])},
		{enc(corners[1]), enc(corners[2])},
	}

	ring := MergeCurvesGeoInt(curves, 100.0) // 100m 容差(> 网格尺寸)
	if len(ring) != 4 {
		t.Fatalf("expected 4 vertices, got %d", len(ring))
	}

	got := AreaGeoInt(ring)
	want := AreaCoords(corners)
	if rel := math.Abs(got-want) / want; rel > 1e-3 {
		t.Fatalf("area=%.2f want=%.2f relErr=%.4f", got, want, rel)
	}
}

func TestMergeCurves_Empty(t *testing.T) {
	if r := MergeCurves(nil, 1.0); r != nil {
		t.Fatalf("nil input should return nil, got %+v", r)
	}
	if r := MergeCurves2(nil, 1.0); r != nil {
		t.Fatalf("nil input should return nil, got %+v", r)
	}
}
