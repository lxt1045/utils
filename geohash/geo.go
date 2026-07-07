package geohash

import (
	"math"
)

/*
//赤道周长 40075.2km ,经线长度 40037/2 km；/12位geohash,即30bit 经度,30bit 纬度,
//矩形的长为:40075.2km * cos(latitude) / 2^30
//矩形的宽为:40037km /2 / 2^30  20018.5
//geohash位数:        12            11            10            9            8            7             6            5            4            3            2            1
//经线/纬线位数:    30/30        27/28        25/25        22/23        20/20        17/18          15/15        12/13        10/10        7/8            5/5            2/3
//纬度/经度
//60°时,长:                                                           0.0191         0.0764       0.6115        2.446        19.568        78.27
//45°时,长:                                                           0.0270         0.108        0.865         3.46         27.67        110.69
//30°时,长:                                                           0.0331         0.1324       1.059         4.237        33.89        135.57
//0°时(赤道),长:  0.0000373    0.0001493   0.0011943    0.0047773     0.0382187      0.15287      1.2230        4.8920       39.136       156.54        1252.3        5009.4
//宽:            0.0000186    0.0001491   0.0005960    0.0047728      0.0190911      0.15273      0.6109        4.8873       19.549       156.39        625.27        5004.6
//三亚18°左右,黑龙江50°左右:                                          0.03km/0.02    0.12km/0.08   0.9km/0.6     3.8/4.8      31/19        123/156
//误差估值：                                                          0.02km         0.1km         0.6km         4km          25km         140kn
//为了计算方便,取近似值:                                                             0.1km         0.6km         4km           25km         140km
//*/

const (
	earthClat = 40075.2     //地球赤道周长40075.2km
	earthClng = 40037.0 / 2 //地球经线长度40037.0/2km

	exp32      = 1 << 32                            // 刻度数量(多少个刻度值)
	deltaAngle = 0.0000000000001                    // 角度偏移,为了计算方便;//不用DELTAANGLE时,在（90/180）会溢出
	encode32   = "0123456789bcdefghjkmnpqrstuvwxyz" //
	invalid    = 0xff
)

var (
	decode32 = func() (de [256]byte) {
		for i := 0; i < len(de); i++ {
			de[i] = invalid
		}
		for i := 0; i < len(encode32); i++ {
			de[encode32[i]] = byte(i)
		}
		return
	}()
)

// Coords  坐标
type Coords struct {
	Lat float64 // 纬度
	Lng float64 // 经度
}

// Area 一个闭合区域：收尾相接
type Area struct {
	area []Coords
}

// Encode 坐标直接转成geohash的显示格式
func Encode(lat, lng float64) string {
	return Geo2Str(EncodeInt(lat, lng), 12)
}
func Decode(str string) (lat, lng float64) {
	hash := Str2Geo(str)
	return DecodeInt(hash)
}

// EncodeInt 输入值:纬度,经度,精度(geohash的长度), 返回geohash
func EncodeInt(lat, lng float64) uint64 {
	x, y := EncodeCoords(lat, lng)
	return interleave64(x, y)
}
func EncodeInt2(lat, lng float64) uint64 {
	x, y := EncodeCoords(lat, lng)
	return interleave64_2(x, y)
}

func DecodeInt(geo uint64) (lat, lng float64) {
	//x, y := Coords2Uint32(latitude,longitude)
	x, y := deinterleave64(geo)
	return DecodeCoords(x, y)
}

// EncodeCoords 输入值:纬度,经度,精度(geohash的长度), 返回一个正方形区域, 该区域由4个坐标点表示
func EncodeCoords(lat, lng float64) (uint32, uint32) {
	const reciprocalPrecisionLat = exp32 / (180.0 + deltaAngle) //纬度的刻度的倒数,每度有多少个刻度值
	const reciprocalPrecisionLng = exp32 / (360.0 + deltaAngle) //经度的刻度的倒数,每度有多少个刻度值
	valLat := (lat + 90) * reciprocalPrecisionLat               //直接使用90,
	valLng := (lng + 180) * reciprocalPrecisionLng              //直接使用180,在（90.180）会有问题

	return uint32(valLat), uint32(valLng)
}

// DecodeCoords 输入值:纬度,经度,精度(geohash的长度), 返回一个正方形区域, 该区域由4个坐标点表示
func DecodeCoords(x uint32, y uint32) (lat, lng float64) {
	const precisionLat = (180.0 + deltaAngle) / float64(exp32) //纬度的刻度,每个刻度值有多少度
	const precisionLng = (360.0 + deltaAngle) / float64(exp32) //经度的刻度,每个刻度值有多少度

	lat = precisionLat*float64(x) - 90.0
	lng = precisionLng*float64(y) - 180.0

	return
}

