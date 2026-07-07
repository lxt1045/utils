package geohash

import (
	"bytes"
	"math"
	"testing"

	codefor_geohash "github.com/Codefor/geohash"
	tomi_hiltunen_geohash "github.com/TomiHiltunen/geohash-golang"
	broadygeohash "github.com/broady/gogeohash" //nolint:misspell
	fanixk_geohash "github.com/fanixk/geohash"
	gansidui_geohash "github.com/gansidui/geohash"
	mmcloughlin_geohash "github.com/mmcloughlin/geohash"
	pierrre_geohash "github.com/pierrre/geohash"
	"golang.org/x/sys/cpu"
)

func TestNeighborsInt(t *testing.T) {
	x, y := 39.92324, 116.3906
	bits := 50

	fEq := func(xs, ys []uint64) (ok bool) {
		if len(xs) != len(ys) {
			return
		}
		for i := range xs {
			if xs[i] != ys[i] {
				return
			}
		}

		return true
	}
	for i := range 1000 {
		x += 0.0001 * float64(i)
		y += 0.0001 * float64(i)
		geo := EncodeInt(x, y)
		neighbors := NeighborsInt(geo, uint(bits))
		neighbors1 := NeighborsInt2(geo, uint(bits))
		neighbors2 := mmcloughlin_geohash.NeighborsIntWithPrecision(geo>>(64-bits), uint(bits))

		for i, n := range neighbors2 {
			neighbors2[i] = n << (64 - bits)
			// t.Logf("%b", n)
		}

		if !fEq(neighbors, neighbors1) {
			t.Fatal("neighbors!=neighbors1")
		}
		if !fEq(neighbors, neighbors2) {
			t.Fatal("neighbors!=neighbors2")
		}
	}

}

func TestNeighborsInt1(t *testing.T) {
	x, y := 39.92324, 116.3906
	bits := 50

	t.Run("NeighborsInt", func(t *testing.T) {
		geo := EncodeInt(x, y)
		t.Logf("%b", geo)
		t.Logf("%s", Geo2Str(geo, 12))

		neighbors := NeighborsInt(geo, uint(bits))
		t.Log("neighbors:")
		for _, n := range neighbors {
			t.Logf("%b", n)
			// t.Logf("%s", Geo2Str(n, 12))
		}
	})
	t.Run("NeighborsInt2", func(t *testing.T) {
		geo := EncodeInt(x, y)
		t.Logf("%b", geo)
		t.Logf("%s", Geo2Str(geo, 12))

		neighbors := NeighborsInt2(geo, uint(bits))
		t.Log("neighbors:")
		for _, n := range neighbors {
			t.Logf("%b", n)
			// t.Logf("%s", Geo2Str(n, 12))
		}
	})

	t.Run("NeighborsIntWithPrecision", func(t *testing.T) {
		geo := mmcloughlin_geohash.EncodeInt(x, y)
		t.Logf("%b", geo)

		neighbors := mmcloughlin_geohash.NeighborsIntWithPrecision(geo>>(64-bits), uint(bits))
		t.Log("neighbors:")
		for _, n := range neighbors {
			t.Logf("%b", n<<(64-bits))
			// t.Logf("%b", n)
		}
	})
	t.Run("NeighborsIntWithPrecision", func(t *testing.T) {
		geo := mmcloughlin_geohash.EncodeInt(x, y)
		t.Logf("%b", geo)

		neighbors := mmcloughlin_geohash.Neighbors("")
		t.Log("neighbors:")
		for _, n := range neighbors {
			// t.Logf("%b", n<<(64-bits))
			t.Logf("%s", n)
		}
	})
}

