package gid

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

var _ = assert.NotNil

func TestSetLastGID(t *testing.T) {
	t.Run("SetLastGID-1", func(t *testing.T) {
		N := 1000
		ts := GetTsNow()
		for i := 0; i < N; i++ {
			id := newID(ts)
			assert.Equal(t, int64(0), id&svcIDMask)
			assert.Equal(t, int64(i), (id>>snLeft)&snMask)
		}
		svcid := int64(10086)
		Init(int16(svcid), 0)
		for i := 0; i < N; i++ {
			id := newID(ts)
			assert.Equal(t, int64(svcid), id&svcIDMask)
			assert.Equal(t, int64(i+N), (id>>snLeft)&snMask)
		}
	})
	t.Run("SetLastGID-2", func(t *testing.T) {
		N := 1000
		ts := GetTsNow()
		svcid := int64(10086)
		Init(int16(svcid), 0)
		for i := 0; i < N; i++ {
			id := newID(ts)
			assert.Equal(t, int64(svcid), id&svcIDMask)
			assert.Equal(t, int64(i), (id>>snLeft)&snMask)
		}
	})

	t.Run("Init", func(t *testing.T) {
		ts := time.Now().Unix()
		lastID := newID(ts)
		Init(0, lastID)
		for i := 0; i < 300; i++ {
			id := newID(ts)
			lastID += idInterval
			if id != lastID {
				t.Logf("id:%064b", lastID)
				t.Logf("id:%064b", id)
				t.Fatalf("id != lastID\nid:%064b\nid:%064b", id, lastID)
			}
		}

		lastID = newID(ts + 100)
		Init(0, lastID)
		for i := 0; i < 300; i++ {
			id := newID(ts)
			lastID += idInterval
			if id != lastID {
				t.Logf("id:%064b", lastID)
				t.Logf("id:%064b", id)
				t.Fatalf("id != lastID\nid:%064b\nid:%064b", id, lastID)
			}
		}

		ts += 100 + 1
		curID := newID(ts)
		if lastID >= curID {
			t.Logf("lastID:%064b", lastID)
			t.Logf(" curID:%064b", curID)
			t.Fatal("lastID >= curID")
		}
		for i := 0; i < 3; i++ {
			lastID, curID = curID+idInterval, newID(ts)
			if lastID != curID {
				t.Logf("id:%064b", lastID)
				t.Logf("id:%064b", curID)
				t.Fatalf("id != lastID\nid:%064b\nid:%064b", lastID, curID)
			}
		}
	})
}

func TestT(t *testing.T) {
	var gid int64 = 1835294501162188800
	t1 := gid / 1073741824
	if t1 != gid>>30 {
		panic("eeee")
	}
	tsStr, agentid, sn := Parse(gid)

	t.Log("agentid", agentid)
	t.Log("sn", sn)
	t.Log("ts", t1)
	t.Log("tsStr", tsStr)
	t.Log("gid", gid)
}

func TestGetGID(t *testing.T) {
	Init(0b01010101010101, 0)
	t.Run("New", func(t *testing.T) {
		var lastID int64
		//     id:0001101010011111101000011110101101000000000000000000000000000000
		t.Log("    |              ts               ||      sn      ||   svc_id   |")
		for i := 0; i < 3; i++ {
			for i := 0; i < 3; i++ {
				id := New()
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
			id := New()
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
				id := New()
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

	if GetTsNow() != time.Now().Unix() {
		panic("GetTsNow")
	}
}