// Geo2Str 转成字母显示格式，只有标准的5bit一组的数据才能转！
func Geo2Str(x uint64, chars uint) string {
	x = x >> (64 - 5*chars)
	b := [12]byte{}
	for i := range 12 {
		b[11-i] = encode32[x&0x1f]
		x >>= 5
	}
	return string(b[:])
}

func Str2Geo(str string) (x uint64) {
	bits := uint(5 * len(str))
	for i := 0; i < len(str); i++ {
		x = (x << 5) | uint64(decode32[str[i]])
	}
	return x << (64 - bits)
}

//交错和反交错

/* Interleave lower bits of x and y, so the bits of x
 * are in the even positions and bits from y in the odd;
 * x and y must initially be less than 2**32 (65536).
 * From:  https://graphics.stanford.edu/~seander/bithacks.html#InterleaveBMN
 */

var (
	B = []uint64{0x5555555555555555, 0x3333333333333333, 0x0F0F0F0F0F0F0F0F,
		0x00FF00FF00FF00FF, 0x0000FFFF0000FFFF, 0x00000000FFFFFFFF}
	S = []uint64{0, 1, 2, 4, 8, 16} //B、S放到函数内每次都生成需要额外5.4ns/op
)

// 偶数位放经度，奇数位放纬度
func interleave64(xlo, ylo uint32) (b uint64) {
	x := uint64(xlo)
	y := uint64(ylo)

	x = (x | (x << 16)) & 0x0000FFFF0000FFFF
	x = (x | (x << 8)) & 0x00FF00FF00FF00FF
	x = (x | (x << 4)) & 0x0F0F0F0F0F0F0F0F
	x = (x | (x << 2)) & 0x3333333333333333
	x = (x | (x << 1)) & 0x5555555555555555

	y = (y | (y << 16)) & 0x0000FFFF0000FFFF
	y = (y | (y << 8)) & 0x00FF00FF00FF00FF
	y = (y | (y << 4)) & 0x0F0F0F0F0F0F0F0F
	y = (y | (y << 2)) & 0x3333333333333333
	y = (y | (y << 1)) & 0x5555555555555555

	return x | (y << 1)
}

// 偶数位放经度，奇数位放纬度
func interleave64_2(xlo, ylo uint32) (b uint64) {
	x := uint64(xlo)
	y := uint64(ylo)
	x = pdep64(x, 0x5555555555555555)
	y = pdep64(y, 0x5555555555555555)

	return x | (y << 1)
}

/* reverse the interleave process
 * derived from http://stackoverflow.com/questions/4909263
 */
func deinterleave64(interleaved uint64) (xlo, ylo uint32) { // uint64 {
	x := interleaved & 0x5555555555555555
	y := interleaved >> 1

	x = x & 0x5555555555555555
	x = (x | (x >> 1)) & 0x3333333333333333
	x = (x | (x >> 2)) & 0x0F0F0F0F0F0F0F0F
	x = (x | (x >> 4)) & 0x00FF00FF00FF00FF
	x = (x | (x >> 8)) & 0x0000FFFF0000FFFF
	x = (x | (x >> 16)) & 0x00000000FFFFFFFF

	y = y & 0x5555555555555555
	y = (y | (y >> 1)) & 0x3333333333333333
	y = (y | (y >> 2)) & 0x0F0F0F0F0F0F0F0F
	y = (y | (y >> 4)) & 0x00FF00FF00FF00FF
	y = (y | (y >> 8)) & 0x0000FFFF0000FFFF
	y = (y | (y >> 16)) & 0x00000000FFFFFFFF

	return uint32(x), uint32(y)
}

// geoPrecision geohash的精度，即刻度大小: 角度
func geoPrecision(bits uint) (latPrecision, lngPrecision float64) {
	b := int(bits)
	latBits := b / 2
	lngBits := b - latBits
	latPrecision = math.Ldexp(180.0, -latBits)
	lngPrecision = math.Ldexp(360.0, -lngBits)
	return
}

func Center(geo uint64, bits uint) (lat, lng float64) {
	x, y := DecodeInt(geo)
	latErr, lngErr := geoPrecision(bits)
	lat, lng = x+latErr/2, y+lngErr/2
	return
}