func TestAll(t *testing.T) {
	x, y := 39.92324, 116.3906
	t.Run("interleave64", func(t *testing.T) {
		a, b := EncodeCoords(x, y)
		c, d := deinterleave64(interleave64(EncodeCoords(x, y)))
		if a != 3100089259 || b != 3536077594 || a != c || b != d {
			t.Fatal("interleave64")
		}
	})

	t.Run("EncodeInt", func(t *testing.T) {
		if EncodeInt(x, y) != 0b1110011101001000111100000011010101100001010011110100011011001101 {
			t.Fatal("interleave64")
		}
		a, b := DecodeInt(EncodeInt(x, y))
		if math.Abs(a-x) > 0.0000001 || math.Abs(b-y) > 0.0000001 {
			t.Fatalf("a:%f, b:%f", a, b)
		}
	})

	t.Run("EncodeCoords", func(t *testing.T) {
		c, d := EncodeCoords(x, y)
		if c != 3100089259 || d != 3536077594 {
			t.Fatal("EncodeCoords")
		}
		a, b := DecodeCoords(EncodeCoords(x, y))
		if math.Abs(a-x) > 0.0000001 || math.Abs(b-y) > 0.0000001 {
			t.Fatalf("a:%f, b:%f", a, b)
		}
		c, d = EncodeCoords(DecodeCoords(EncodeCoords(x, y)))
		if c != 3100089259 || d != 3536077594 {
			t.Fatal("EncodeCoords")
		}
	})

	t.Run("Encode", func(t *testing.T) {
		if Encode(x, y) != "wx4g0ec19x3d" {
			t.Fatal("Encode")
		}
		if 0b1110011101001000111100000011010101100001010011110100011011000000 != Str2Geo(Encode(x, y)) {
			t.Fatal("Encode")
		}
		a, b := Decode(Encode(x, y))
		if math.Abs(a-x) > 0.000001 || math.Abs(b-y) > 0.000001 {
			t.Fatalf("a:%f, b:%f", a, b)
		}
	})

	t.Run("test", func(t *testing.T) {
		t.Log("EncodeInt:")
		t.Logf("%b", EncodeInt(x, y))
		t.Logf("%b", mmcloughlin_geohash.EncodeInt(x, y))
		t.Log(DecodeInt(EncodeInt(x, y)))
		t.Log(mmcloughlin_geohash.DecodeInt(EncodeInt(x, y)))

		t.Log("interleave64:")
		t.Log(EncodeCoords(x, y))
		t.Log(deinterleave64(interleave64(EncodeCoords(x, y))))

		t.Log("EncodeCoords:")
		t.Log(EncodeCoords(x, y))
		t.Log(DecodeCoords(EncodeCoords(x, y)))
		t.Log(EncodeCoords(DecodeCoords(EncodeCoords(x, y))))

		//
		t.Log("Encode:")
		t.Log(Encode(x, y))
		t.Logf("%b", Str2Geo(Encode(x, y)))
		x1, y1 := Decode(Encode(x, y))
		t.Logf("%f, %f", x1, y1)
	})

}

func TestEncode2(t *testing.T) {
	x, y := 39.92324, 116.3906
	xx := mmcloughlin_geohash.EncodeInt(x, y)
	t.Errorf("%b", EncodeInt(x, y))
	t.Errorf("%b", xx)
	t.Error(Encode(x, y))
	t.Errorf("%b", Str2Geo(Encode(x, y)))
	x1, y1 := Decode(Encode(x, y))
	t.Errorf("%f, %f", x1, y1)
	t.Error(Encode(90, 180))
	// t.Error(Decode(Encode(90, 180)))
	t.Error(Geo2Str(xx, 12))
	t.Error(mmcloughlin_geohash.Encode(x, y))
	t.Error(mmcloughlin_geohash.Encode(90, 180))
	t.Error(gansidui_geohash.Encode(x, y, 12))
	t.Error(pierrre_geohash.Encode(x, y, 12))
	t.Error(codefor_geohash.Encode(x, y))
	t.Error(tomi_hiltunen_geohash.EncodeWithPrecision(x, y, 12))
	t.Error(broadygeohash.Encode(x, y))
	t.Error(fanixk_geohash.PrecisionEncode(x, y, 12))
	t.Error(fanixk_geohash.PrecisionEncode(90, 180, 12))
}

func TestEncode3(t *testing.T) {
	x, y := 39.92324, 116.3906
	t.Errorf("%b", EncodeInt(x, y))
	t.Errorf("%b", mmcloughlin_geohash.EncodeInt(x, y))
	t.Errorf("%b", mmcloughlin_geohash.EncodeIntx(x, y))
}

func TestEncode5(t *testing.T) {
	if cpu.X86.HasBMI2 {
		println("CPU supports BMI2 (PDEP/PEXT)")

		src := uint64(0b111111111111)
		mask := uint64(0x5555555555555555) // 交替位掩码
		result := pdep64(src, mask)
		// result 的二进制将是 0b0101010101010101
		t.Logf("%b", result)
	} else {
		println("CPU does NOT support BMI2, using fallback")
	}
}

