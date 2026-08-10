package channel_test

import (
	"runtime"
	"sync"
	"testing"

	"github.com/lxt1045/utils/channel"
)

func TestChanN(t *testing.T) {
	ch := channel.NewChanN[Log](1024)
	as := []Log{
		Log{LogId: 0}, Log{LogId: 1}, Log{LogId: 2}, Log{LogId: 3},
		Log{LogId: 4}, Log{LogId: 5}, Log{LogId: 6}, Log{LogId: 7},
	}

	n, closed := ch.SendN(as)
	if n != len(as) || closed {
		t.Error("++++++++++++++++error:err")
	}

	for i := 0; i < 8; i += 4 {
		msgs, closed := ch.Recv(4, 1)
		if closed || len(msgs) != 4 {
			t.Errorf("++++++++++++++++error:closed:%v, len(msgs)==%d", closed, len(msgs))
		}
		for j, m := range msgs {
			if int(m.LogId) != i+j {
				t.Errorf("++++++++++++++++error; or m.LogId:%d != i:%d", m.LogId, i+j)
			}
		}
	}
	msgs, closed := ch.Recv(4, 0)
	if len(msgs) != 0 || closed == true {
		t.Error("++++++++++++++++error:err:", msgs, "closed:", closed)
	}
	ch.Close()
	msgs, closed = ch.Recv(4, 0)
	if len(msgs) != 0 || closed != true {
		t.Error("++++++++++++++++error:err:", msgs, "closed:", closed)
	}
}

func TestChanNSend(t *testing.T) {
	ch := channel.NewChanN[Log](8)
	as := []Log{
		Log{LogId: 0}, Log{LogId: 1}, Log{LogId: 2}, Log{LogId: 3},
		Log{LogId: 4}, Log{LogId: 5}, Log{LogId: 6}, Log{LogId: 7},
	}
	{
		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			msgs, closed := ch.Recv(1, 1)
			if closed || len(msgs) != 1 {
				t.Errorf("++++++++++++++++error:closed:%v, len(msgs)==%d", closed, len(msgs))
			}
			wg.Done()
		}()
		runtime.Gosched() //切换协程，保证先Recv
		full, closed := ch.Send(as[3])
		if full || closed {
			t.Error("++++++++++++++++error:err")
		}
		wg.Wait()
	}
	{
		for _, v := range as {
			full, closed := ch.Send(v)
			if full || closed {
				t.Error("++++++++++++++++error:err")
			}
		}
		msgs, closed := ch.Recv(8, 1)
		if closed || len(msgs) != 8 {
			t.Errorf("++++++++++++++++error:closed:%v, len(msgs)==%d", closed, len(msgs))
		}
		for i, m := range msgs {
			if int(m.LogId) != i {
				t.Errorf("++++++++++++++++error:ok; or m.LogId:%d != i:%d", m.LogId, i)
			} else {
				t.Logf("get msg:%v", m)
			}
		}
	}
	//full
	ch.SendN(as)
	full, closed := ch.Send(as[7])
	if !full || closed {
		t.Error("++++++++++++++++error:err")
	}

	//closed
	ch.Close()
	full, closed = ch.Send(as[7])
	if full || !closed {
		t.Error("++++++++++++++++error:err")
	}
}

