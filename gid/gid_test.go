package gid

import (
	"testing"
	"time"

	"github.com/bytedance/mockey"
	"github.com/stretchr/testify/assert"
)

var _ = assert.NotNil

func TestSetLastGID(t *testing.T) {
	t.Run("SetLastGID-1", func(t *testing.T) {
		N := 1000
		for i := 0; i < N; i++ {
			id := GetGID()
			assert.Equal(t, int64(0), id&AgentIDMask)
			assert.Equal(t, int64(i), (id>>14)&0xffff)
		}
		agentid := int64(1086)
		InitClient(int16(agentid), 0)
		for i := 0; i < N; i++ {
			id := GetGID()
			assert.Equal(t, int64(agentid), id&AgentIDMask)
			assert.Equal(t, int64(i+N), (id>>14)&0xffff)
		}
	})

	t.Run("SetLastGID", func(t *testing.T) {
		lastID = 0
		tNow, err := time.Parse(time.RFC3339, "2006-01-02T15:04:05Z")
		if err != nil {
			t.Fatal(err)
		}
		fUnPatch := mockey.Mock(time.Now).Return(tNow).Build().UnPatch

		var lastID int64
		id := GetGID()
		if lastID >= id {
			t.Fatal("lastID>id", lastID, id)
		}
		lastID = id
		fGetID := func(i time.Duration) int64 {
			tsID0 := (time.Now().Add(i).Unix() & 0x1FFFFFFFF) << 30 // 当前秒数第0个编号
			tsID0 |= agentID
			return tsID0
		}
		nextSecond := time.Second * 100
		maxID := fGetID(nextSecond)
		SetLastGID(maxID)
		for i := 0; i < 300; i++ {
			id := GetGID()
			maxID += idInterval
			if id != maxID {
				t.Logf("id:%064b", maxID)
				t.Logf("id:%064b", id)
				t.Fatalf("id != maxID\nid:%064b\nid:%064b", id, maxID)
			}
		}
		fUnPatch()
		tNow = tNow.Add(nextSecond * 3)
		defer mockey.Mock(time.Now).Return(tNow).Build().UnPatch()
		lastMaxID, maxID := maxID, fGetID(0)
		if lastMaxID >= maxID {
			t.Logf("maxID:%064b", maxID)
			t.Logf("maxID:%064b", fGetID(0))
			t.Fatal("lastMaxID >= maxID")
		}
		maxID = fGetID(0)
		for i := 0; i < 3; i++ {
			id := GetGID()
			if id != maxID {
				t.Logf("id:%064b", maxID)
				t.Logf("id:%064b", id)
				t.Fatalf("id != maxID\nid:%064b\nid:%064b", id, maxID)
			}
			maxID += idInterval
		}
	})
}

func TestT(t *testing.T) {
	var Logid int64 = 1835294501162188800
	t1 := Logid / 1073741824
	if t1 != Logid>>30 {
		panic("eeee")
	}
	tsStr, agentid, sNO := Parse(Logid)

	t.Log("agentid", agentid)
	t.Log("sNO", sNO)
	t.Log("ts", t1)
	t.Log("tsStr", tsStr)
	t.Log("Logid", Logid)
}

func TestGetGID(t *testing.T) {
	t.Run("GetGID", func(t *testing.T) {
		var lastID int64
		for i := 0; i < 3; i++ {
			for i := 0; i < 3; i++ {
				id := GetGID()
				if lastID >= id {
					t.Fatal("lastID>id", lastID, id)
				}
				lastID = id

				// t.Logf("id:%020d", id)
				// 3210987654321098765432109876543210987654321098765432109876543210
				// 0001100100100111100101000001010001000000000000000100000000000001
				t.Logf("id:%064b", id)
			}
			time.Sleep(time.Second)
		}
	})
}

func BenchmarkGetGID(b *testing.B) {
	b.Run("GetGID", func(b *testing.B) {
		var lastID int64
		for i := 0; i < b.N; i++ {
			id := GetGID()
			if lastID >= id {
				b.Fatal("lastID>id", lastID, id)
			}
			lastID = id
		}
	})

	b.Run("GetGID-RunParallel", func(b *testing.B) {
		b.RunParallel(func(pb *testing.PB) {
			var lastID int64
			for pb.Next() {
				id := GetGID()
				if lastID >= id {
					b.Fatal("lastID>id", lastID, id)
				}
				lastID = id
			}
		})
	})

	b.Run("time.Now().Unix()", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = time.Now().UnixNano()
		}
	})

	now := time.Now()
	b.Run("time.Since(tt)", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = time.Since(now)
		}
	})
	b.Run("time.Now(tt)+Sub", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = time.Now().Sub(now)
		}
	})
	b.Run("RuntimeNano", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = RuntimeNano()
		}
	})
	b.Run("GetTsNow", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = GetTsNow()
		}
	})
}

func TestGetTsNow(t *testing.T) {
	t.Log("GetTsNow", GetTsNow())
	t.Log("time.Now().Unix()", time.Now().Unix())
}

func TestGIDResetAgentID(t *testing.T) {
	t.Run("1", func(t *testing.T) {
		id := GIDResetAgentID(0x7FFFFFFFFFFFFFFF, 1)
		assert.Equal(t, id, int64(0x7FFFFFFFFFFFC001))
	})
	t.Run("0x3FFE", func(t *testing.T) {
		id := GIDResetAgentID(0x7FFFFFFFFFFFFFFF, 0x3FFE)
		// t.Logf("0x3FFE:%X", id)
		assert.Equal(t, id, int64(0x7FFFFFFFFFFFFFFE))
	})
}