func TestEncode4(t *testing.T) {
	var exp232 = math.Exp2(32)
	t.Errorf("exp232:%f", exp232)
	t.Errorf(" exp32:%d", exp32)

	encodeRange := func(x, r float64) uint32 {
		p := (x + r) / (2 * r)
		return uint32(p * exp232)
	}
	decodeRange := func(X uint32, r float64) float64 {
		p := float64(X) / exp232
		x := 2*r*p - r
		return x
	}

	// Spread out the 32 bits of x into 64 bits, where the bits of x occupy even
	// bit positions.
	spread := func(x uint32) uint64 {
		X := uint64(x)
		X = (X | (X << 16)) & 0x0000ffff0000ffff
		X = (X | (X << 8)) & 0x00ff00ff00ff00ff
		X = (X | (X << 4)) & 0x0f0f0f0f0f0f0f0f
		X = (X | (X << 2)) & 0x3333333333333333
		X = (X | (X << 1)) & 0x5555555555555555
		return X
	}
	// Interleave the bits of x and y. In the result, x and y occupy even and odd
	// bitlevels, respectively.
	interleave := func(x, y uint32) uint64 {
		return spread(x) | (spread(y) << 1)
	}

	encodeInt := func(lat, lng float64) uint64 {
		latInt := encodeRange(lat, 90)
		lngInt := encodeRange(lng, 180)
		return interleave(latInt, lngInt)
	}
	_ = encodeInt

	x, y := 39.92324, 116.3906
	a, b := EncodeCoords(x, y)
	x1, y1 := DecodeCoords(a, b)
	t.Errorf("%b", encodeRange(x, 90))
	t.Errorf("%b", a)
	t.Errorf("%f", decodeRange(encodeRange(x, 90), 90))
	t.Errorf("%f,%f", x1, y1)
	t.Errorf("%b", encodeRange(y, 180))
	t.Errorf("%b", b)
}

func TestEncode6(t *testing.T) {
	x, y := 39.92324, 116.3906
	a := EncodeInt(x, y)
	b := EncodeInt2(x, y)
	t.Errorf("%b", a)
	t.Errorf("%b", b)
}

var X [][2]float64

func init() {
	const N = 1000 + 7
	X = make([][2]float64, 0, N)
	X = append(X, [2]float64{39.92324, 116.3906})
	for i := 1; i < N; i++ {
		x := X[i-1][0] + 0.00123
		y := X[i-1][1] + 0.00456
		X = append(X, [2]float64{x, y})
	}
	X = append(X, [2]float64{0.92324, 0.3906})
	X = append(X, [2]float64{90, 180})
	X = append(X, [2]float64{-0.92324, -0.3906})
	X = append(X, [2]float64{-90, -180})
	X = append(X, [2]float64{89.999999, 179.99999})
	X = append(X, [2]float64{0.000001, 0.000001})
	X = append(X, [2]float64{-89.999999999999, -179.999999999999})
}

func TestCoords2Geox(t *testing.T) {
	x, y := 39.92324, 116.3906
	a := EncodeInt(x, y)
	b := mmcloughlin_geohash.EncodeInt(y, x)
	c := mmcloughlin_geohash.EncodeIntx(y, x)

	if a != b {
		t.Errorf("\na:%b,\nb:%b\nc:%b", a, b, c)
		// a:10011010110001011010000100101011100000101101101010011100110010,
		// b:1000001110100100011110000001101010110000101001111010001101100110
		// c:1000011010110001011010000100101011100000101101101010011100110010
	}
	t.Error(Encode(x, y))
	t.Error(mmcloughlin_geohash.Encode(y, x))
}

func TestCoords2Geo111(t *testing.T) {
	x1 := Encode(116.3906, 39.92324)
	x, _ := encode(39.92324, 116.3906, 12)
	if x1 != x {
		t.Errorf("my:%s <--> his:%s", x1, x)
	}
	t.Error(x) // wx4g0ec19x3du
}

// 输入坐标：(39.92324, 116.3906, 0)； 预计返回 ： wx4g0ec19x3d
func TestCoords2Geo1(t *testing.T) {
	x1 := Encode(116.3906, 39.92324)
	x, _ := encode(39.92324, 116.3906, 12)
	if x1 != x {
		t.Errorf("my:%s <--> his:%s", x1, x)
	}
}
func TestCoords2Geo(t *testing.T) {
	//getArray()
	for _, val := range X {
		x1 := Encode(val[1], val[0])
		x, _ := encode(val[0], val[1], 12)
		if x1 != x {
			t.Errorf("my:%s <--> his:%s", x1, x)
		}
	}
}