func TestChanNSendN(t *testing.T) {
	ch := channel.NewChanN[Log](8)
	as := []Log{
		Log{LogId: 0}, Log{LogId: 1}, Log{LogId: 2}, Log{LogId: 3},
		Log{LogId: 4}, Log{LogId: 5}, Log{LogId: 6}, Log{LogId: 7},
	}
	as1 := []Log{
		Log{LogId: 0}, Log{LogId: 1}, Log{LogId: 2}, Log{LogId: 3},
		Log{LogId: 4}, Log{LogId: 5}, Log{LogId: 6}, Log{LogId: 7},
		Log{LogId: 0}, Log{LogId: 1}, Log{LogId: 2}, Log{LogId: 3},
	}
	{
		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			msgs, closed := ch.Recv(16, 1)
			if closed || len(msgs) != 8 {
				t.Errorf("++++++++++++++++error:closed:%v, len(msgs)==%d", closed, len(msgs))
			}
			wg.Done()
		}()
		runtime.Gosched() //切换协程，保证先Recv
		n, closed := ch.SendN(as)
		if n != 8 || closed {
			t.Errorf("++++++++++++++++error:n:%d != 8 || closed:%v", n, closed)
		}
		wg.Wait()
	}
	//普通操作
	{
		n, closed := ch.SendN(as[:4])
		if n != 4 || closed {
			t.Error("++++++++++++++++error:err")
		}
		n, closed = ch.SendN(as[4:])
		if n != 4 || closed {
			t.Error("++++++++++++++++error:err")
		}

		msgs, closed := ch.Recv(8, 1)
		if closed || len(msgs) != 8 {
			t.Errorf("++++++++++++++++error:closed:%v, len(msgs)==%d", closed, len(msgs))
		}
		for i, m := range msgs {
			if int(m.LogId) != i {
				t.Errorf("++++++++++++++++error:ok; or m.LogId:%d != i:%d", m.LogId, i)
			} else {
				t.Logf("get msg:%v", m)
			}
		}
	}
	{
		n, closed := ch.SendN(as[:4])
		if n != 4 || closed {
			t.Error("++++++++++++++++error:err")
		}
		n, closed = ch.SendN(as1[4:9])
		if n != 4 || closed {
			t.Error("++++++++++++++++error:err")
		}
		msgs, closed := ch.Recv(16, 1)
		if closed || len(msgs) != 8 {
			t.Errorf("++++++++++++++++error:closed:%v, len(msgs)==%d", closed, len(msgs))
		}
		for i, m := range msgs {
			if int(m.LogId) != i {
				t.Errorf("++++++++++++++++error; or m.LogId:%d != i:%d", m.LogId, i)
			} else {
				t.Logf("get msg:%v", m)
			}
		}
	}
	{
		n, closed := ch.SendN(as1)
		if n != 8 || closed {
			t.Error("++++++++++++++++error:err")
		}
		msgs, closed := ch.Recv(16, 1)
		if closed || len(msgs) != 8 {
			t.Errorf("++++++++++++++++error:closed:%v, len(msgs)==%d", closed, len(msgs))
		}
		for i, m := range msgs {
			if int(m.LogId) != i {
				t.Errorf("++++++++++++++++error; or m.LogId:%d != i:%d", m.LogId, i)
			} else {
				t.Logf("get msg:%v", m)
			}
		}
	}
	{
		n, closed := ch.SendN(as1)
		n, closed = ch.SendN(as)
		if n != 0 || closed != false {
			t.Error("++++++++++++++++error:err")
		}
		//closed
		ch.Close()
		n, closed = ch.SendN(as)
		if n != 0 || !closed {
			t.Error("++++++++++++++++error:err")
		}
	}
	{
		ch := channel.NewChanN[Log](0)
		if n, closed := ch.SendN(as); n != 1 || closed != false {
			t.Error("++++++++++++++++error:err")
		}
	}
}

func TestChanNRecv(t *testing.T) {
	ch := channel.NewChanN[Log](8)
	as := []Log{
		Log{LogId: 0}, Log{LogId: 1}, Log{LogId: 2}, Log{LogId: 3},
		Log{LogId: 4}, Log{LogId: 5}, Log{LogId: 6}, Log{LogId: 7},
	}
	{
		n, closed := ch.SendN(as)
		if n != len(as) || closed {
			t.Error("++++++++++++++++error:err")
		}

		for i := 0; i < 8; i += 4 {
			msgs, closed := ch.Recv(4, 1)
			if closed || len(msgs) != 4 {
				t.Errorf("++++++++++++++++error:closed:%v, len(msgs)==%d", closed, len(msgs))
			}
			for j, m := range msgs {
				if int(m.LogId) != i+j {
					t.Errorf("++++++++++++++++error; or m.LogId:%d != i:%d", m.LogId, i+j)
				}
			}
		}
		msgs, closed := ch.Recv(4, 0)
		if len(msgs) != 0 || closed == true {
			t.Error("++++++++++++++++error:err:", msgs, "closed:", closed)
		}
		ch.Close()
		msgs, closed = ch.Recv(4, 0)
		if len(msgs) != 0 || closed != true {
			t.Error("++++++++++++++++error:err:", msgs, "closed:", closed)
		}
	}
}

