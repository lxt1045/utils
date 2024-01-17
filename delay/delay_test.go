package delay

import (
	"sync/atomic"
	"testing"
	"time"

	"github.com/bytedance/mockey"
	"github.com/stretchr/testify/assert"
)

type delayData struct {
	data int
	f    func(d delayData)
}

func (d delayData) Post() {
	if d.f != nil {
		d.f(d)
	}
}

func TestNew(t *testing.T) {
	tNow, err := time.Parse(time.RFC3339, "2023-10-02T00:00:00Z")
	if err != nil {
		t.Fatal(err)
	}
	pNow := &tNow
	// fUnPatch := mockey.Mock(time.Now).Return(tNow).Build().UnPatch
	defer mockey.Mock(time.Now).To(func() time.Time { return *pNow }).Build().UnPatch()

	varDelay := 0
	f := func(d delayData) {
		// 延时处理
		varDelay++
	}
	N := 8
	p := New[delayData](N, int64(1*time.Second), false)
	assert.Equal(t, p.qcap, uint32(N))

	datas := []delayData{
		{data: 1, f: f}, {data: 2, f: f}, {data: 3, f: f}, {data: 4, f: f}, {data: 5, f: f},
		{data: 6, f: f}, {data: 7, f: f}, {data: 8, f: f}, {data: 9, f: f},
	}

	t.Run("Push", func(t *testing.T) {
		for i, d := range datas[:7] {
			closed := p.Push(d)
			if closed {
				t.Fatal(err)
			}

			assert.Equal(t, p.qlen, int32(i+1))
			assert.Equal(t, p.head, uint32(0))
			assert.Equal(t, p.tail, uint32(i+1))
			assert.Equal(t, p.closed, uint32(0))

			t.Run("Range", func(t *testing.T) {
				j := 0
				p.Range(func(d delayData) {
					j++
					assert.Equal(t, j, d.data)
					return
				})
				assert.Equal(t, j, i+1)
			})
		}
		for i, d := range datas[7:] {
			closed := p.Push(d)
			if closed {
				t.Fatal(err)
			}

			assert.Equal(t, p.qlen, int32(i+8))
			assert.Equal(t, p.head, uint32(0))
			assert.Equal(t, p.tail, uint32(i+8))
			assert.Equal(t, p.closed, uint32(0))

			t.Run("Range", func(t *testing.T) {
				j := 0
				p.Range(func(d delayData) {
					j++
					assert.Equal(t, j, d.data)
					return
				})
			})
		}
	})

	pp := *p
	for i := range datas {
		pp.head = uint32(i)
		t.Run("Range-for", func(t *testing.T) {
			j := i
			pp.Range(func(d delayData) {
				j++
				assert.Equal(t, j, d.data)
				return
			})
			assert.Equal(t, j, len(datas))
		})
	}

	t.Run("popN", func(t *testing.T) {
		for i := p.head; i != p.tail; i = (i + 1) % p.qcap {
			if p.queue[i].deadline != 0 {
				p.queue[i].deadline = pNow.UnixNano() + int64(i)*int64(time.Second)
			}
		}
		idx, l := 1, int(p.qlen)
		for i := 0; i <= l; i++ {
			p.pop(nil)
			j := idx
			p.Range(func(d delayData) {
				assert.Equal(t, j, d.data)
				j++
				return
			})
			assert.Equal(t, p.qlen, int32(len(datas)-int(i)))
			assert.Equal(t, j-idx, len(datas)-int(i))
			idx++
			*pNow = pNow.Add(time.Second) //
		}
		assert.Equal(t, p.qlen, int32(0))
		assert.Equal(t, p.tail, p.head)
		p.pop(nil)
		assert.Equal(t, p.qlen, int32(0))
		assert.Equal(t, p.tail, p.head)
	})

	t.Run("Close", func(t *testing.T) {
		p.Close()
	})
}

func BenchmarkPush(b *testing.B) {
	N := 1024 * 1024
	p := New[delayData](N, int64(1*time.Millisecond))
	assert.Equal(b, p.qcap, uint32(N))

	var varDelay int64
	f := func(d delayData) {
		// 延时处理
		atomic.AddInt64(&varDelay, 1)
	}

	datas := []delayData{
		{data: 1, f: f}, {data: 2, f: f}, {data: 3, f: f}, {data: 4, f: f}, {data: 5, f: f},
		{data: 6, f: f}, {data: 7, f: f}, {data: 8, f: f}, {data: 9, f: f},
	}

	b.Run("Push", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			d := datas[i%len(datas)]
			p.Push(d)
		}
		b.SetBytes(int64(b.N))
	})
	b.Logf("varDelay:%d", atomic.LoadInt64(&varDelay))
}