func TestUint32ToCoords(t *testing.T) {
	x0, y0 := 116.3363, 39.91350
	t.Log(x0, y0)
	x1, y1 := EncodeCoords(x0, y0)
	x2, y2 := DecodeCoords(x1, y1)
	t.Log(x2, y2) //32bits: 116.3362999353559 39.91349997930237 //30bits: 116.33629985153686 39.91349989548334
}

//////////////////////////////////////////////////////////////////////////////////////////////////////////
//以下代码来自"github.com/gansidui/geohash"，LICENSE类型：MIT
//本测试用一下代码做对比

const (
	//BASE32                = "0123456789bcdefghjkmnpqrstuvwxyz"
	MAX_LATITUDE  float64 = 90
	MIN_LATITUDE  float64 = -90
	MAX_LONGITUDE float64 = 180
	MIN_LONGITUDE float64 = -180
)

var (
	bits = []int{16, 8, 4, 2, 1}
	//base32 = []byte(BASE32)
)

type Box struct {
	MinLat, MaxLat float64 // 纬度
	MinLng, MaxLng float64 // 经度
}

func (this *Box) Width() float64 {
	return this.MaxLng - this.MinLng
}

func (this *Box) Height() float64 {
	return this.MaxLat - this.MinLat
}

// 输入值：纬度，经度，精度(geohash的长度)
// 返回geohash, 以及该点所在的区域
func encode(latitude, longitude float64, precision int) (string, *Box) {
	var geohash bytes.Buffer
	var minLat, maxLat float64 = MIN_LATITUDE, MAX_LATITUDE
	var minLng, maxLng float64 = MIN_LONGITUDE, MAX_LONGITUDE
	var mid float64 = 0

	bit, ch, length, isEven := 0, 0, 0, true
	for length < precision {
		if isEven {
			if mid = (minLng + maxLng) / 2; mid < longitude {
				ch |= bits[bit]
				minLng = mid
			} else {
				maxLng = mid
			}
		} else {
			if mid = (minLat + maxLat) / 2; mid < latitude {
				ch |= bits[bit]
				minLat = mid
			} else {
				maxLat = mid
			}
		}

		isEven = !isEven
		if bit < 4 {
			bit++
		} else {
			geohash.WriteByte(encode32[ch])
			length, bit, ch = length+1, 0, 0
		}
	}

	b := &Box{
		MinLat: minLat,
		MaxLat: maxLat,
		MinLng: minLng,
		MaxLng: maxLng,
	}

	return geohash.String(), b
}

// 计算该点（latitude, longitude）在精度precision下的邻居 -- 周围8个区域+本身所在区域
// 返回这些区域的geohash值，总共9个
func GetNeighbors(latitude, longitude float64, precision int) []string {
	geohashs := make([]string, 9)

	// 本身
	geohash, b := encode(latitude, longitude, precision)
	geohashs[0] = geohash

	// 上下左右
	geohashUp, _ := encode((b.MinLat+b.MaxLat)/2+b.Height(), (b.MinLng+b.MaxLng)/2, precision)
	geohashDown, _ := encode((b.MinLat+b.MaxLat)/2-b.Height(), (b.MinLng+b.MaxLng)/2, precision)
	geohashLeft, _ := encode((b.MinLat+b.MaxLat)/2, (b.MinLng+b.MaxLng)/2-b.Width(), precision)
	geohashRight, _ := encode((b.MinLat+b.MaxLat)/2, (b.MinLng+b.MaxLng)/2+b.Width(), precision)

	// 四个角
	geohashLeftUp, _ := encode((b.MinLat+b.MaxLat)/2+b.Height(), (b.MinLng+b.MaxLng)/2-b.Width(), precision)
	geohashLeftDown, _ := encode((b.MinLat+b.MaxLat)/2-b.Height(), (b.MinLng+b.MaxLng)/2-b.Width(), precision)
	geohashRightUp, _ := encode((b.MinLat+b.MaxLat)/2+b.Height(), (b.MinLng+b.MaxLng)/2+b.Width(), precision)
	geohashRightDown, _ := encode((b.MinLat+b.MaxLat)/2-b.Height(), (b.MinLng+b.MaxLng)/2+b.Width(), precision)

	geohashs[1], geohashs[2], geohashs[3], geohashs[4] = geohashUp, geohashDown, geohashLeft, geohashRight
	geohashs[5], geohashs[6], geohashs[7], geohashs[8] = geohashLeftUp, geohashLeftDown, geohashRightUp, geohashRightDown

	return geohashs
}