func TestChanNRecvBlockClosed(t *testing.T) {
	runtime.GOMAXPROCS(1)
	ch := channel.NewChanN[Log](8)
	as := []Log{
		Log{LogId: 0}, Log{LogId: 1}, Log{LogId: 2}, Log{LogId: 3},
		Log{LogId: 4}, Log{LogId: 5}, Log{LogId: 6}, Log{LogId: 7},
	}
	{
		var wg sync.WaitGroup
		wg.Add(2)
		for i := 0; i < 2; i++ {
			go func() {
				msgs, closed := ch.Recv(1, 1)
				if closed || len(msgs) != 1 {
					t.Errorf("++++++++++++++++error:closed:%v, len(msgs)==%d", closed, len(msgs))
				}
				wg.Done()
			}()
		}
		runtime.Gosched() //切换协程，保证先Recv
		n, closed := ch.SendN(as[:2])
		if n != 2 || closed {
			t.Errorf("++++++++++++++++error:n:%d != 8 || closed:%v", n, closed)
		}
		wg.Wait()
	}
	{
		var wg sync.WaitGroup
		wg.Add(2)
		//closed
		for i := 0; i < 2; i++ {
			go func() {
				msgs, closed := ch.Recv(1, 1)
				if !closed || len(msgs) != 0 {
					t.Errorf("++++++++++++++++error:closed:%v, len(msgs)==%d", closed, len(msgs))
				}
				wg.Done()
			}()
		}

		runtime.Gosched()
		ch.Close()
		wg.Wait()
	}
}

func TestChanNSendCover(t *testing.T) {
	ch := channel.NewChanN[Log](8)
	as := []Log{
		Log{LogId: 0}, Log{LogId: 1}, Log{LogId: 2}, Log{LogId: 3},
		Log{LogId: 4}, Log{LogId: 5}, Log{LogId: 6}, Log{LogId: 7},
	}
	{
		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			msgs, closed := ch.Recv(1, 1)
			if closed || len(msgs) != 1 {
				t.Errorf("++++++++++++++++error:closed:%v, len(msgs)==%d", closed, len(msgs))
			}
			wg.Done()
		}()
		runtime.Gosched() //切换协程，保证先Recv
		full, closed := ch.Send(as[3])
		if full || closed {
			t.Error("++++++++++++++++error:err")
		}
		wg.Wait()
	}
	{
		for _, v := range as {
			full, closed := ch.Send(v)
			if full || closed {
				t.Error("++++++++++++++++error:err")
			}
		}
		msgs, closed := ch.Recv(8, 1)
		if closed || len(msgs) != 8 {
			t.Errorf("++++++++++++++++error:closed:%v, len(msgs)==%d", closed, len(msgs))
		}
		for i, m := range msgs {
			if int(m.LogId) != i {
				t.Errorf("++++++++++++++++error; or m.LogId:%d != i:%d", m.LogId, i)
			} else {
				t.Logf("get msg:%v", m)
			}
		}
	}
	//full
	ch.SendN(as)
	full, closed := ch.Send(as[7])
	if !full || closed {
		t.Error("++++++++++++++++error:err")
	}

	//closed
	ch.Close()
	full, closed = ch.Send(as[7])
	if full || !closed {
		t.Error("++++++++++++++++error:err")
	}
}

