package gid

import (
	"sync"
	"sync/atomic"
	"time"

	_ "unsafe"
)

// 不用(1bit)  | 时间戳(s, 33bit,至2242年)  |  序列号(16bit,65536) | svc_id(14bit,16384)
//  63         |  30~62                     |   14~29             |    0~13
// 0x01 <<63   |    0x1ffffffff <<30        |   0xffff <<14       |    0x3fff

const (
	svcIDMask        = 0x3FFF      // 0b0011 1111 1111 1111
	tsMask           = 0x1ffffffff // 时间戳掩码
	tsLeft           = 30          // 时间戳左移位数
	snMask           = 0xffff      // 序列号掩码
	snLeft           = 14          // 序列号左移位数
	idInterval int64 = 1 << snLeft
)

var (
	svcID   int64 = 0 & svcIDMask
	lastID  int64 // 上次分配的id, 避免重复分配
	gidLock sync.Mutex

	timeMonotonic, tsMonotonic, tsRuntimeNano = func() (time.Time, int64, int64) {
		tNow := time.Now()
		return tNow, tNow.Unix(), RuntimeNano()
	}()
)

//go:linkname RuntimeNano runtime.nanotime
func RuntimeNano() int64

// GetTsNow 需要处理时间回溯的问题
func GetTsNow() int64 {
	// return time.Now().Unix()

	// 这里使用用单调时间; 中间修改了系统时间重启钱都不会产生时间回溯问题
	// return int64(time.Since(timeMonotonic)/time.Second) + tsMonotonic

	return (RuntimeNano()-tsRuntimeNano)/int64(time.Second) + tsMonotonic
}

// Parse 返回 GID 的信息
func Parse(id int64) (ts, svcID, sn int64) {
	ts = id >> tsLeft
	svcID = id & svcIDMask
	sn = (id >> snLeft) & snMask
	return
}

// Format 自己组装 GID
func Format(ts, svcID, sn int64) (id int64) {
	ts = id >> tsLeft
	svcID = id & svcIDMask
	sn = (id >> snLeft) & snMask
	return ((ts & tsMask) << tsLeft) | ((sn & snMask) << snLeft) | (svcID & svcIDMask)
}

// Interval 返回两个 GID 的 时间间隔
func Interval(gid0, gid1 int64) int64 {
	ts0 := gid0 >> tsLeft
	ts1 := gid1 >> tsLeft
	return ts0 - ts1
}

// ToTs 获取 GID 的时间戳
func ToTs(gid int64) int64 {
	return gid >> tsLeft
}

// FromTs 用秒时间戳转换成 GID
func FromTs(ts int64) int64 {
	// tsID0 |= atomic.LoadInt64(&svcID)
	return fromTs(ts) | svcID
}

func fromTs(ts int64) int64 {
	return (ts & tsMask) << tsLeft
}

func setLastGID(idNew int64) {
	for {
		idOld := atomic.LoadInt64(&lastID)
		if idNew < idOld {
			return
		}
		swapped := atomic.CompareAndSwapInt64(&lastID, idOld, idNew)
		if swapped {
			return
		}
	}
}

func Init(svcid int16, lastid int64) {
	if svcid > 0 {
		atomic.StoreInt64(&svcID, int64(svcid)&svcIDMask)
	}
	if lastid > 0 {
		setLastGID(lastid)
	}
}

// New 生成新的单调 GID
func New() int64 {
	return newID(GetTsNow())
	// t.Logf("GID:%020d", id)
	// // 3210987654321098765432109876543210987654321098765432109876543210
	// // 0001100100100111100101000001010001000000000000000100000000000001
	// t.Logf("GID:%064b", id)

	// t.Logf("divide:%064b, x:%d", id/(1<<tsLeft), 1<<tsLeft)
	// t.Logf("divide:%064b", id>>tsLeft)
}
func newID(ts int64) int64 {
	return newRawID(ts) | svcID
}

func newRawID(ts int64) int64 {
	tsID := fromTs(ts) // 当前秒数第0个编号
	id := atomic.AddInt64(&lastID, idInterval)
	if id > tsID {
		return id
	}

	if gidLock.TryLock() {
		defer gidLock.Unlock()
		id := atomic.AddInt64(&lastID, idInterval)
		if id > tsID {
			return id
		}
		atomic.StoreInt64(&lastID, tsID)
		return tsID
	}
	return id
}
