package gid

import (
	"sync"
	"sync/atomic"
	"time"

	_ "unsafe"
)

var (
	agentID    int64 = 0 & 0x3FFF // 0b0001 1111 1111 1111
	idInterval int64 = 1 << 14
	lastID     int64
	gidLock    sync.Mutex

	timeMonotonic, tsMonotonic, tsRuntimeNano = func() (time.Time, int64, int64) {
		tNow := time.Now()
		return tNow, tNow.Unix(), RuntimeNano()
	}()
)

const (
	AgentIDMask          = 0x3FFF
	OffsetServiceAgentID = 0x3000
	MaxServiceAgentID    = 0xfff
	MaxClientAgentID     = 0x2FFF
)

func InitClient(agentid int16, lastid int64) {
	atomic.StoreInt64(&agentID, int64(agentid&0x3FFF))
	if (lastid & AgentIDMask) == int64(agentid) {
		SetLastGID(lastid)
		return
	}
	for {
		lastid := atomic.LoadInt64(&lastID)
		if lastid&0x3fff == agentID {
			return
		}
		now := (lastid & (^0x3fff)) | agentID
		swapped := atomic.CompareAndSwapInt64(&lastID, lastid, now)
		if swapped {
			return
		}
	}
}

// tLastUpdated 单位 秒
func InitService(svcid int16, tLastUpdated int64) {
	agentID = int64(svcid&0xFFF) | 0x3000

	lastid := TsToGID(tLastUpdated + 30) // 为避免重复，往前走30秒；service 每10s刷新一次
	SetLastGID(lastid)
}

func SetLastGID(lastid int64) {
	for {
		now := atomic.LoadInt64(&lastID)
		if lastid < now {
			return
		}
		if lastid&0x3fff != agentID {
			lastid = (lastid & (^0x3fff)) | agentID
		}
		swapped := atomic.CompareAndSwapInt64(&lastID, now, lastid)
		if swapped {
			return
		}
	}
}

func Parse(id int64) (tsStr string, agentid, sNO int64) {
	t1 := id >> 30
	tsStr = time.Unix(t1, 0).Format(time.RFC3339)
	agentid = id & 0x3fff
	sNO = (id >> 14) & 0xffff
	return
}

func TsToGID(ts int64) int64 {
	tsID0 := (ts & 0x1FFFFFFFF) << 30
	tsID0 |= agentID
	return tsID0
}

func GIDToTs(gid int64) int64 {
	return gid >> 30
}

func DiffTs(gid0, gid1 int64) int64 {
	ts0 := gid0 >> 30
	ts1 := gid1 >> 30
	return ts0 - ts1
}

//go:linkname RuntimeNano runtime.nanotime
func RuntimeNano() int64

func GetTsNow() int64 {
	// return time.Now().Unix()

	// 这里使用用单调时间
	// return int64(time.Since(timeMonotonic)/time.Second) + tsMonotonic

	return (RuntimeNano()-tsRuntimeNano)/int64(time.Second) + tsMonotonic

}

/*
	    基于IM（即时通信）的ID可以采用：
		不用(1bit)  | 时间戳(min, 20bit, 1.9年) |  序列号(11bit,2048) | agent_id(32bit,42亿)
		63         |  43~62                |   32~42           |    0~31
*/

/*
	    基于IM（即时通信）的ID可以采用：
		不用(1bit)  | 时间戳(s, 25bit, 1年) |  序列号(11bit,2048)| agent_id(27bit,1.3亿,连接后分配)
		63         |  38~62                |   27~37         |    0~26
*/
/*
	    基于IM（即时通信）的ID可以采用：
		不用(1bit)  | 时间戳(ms, 36bit, 2年) | agent_id(27bit,1.3亿,连接后分配)
		63         |  27~62                 |      0~26
*/
func GetGID() int64 {
	// 不用(1bit)  | 时间戳(s, 33bit,至2242年)  |  序列号(16bit,65536) | agent_id(14bit,16384;0x3000~0x3fff 预留给service)
	//  63         |  30~62                     |   14~29             |    0~13
	// 0x01 <<63   |    0x1ffffffff <<30        |   0xffff <<14       |    0x3fff

	tsID0 := TsToGID(GetTsNow()) // 当前秒数第0个编号
	id := atomic.AddInt64(&lastID, idInterval)
	if id > tsID0 {
		return id
	}

	if gidLock.TryLock() {
		defer gidLock.Unlock()
		atomic.StoreInt64(&lastID, tsID0)
		return tsID0
	}
	return id
	// t.Logf("GID:%020d", id)
	// // 3210987654321098765432109876543210987654321098765432109876543210
	// // 0001100100100111100101000001010001000000000000000100000000000001
	// t.Logf("GID:%064b", id)

	// t.Logf("divide:%064b, x:%d", id/(1<<30), 1<<30)
	// t.Logf("divide:%064b", id>>30)
}

func GetGIDWithAgentID(agentID int64) int64 {
	gid := GetGID()
	return GIDResetAgentID(gid, agentID)
}

func GIDResetAgentID(gid, agentID int64) int64 {
	gid = (gid & 0x7FFFFFFFFFFFC000) | (agentID & 0x3FFF)
	return gid
}