func TestChanNSendCoverN(t *testing.T) {
	ch := channel.NewChanN[Log](8)
	as := []Log{
		Log{LogId: 0}, Log{LogId: 1}, Log{LogId: 2}, Log{LogId: 3},
		Log{LogId: 4}, Log{LogId: 5}, Log{LogId: 6}, Log{LogId: 7},
	}
	as1 := []Log{
		Log{LogId: 0}, Log{LogId: 1}, Log{LogId: 2}, Log{LogId: 3},
		Log{LogId: 4}, Log{LogId: 5}, Log{LogId: 6}, Log{LogId: 7},
		Log{LogId: 0}, Log{LogId: 1}, Log{LogId: 2}, Log{LogId: 3},
	}
	{
		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			msgs, closed := ch.Recv(16, 1)
			if closed || len(msgs) != 8 {
				t.Errorf("++++++++++++++++error:closed:%v, len(msgs)==%d", closed, len(msgs))
			}
			wg.Done()
		}()
		runtime.Gosched() //切换协程，保证先Recv
		n, closed := ch.SendN(as)
		if n != 8 || closed {
			t.Errorf("++++++++++++++++error:n:%d != 8 || closed:%v", n, closed)
		}
		wg.Wait()
	}
	//普通操作
	{
		n, closed := ch.SendN(as[:4])
		if n != 4 || closed {
			t.Error("++++++++++++++++error:err")
		}
		n, closed = ch.SendN(as[4:])
		if n != 4 || closed {
			t.Error("++++++++++++++++error:err")
		}

		msgs, closed := ch.Recv(8, 1)
		if closed || len(msgs) != 8 {
			t.Errorf("++++++++++++++++error:closed:%v, len(msgs)==%d", closed, len(msgs))
		}
		for i, m := range msgs {

			if int(m.LogId) != i {
				t.Errorf("++++++++++++++++error; or m.LogId:%d != i:%d", m.LogId, i)
			} else {
				t.Logf("get msg:%v", m)
			}
		}
	}
	{
		n, closed := ch.SendN(as[:4])
		if n != 4 || closed {
			t.Error("++++++++++++++++error:err")
		}
		n, closed = ch.SendN(as1[4:9])
		if n != 5 || closed {
			t.Error("++++++++++++++++error:err")
		}
		msgs, closed := ch.Recv(16, 1)
		if closed || len(msgs) != 8 {
			t.Errorf("++++++++++++++++error:closed:%v, len(msgs)==%d", closed, len(msgs))
		}
		for i, m := range msgs {

			if int(m.LogId) != (i+1)%8 {
				t.Errorf("++++++++++++++++error; or m.LogId:%d != i:%d", m.LogId, (i+1)%8)
			} else {
				t.Logf("get msg:%v", m)
			}
		}
	}
	{
		n, closed := ch.SendN(as1)
		if n != 8 || closed {
			t.Error("++++++++++++++++error:err")
		}
		msgs, closed := ch.Recv(16, 1)
		if closed || len(msgs) != 8 {
			t.Errorf("++++++++++++++++error:closed:%v, len(msgs)==%d", closed, len(msgs))
		}
		for i, m := range msgs {

			if int(m.LogId) != i {
				t.Errorf("++++++++++++++++error; or m.LogId:%d != i:%d", m.LogId, i)
			} else {
				t.Logf("get msg:%v", m)
			}
		}
	}
	{
		//closed
		ch.Close()
		n, closed := ch.SendN(as)
		if n != 0 || !closed {
			t.Error("++++++++++++++++error:err")
		}
	}
}

func TestChanNCover(t *testing.T) {
	ch := channel.NewChanN[Log](1024)
	as := []Log{
		Log{LogId: 0}, Log{LogId: 1}, Log{LogId: 2}, Log{LogId: 3},
		Log{LogId: 4}, Log{LogId: 5}, Log{LogId: 6}, Log{LogId: 7},
	}

	n, closed := ch.SendN(as)
	if n != len(as) || closed {
		t.Error("++++++++++++++++error:err")
	}

	for i := 0; i < 8; i += 4 {
		msgs, closed := ch.Recv(4, 1)
		if closed || len(msgs) != 4 {
			t.Errorf("++++++++++++++++error:closed:%v, len(msgs)==%d", closed, len(msgs))
		}
		for j, m := range msgs {

			if int(m.LogId) != i+j {
				t.Errorf("++++++++++++++++error; or m.LogId:%d != i:%d", m.LogId, i+j)
			}
		}
	}
	msgs, closed := ch.Recv(4, 0)
	if len(msgs) != 0 || closed == true {
		t.Error("++++++++++++++++error:err:", msgs, "closed:", closed)
	}
	ch.Close()
	msgs, closed = ch.Recv(4, 0)
	if len(msgs) != 0 || closed != true {
		t.Error("++++++++++++++++error:err:", msgs, "closed:", closed)
	}
}

