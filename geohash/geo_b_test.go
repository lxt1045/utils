package geohash

import (
	"testing"

	codefor_geohash "github.com/Codefor/geohash"
	tomi_hiltunen_geohash "github.com/TomiHiltunen/geohash-golang"
	broadygeohash "github.com/broady/gogeohash" //nolint:misspell
	fanixk_geohash "github.com/fanixk/geohash"
	gansidui_geohash "github.com/gansidui/geohash"
	mmcloughlin_geohash "github.com/mmcloughlin/geohash"
	pierrre_geohash "github.com/pierrre/geohash"
)

func BenchmarkNeighborsInt(b *testing.B) {
	x, y := 39.92324, 116.3906
	bits := 50
	geo := EncodeInt(x, y)
	b.Logf("%b", geo)
	b.Logf("%s", Geo2Str(geo, 12))
	b.Run("my", func(b *testing.B) {
		for i := 0; i < b.N; i++ { //use b.N for looping
			NeighborsInt(geo, uint(bits))
		}
	})
	b.Run("mmcloughlin_geohash", func(b *testing.B) {
		for i := 0; i < b.N; i++ { //use b.N for looping
			NeighborsInt2(geo, uint(bits))
		}
	})

	geo2 := geo >> (64 - bits)
	b.Run("mmcloughlin_geohash.NeighborsIntWithPrecision", func(b *testing.B) {
		for i := 0; i < b.N; i++ { //use b.N for looping
			mmcloughlin_geohash.NeighborsIntWithPrecision(geo2, uint(bits))
		}
	})
}
func BenchmarkEncode2(b *testing.B) {
	x, y := 39.92324, 116.3906
	b.Run("my", func(b *testing.B) {
		for i := 0; i < b.N; i++ { //use b.N for looping
			EncodeInt(x, y)
		}
	})
	b.Run("mmcloughlin_geohash", func(b *testing.B) {
		for i := 0; i < b.N; i++ { //use b.N for looping
			mmcloughlin_geohash.EncodeInt(x, y)
		}
	})
	b.Run("mmcloughlin_geohash1x", func(b *testing.B) {
		for i := 0; i < b.N; i++ { //use b.N for looping
			mmcloughlin_geohash.EncodeIntx(x, y)
		}
	})
	b.Run("gansidui_geohash", func(b *testing.B) {
		for i := 0; i < b.N; i++ { //use b.N for looping
			gansidui_geohash.Encode(x, y, 12)
		}
	})

	b.Run("pierrre_geohash", func(b *testing.B) {
		for i := 0; i < b.N; i++ { //use b.N for looping
			pierrre_geohash.Encode(x, y, 12)
		}
	})
	b.Run("codefor_geohash", func(b *testing.B) {
		for i := 0; i < b.N; i++ { //use b.N for looping
			codefor_geohash.Encode(x, y)
		}
	})
	b.Run("tomi_hiltunen_geohash", func(b *testing.B) {
		for i := 0; i < b.N; i++ { //use b.N for looping
			tomi_hiltunen_geohash.EncodeWithPrecision(x, y, 12)
		}
	})

	b.Run("broadygeohash", func(b *testing.B) {
		for i := 0; i < b.N; i++ { //use b.N for looping
			broadygeohash.Encode(x, y)
		}
	})
	b.Run("fanixk_geohash", func(b *testing.B) {
		for i := 0; i < b.N; i++ { //use b.N for looping
			fanixk_geohash.PrecisionEncode(x, y, 12)
		}
	})
}

func BenchmarkEncode3(b *testing.B) {
	lat, lng := 39.92324, 116.3906
	b.Run("EncodeInt", func(b *testing.B) {
		for i := 0; i < b.N; i++ { //use b.N for looping
			EncodeInt(lat, lng)
		}
	})
	b.Run("EncodeInt2", func(b *testing.B) {
		for i := 0; i < b.N; i++ { //use b.N for looping
			EncodeInt2(lat, lng)
		}
	})

	x, y := EncodeCoords(lat, lng)

	b.Run("interleave64", func(b *testing.B) {
		for i := 0; i < b.N; i++ { //use b.N for looping
			interleave64(x, y)
		}
	})
	b.Run("interleave64_2", func(b *testing.B) {
		for i := 0; i < b.N; i++ { //use b.N for looping
			interleave64_2(x, y)
		}
	})
}

func Benchmark_pdep64(b *testing.B) {
	src := uint64(0b111111111111)
	mask := uint64(0x5555555555555555) // 交替位掩码

	result := pdep64(src, mask)
	for i := 0; i < b.N; i++ { //use b.N for looping

		result = pdep64(src, mask)
	}
	b.Log(result)
}

// *
func BenchmarkEncodeMy(b *testing.B) {
	for i := 0; i < b.N; i++ { //use b.N for looping
		EncodeInt(39.92324, 116.3906)
	}
}
func BenchmarkEncode(b *testing.B) {
	for i := 0; i < b.N; i++ { //use b.N for looping
		encode(39.92324, 116.3906, 12)
	}
}

func BenchmarkGeoMerge(b *testing.B) {
	for i := 0; i < b.N; i++ { //use b.N for looping
		interleave64(23651, 26978)
	}
}

// BenchmarkDist-4   	50000000	        28.6 ns/op	       0 B/op	       0 allocs/op
// BenchmarkDist-4   	50000000	        27.6 ns/op	       0 B/op	       0 allocs/op
func BenchmarkDist(b *testing.B) {
	for i := 0; i < b.N; i++ { //use b.N for looping
		Dist(Coords{116.3906, 39.92324}, Coords{118.3906, 38.92324})
	}
}

// */
// BenchmarkDist2-4   	50000000	        25.5 ns/op	       0 B/op	       0 allocs/op
func BenchmarkDist2(b *testing.B) {
	c1, c2 := Coords{116.3906, 39.92324}, Coords{118.3906, 38.92324}
	for i := 0; i < b.N; i++ { //use b.N for looping
		Dist2(c1, c2)
	}
}