func NeighborsInt(geo uint64, bits uint) []uint64 {
	latBits := bits / 2       // lat 误差 bit 位
	lngBits := bits - latBits // lng 误差 bit 位
	latDelta := uint32(1 << (32 - latBits))
	lngDelta := uint32(1 << (32 - lngBits))

	// mask := uint64(0xFFFFFFFFFFFFFFFF << bits)
	mask := uint64(math.MaxUint64) << (64 - bits)
	geo = geo & mask
	lat, lng := deinterleave64(geo)

	/*
		NW   N   NE
		W   me   E
		SW   S   SE
	*/
	return []uint64{
		// N
		interleave64(lat+latDelta, lng),
		// NE,
		interleave64(lat+latDelta, lng+lngDelta),
		// E,
		interleave64(lat, lng+lngDelta),
		// SE,
		interleave64(lat-latDelta, lng+lngDelta),
		// S,
		interleave64(lat-latDelta, lng),
		// SW,
		interleave64(lat-latDelta, lng-lngDelta),
		// W,
		interleave64(lat, lng-lngDelta),
		// NW
		interleave64(lat+latDelta, lng-lngDelta),
	}
}

func NeighborsInt2(geo uint64, bits uint) []uint64 {
	const latMask, lngMask uint64 = 0x5555555555555555, 0x5555555555555555 << 1
	const latAdd, lngAdd uint64 = 0x5555555555555555 << 1, 0x5555555555555555

	bitsLeft := 64 - bits
	// 不用 deinterleave64 和 interleave64: 拆开后，直接加减
	deta := uint64(0b11 << bitsLeft)
	latDelta := deta & latMask
	lngDelta := deta & lngMask

	// mask := uint64(0xFFFFFFFFFFFFFFFF << bits)
	mask := uint64(math.MaxUint64) << bitsLeft
	geo = geo & mask
	lat, lng := geo&latMask, geo&lngMask

	// fmt.Printf("lat:%064b\n", lat)
	// fmt.Printf("lde:%064b\n", latDelta)
	// fmt.Printf("lad:%064b\n", latAdd)
	// fmt.Printf("lma:%064b\n\n", latMask)
	// fmt.Printf("lng:%064b\n", lng)
	// fmt.Printf("lde:%064b\n", lngDelta)
	// fmt.Printf("lad:%064b\n", lngAdd)
	// fmt.Printf("lma:%064b\n\n", lngMask)
	/*
		NW   N   NE
		W   me   E
		SW   S   SE
	*/
	latN := (lat + latAdd + latDelta) & latMask
	latS := (lat - latDelta) & latMask
	lngE := (lng + lngAdd + lngDelta) & lngMask
	lngW := (lng - lngDelta) & lngMask
	return []uint64{
		// N
		// interleave64(lat+latDelta, lng),
		latN | lng,
		// NE,
		// interleave64(lat+latDelta, lng+lngDelta),
		latN | lngE,
		// E,
		// interleave64(lat, lng+lngDelta),
		lat | lngE,
		// SE,
		// interleave64(lat-latDelta, lng+lngDelta),
		latS | lngE,
		// S,
		// interleave64(lat-latDelta, lng),
		latS | lng,
		// SW,
		// interleave64(lat-latDelta, lng-lngDelta),
		latS | lngW,
		// W,
		// interleave64(lat, lng-lngDelta),
		lat | lngW,
		// NW
		// interleave64(lat+latDelta, lng-lngDelta),
		latN | lngW,
	}
}

// Dist 计算两坐标间的距离(distance)
func Dist(p1, p2 Coords) (l float64) {
	calculateY := func(p1, p2 Coords) float64 {
		deltaY := p1.Lat - p2.Lat
		if deltaY < 0 {
			deltaY = -deltaY
		}
		return deltaY / 180.0 * earthClng //180°时，即经线长度: earthClng
	}
	calculateX := func(p1, p2 Coords) float64 {
		alphaY := p1.Lat
		deltaX := p1.Lng - p2.Lng
		if deltaX < 0 {
			deltaX = -deltaX
		}
		cosY := math.Cos(math.Pi * alphaY / 180) //该纬度上的纬线圈对应的角度的cos()
		C := earthClat * cosY                    //earthClat 是赤道的长度，C是纬线圈的长度(周长和半径成正比)
		return deltaX / 360.0 * C
	}

	x := calculateX(p1, p2)
	y := calculateY(p1, p2)

	return math.Sqrt(x*x + y*y)
}

// Dist2 计算两坐标间的距离(distance)
func Dist2(p1, p2 Coords) (l float64) {

	deltaY := p1.Lat - p2.Lat
	// if deltaY < 0 {
	// 	deltaY = -deltaY
	// }
	y := deltaY / 180.0 * earthClng //180°时，即经线长度: earthClng

	deltaX := p1.Lng - p2.Lng
	// if deltaX < 0 {
	// 	deltaX = -deltaX
	// }
	cosY := math.Cos(math.Pi * p1.Lat / 180.0) //该纬度上的纬线圈对应的角度的cos()
	C := earthClat * cosY                      //earthClat 是赤道的长度，C是纬线圈的长度(周长和半径成正比)
	x := deltaX / 360.0 * C

	return math.Sqrt(x*x + y*y)
}