func TestChanNRecvBlockClosedCover(t *testing.T) {
	runtime.GOMAXPROCS(1)
	ch := channel.NewChanN[Log](8)
	as := []Log{
		Log{LogId: 0}, Log{LogId: 1}, Log{LogId: 2}, Log{LogId: 3},
		Log{LogId: 4}, Log{LogId: 5}, Log{LogId: 6}, Log{LogId: 7},
	}
	{
		var wg sync.WaitGroup
		wg.Add(2)
		for i := 0; i < 2; i++ {
			go func() {
				msgs, closed := ch.Recv(1, 1)
				if closed || len(msgs) != 1 {
					t.Errorf("++++++++++++++++error:closed:%v, len(msgs)==%d", closed, len(msgs))
				}
				wg.Done()
			}()
		}
		runtime.Gosched() //切换协程，保证先Recv
		n, closed := ch.SendN(as[:2])
		if n != 2 || closed {
			t.Errorf("++++++++++++++++error:n:%d != 8 || closed:%v", n, closed)
		}
		wg.Wait()
	}
	{
		var wg sync.WaitGroup
		wg.Add(2)
		//closed
		for i := 0; i < 2; i++ {
			go func() {
				msgs, closed := ch.Recv(1, 1)
				if !closed || len(msgs) != 0 {
					t.Errorf("++++++++++++++++error:closed:%v, len(msgs)==%d", closed, len(msgs))
				}
				wg.Done()
			}()
		}

		runtime.Gosched()
		ch.Close()
		wg.Wait()
	}
}

//以下是性能测试

// BenchmarkSend0-4   	50000000	        26.1 ns/op	1913873693.37 MB/s	       0 B/op	       0 allocs/op
func BenchmarkSend0(b *testing.B) {
	b.StopTimer()
	ch := channel.NewChanN[*Log](b.N)
	a := &Log{}
	as := []*Log{a}
	_ = as

	b.ReportAllocs()
	b.StartTimer()
	for i := 0; i < b.N; i++ { //use b.N for looping
		ch.Send(a)
	}
	b.SetBytes(int64(b.N))
}

// 1: BenchmarkSendN0-4   	50000000	        30.3 ns/op	1648829410.26 MB/s	       0 B/op	       0 allocs/op
// 8: BenchmarkSendN0-4   	30000000	        36.4 ns/op	823862877.00 MB/s	       0 B/op	       0 allocs/op
// 16:BenchmarkSendN0-4   	30000000	        48.7 ns/op	615896283.07 MB/s	       0 B/op	       0 allocs/op
func BenchmarkSendN0(b *testing.B) {
	b.StopTimer()
	a := &Log{}
	as := []*Log{a, a, a, a, a, a, a, a, a, a, a, a, a, a, a, a}
	as = as[:16]
	ch := channel.NewChanN[*Log](b.N)

	b.ReportAllocs()
	for _ = range as {
		b.StartTimer()
		for i := 0; i < b.N/len(as); i++ { //use b.N for looping
			ch.SendN(as)
		}
		b.StopTimer()
		for i := 0; i < b.N/32; i++ { //use b.N for looping
			ch.Recv(32, 0)
		}
	}
	b.SetBytes(int64(b.N))
}

// 1：BenchmarkRecv0-4   	20000000	        68.0 ns/op	294069774.23 MB/s	      16 B/op	       1 allocs/op
// 8：BenchmarkRecv0-4   	10000000	       138 ns/op	72243639.65 MB/s	     128 B/op	       1 allocs/op
// 16:BenchmarkRecv0-4   	10000000	       189 ns/op	52832891.73 MB/s	     256 B/op	       1 allocs/op
// 32:BenchmarkRecv0-4   	 5000000	       381 ns/op	13117991.88 MB/s	     512 B/op	       1 allocs/op
func BenchmarkRecv0(b *testing.B) {
	b.StopTimer()
	N := 8
	ch := channel.NewChanN[*Log](b.N)
	a := &Log{}
	as := []*Log{a, a, a, a, a, a, a, a, a, a, a, a, a, a, a, a}
	as = as[:8]

	for i := 0; i < b.N; i++ { //use b.N for looping
		ch.Send(as[0])
	}

	b.ReportAllocs()

	for j := 0; j < N; j++ {
		b.StartTimer()
		for i := 0; i < b.N/N; i++ { //use b.N for looping
			ch.Recv(N, 0)
		}
		b.StopTimer()
		for i := 0; i < b.N/len(as); i++ { //use b.N for looping
			ch.SendN(as)
		}
	}
	b.SetBytes(int64(b.N))
}

// 1：BenchmarkRecv1-4   	50000000	        29.4 ns/op	1699319891.40 MB/s	       0 B/op	       0 allocs/op
// 8：BenchmarkRecv1-4   	50000000	        31.1 ns/op	1607578510.92 MB/s	       0 B/op	       0 allocs/op
// 16:BenchmarkRecv1-4   	30000000	        35.3 ns/op	848777067.85 MB/s	       0 B/op	       0 allocs/op
// 32:BenchmarkRecv1-4   	30000000	        52.1 ns/op	575789988.66 MB/s	       0 B/op	       0 allocs/op
func BenchmarkRecv1(b *testing.B) {
	b.StopTimer()
	N := 16
	ch := channel.NewChanN[*Log](b.N)
	a := &Log{}
	as := []*Log{a, a, a, a, a, a, a, a, a, a, a, a, a, a, a, a}
	as = as[:N]

	for i := 0; i < b.N; i++ { //use b.N for looping
		ch.Send(as[0])
	}

	b.ReportAllocs()
	gets := make([]*Log, N)
	for j := 0; j < N; j++ {
		b.StartTimer()
		for i := 0; i < b.N/N; i++ { //use b.N for looping
			ch.Read(gets, 0)
		}
		b.StopTimer()
		for i := 0; i < b.N/len(as); i++ { //use b.N for looping
			ch.SendN(as)
		}
	}
	b.SetBytes(int64(b.N))
}

// 额外收获，一次interface{}()强制转化要30ns,,,,,!
func BenchmarkSend(b *testing.B) {
	ch := channel.NewChanN[*Log](1024)
	a := &Log{}
	as := []*Log{a}
	_ = as
	var wg sync.WaitGroup
	wg.Add(1)
	for i := 0; i < 1; i++ {
		go func() {
			wg.Done()
			for {
				for i := 0; i < 1; i++ {
					//ch.Recv(1, 1)
				}
				runtime.Gosched()
			}
		}()
	}
	for i := 0; i < 0; i++ {
		go func() {
			wg.Done()
			for {
				for i := 0; i < 64; i++ {
					ch.Send(a)
				}
				runtime.Gosched()
			}
		}()
	}

	wg.Wait()
	for i := 0; i < b.N; i++ { //use b.N for looping
		//单协程写，无读，仅覆盖旧数据,ch.Send(a)：
		//BenchmarkChanWrap3Send-4   	20000000	        64.1 ns/op	      32 B/op	       1 allocs/op
		//BenchmarkChanWrap3Send-4   	20000000	        70.7 ns/op	      32 B/op	       1 allocs/op
		//单协程写，无读，仅覆盖旧数据,ch.SendN(as)：
		//BenchmarkChanWrap3Send-4   	50000000	        30.4 ns/op	       0 B/op	       0 allocs/op
		//单协程写，无读，仅覆盖旧数据,ch.SendN([]Log{a})：
		//BenchmarkChanWrap3Send-4   	20000000	        71.5 ns/op	      32 B/op	       1 allocs/op
		//单协程写，无读，仅覆盖旧数据,ch.Send(ia)：一次interf{}()强制转化要30ns,,,,,!
		//BenchmarkChanWrap3Send-4   	50000000	        26.4 ns/op	       0 B/op	       0 allocs/op
		//单协程写，单协程读，一次一个数据：
		//BenchmarkChanWrap3Send-4   	10000000	       115 ns/op	      33 B/op	       1 allocs/op
		//BenchmarkChanWrap3Send-4   	20000000	       119 ns/op	      33 B/op	       1 allocs/op
		//单协程写，单协程读，一次16个数据：
		//BenchmarkChanWrap3Send-4   	20000000	        78.2 ns/op	      32 B/op	       1 allocs/op
		//BenchmarkChanWrap3Send-4   	20000000	        77.0 ns/op	      32 B/op	       1 allocs/op
		//8协程写，8协程读操作：
		//BenchmarkChanWrap3Send-4   	 5000000	       356 ns/op	     274 B/op	       7 allocs/op
		//BenchmarkChanWrap3Send-4   	 5000000	       354 ns/op	     274 B/op	       7 allocs/op
		//8协程写，无读：
		//BenchmarkChanWrap3Send-4   	10000000	       188 ns/op	     153 B/op	       4 allocs/op
		//BenchmarkChanWrap3Send-4   	10000000	       197 ns/op	     154 B/op	       4 allocs/op
		//128协程写，无读：
		//BenchmarkChanWrap3Send-4   	 1000000	      1017 ns/op	     507 B/op	      15 allocs/op
		//BenchmarkChanWrap3Send-4   	 2000000	       826 ns/op	     512 B/op	      15 allocs/op
		//BenchmarkChanWrap3Send-4   	 2000000	       964 ns/op	     600 B/op	      18 allocs/op
		//64写，64读，一次读16个数据：
		//BenchmarkChanWrap3Send-4   	 2000000	       907 ns/op	     613 B/op	      17 allocs/op
		//BenchmarkChanWrap3Send-4   	 2000000	       753 ns/op	     497 B/op	      13 allocs/op
		//64写，64读，一次读1个数据：
		//BenchmarkChanWrap3Send-4   	 3000000	       651 ns/op	     392 B/op	      12 allocs/op
		//BenchmarkChanWrap3Send-4   	 3000000	       792 ns/op	     490 B/op	      15 allocs/op
		//BenchmarkChanWrap3Send-4   	 3000000	       718 ns/op	     443 B/op	      13 allocs/op
		//ch.SendN(as)
		//ch.Send(a)
		//ch.SendN([]Log{a})
		ch.Send(a)
	}
}

/*
func BenchmarkChanWrap3SendN(b *testing.B) {
	ch := NewChanWrap3(1024)
	a := Log{}
	as := make([]Log, 64)
	as1 := []Log{
		as[0], as[1], as[2], as[3],
		as[0], as[1], as[2], as[3],
	}
	var wg sync.WaitGroup
	wg.Add(127)
	for i := 0; i < 64; i++ {
		go func() {
			wg.Done()
			for {
				for i := 0; i < 64; i++ {
					ch.Recv(8, 1)
				}
				runtime.Gosched()
			}
		}()
	}
	for i := 0; i < 63; i++ {
		go func() {
			wg.Done()
			for {
				for i := 0; i < 64; i++ {
					ch.Send(a)
				}
				runtime.Gosched()
			}
		}()
	}

	wg.Wait()
	for i := 0; i < b.N; i++ { //use b.N for looping
		//单协程写，无读，仅覆盖旧数据,一次写入8个数据：
		//BenchmarkChanWrap3SendN-4   	50000000	        34.0 ns/op	       0 B/op	       0 allocs/op
		//单协程写，单协程读，一次写8，读4个数据：
		//BenchmarkChanWrap3SendN-4   	20000000	        78.7 ns/op	       4 B/op	       0 allocs/op
		//单协程写，单协程读，一次写8，读64个数据：
		//BenchmarkChanWrap3SendN-4   	20000000	        76.1 ns/op	      33 B/op	       0 allocs/op
		//单协程写，单协程读，一次16个数据：
		//BenchmarkChanWrap3Send-4   	20000000	        78.2 ns/op	      32 B/op	       1 allocs/op
		//BenchmarkChanWrap3Send-4   	20000000	        77.0 ns/op	      32 B/op	       1 allocs/op
		//8协程写，8协程读操作：
		//BenchmarkChanWrap3SendN-4   	30000000	        52.7 ns/op	      38 B/op	       0 allocs/op
		//64写，64读，一次读16个数据：
		//BenchmarkChanWrap3SendN-4   	10000000	       164 ns/op	     103 B/op	       2 allocs/op
		//BenchmarkChanWrap3SendN-4   	 5000000	       211 ns/op	     118 B/op	       2 allocs/op
		ch.SendN(as1)
	}
}

func BenchmarkChanWrap3Recv(b *testing.B) {
	ch := NewChanWrap3(1024)
	a := Log{}
	var wg sync.WaitGroup
	wg.Add(1)
	for i := 0; i < 0; i++ {
		go func() {
			wg.Done()
			for {
				for i := 0; i < 1; i++ {
					ch.Recv(1, 0)
				}
				runtime.Gosched()
			}
		}()
	}
	for i := 0; i < 1; i++ {
		go func() {
			wg.Done()
			for {
				for i := 0; i < 64; i++ {
					ch.Send(a)
				}
				runtime.Gosched()
			}
		}()
	}

	wg.Wait()
	for i := 0; i < b.N; i++ { //use b.N for looping
		//单协程写，1读,一次读一个数据：
		//BenchmarkChanWrap3Recv-4   	30000000	        75.6 ns/op	      48 B/op	       1 allocs/op
		//BenchmarkChanWrap3Recv-4   	50000000	        76.8 ns/op	      49 B/op	       1 allocs/op
		//8协程写，单协程读，一次16个数据，no block：
		//BenchmarkChanWrap3Recv-4   	30000000	        73.5 ns/op	      44 B/op	       1 allocs/op
		//BenchmarkChanWrap3Recv-4   	50000000	        74.4 ns/op	      47 B/op	       1 allocs/op
		//8协程写,一次写一个，单协程读，一次1个数据,block：
		//BenchmarkChanWrap3Recv-4   	 5000000	       393 ns/op	     277 B/op	       9 allocs/op
		//BenchmarkChanWrap3Recv-4   	 5000000	       387 ns/op	     276 B/op	       9 allocs/op
		//8协程写,一次写一个，单协程读，一次16个数据,block：
		//BenchmarkChanWrap3Recv-4   	  500000	      2790 ns/op	    2276 B/op	      65 allocs/op
		//8协程写，8协程读操作，一次读一个：
		//BenchmarkChanWrap3Recv-4   	30000000	        79.2 ns/op	      46 B/op	       1 allocs/op
		//BenchmarkChanWrap3Recv-4   	30000000	        81.1 ns/op	      48 B/op	       1 allocs/op
		//8协程写，无读：
		//BenchmarkChanWrap3Send-4   	10000000	       188 ns/op	     153 B/op	       4 allocs/op
		//BenchmarkChanWrap3Send-4   	10000000	       197 ns/op	     154 B/op	       4 allocs/op
		//128协程写，无读：
		//BenchmarkChanWrap3Send-4   	 1000000	      1017 ns/op	     507 B/op	      15 allocs/op
		//BenchmarkChanWrap3Send-4   	 2000000	       826 ns/op	     512 B/op	      15 allocs/op
		//BenchmarkChanWrap3Send-4   	 2000000	       964 ns/op	     600 B/op	      18 allocs/op
		//64写，64读，一次读16个数据：
		//BenchmarkChanWrap3Send-4   	 2000000	       907 ns/op	     613 B/op	      17 allocs/op
		//BenchmarkChanWrap3Send-4   	 2000000	       753 ns/op	     497 B/op	      13 allocs/op
		//64写，64读，一次读1个数据：
		//BenchmarkChanWrap3Send-4   	 3000000	       651 ns/op	     392 B/op	      12 allocs/op
		//BenchmarkChanWrap3Send-4   	 3000000	       792 ns/op	     490 B/op	      15 allocs/op
		//BenchmarkChanWrap3Send-4   	 3000000	       718 ns/op	     443 B/op	      13 allocs/op
		ch.Recv(16, 1)
	}
}

func BenchmarkChanWrap3RecvIface(b *testing.B) {
	ch := NewChanWrap3(1024)
	as := make([]Log, 64*16)
	for i, _ := range as {
		as[i] = Log{LogId: uint16(i % 16)}
	}
	var wg sync.WaitGroup
	wg.Add(1)
	for i := 0; i < 0; i++ {
		go func() {
			wg.Done()
			for {
				for i := 0; i < 1; i++ {
					ch.Recv(1, 0)
				}
				runtime.Gosched()
			}
		}()
	}
	for i := 0; i < 1; i++ {
		go func() {
			wg.Done()
			for {
				for i := 0; i < 64; i++ {
					//ch.SendN(as[i*16 : (i+1)*16])
					ch.SendN(as[:4])
				}
				runtime.Gosched()
			}
		}()
	}

	wg.Wait()
	iface, _ := ch.Recv(2, 1)
	_ = iface
	for i := 0; i < b.N; i++ { //use b.N for looping
		iface, _ := ch.Recv(2, 1)
		//ch.Recv(1, 1)
		pkg, ok := iface[0].(Log)
		if !ok {
			b.Error("error", pkg)
		}
		//	// if pkg.LogId != 111 {
		//	// 	b.Errorf("error, ok:%v, pkg.LogId:%d!=111", pkg.LogId)
		//	// }

	}
}

//*/
