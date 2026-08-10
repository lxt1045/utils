package channel_test

import (
	//"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
)

type Log struct {
	LogId int64
}

// 0协程写，1协程读：
// 100000000	        10.13 ns/op	9876432984.01 MB/s	       0 B/op	       0 allocs/op
func BenchmarkChanRawRecv0(b *testing.B) {
	b.StopTimer()
	ch := make(chan *Log, b.N)
	pkg := &Log{}
	for i := 0; i < b.N; i++ { //use b.N for looping
		ch <- pkg
	}
	b.ReportAllocs()
	b.StartTimer()
	for i := 0; i < b.N; i++ { //use b.N for looping
		<-ch
	}
	b.SetBytes(int64(b.N))
}

// 0协程写，1协程读：
// 107439499	        12.70 ns/op	8459037669.85 MB/s	       0 B/op	       0 allocs/op
func BenchmarkChanRawSend0(b *testing.B) {
	b.StopTimer()
	ch := make(chan *Log, b.N)
	pkg := &Log{}
	b.ReportAllocs()
	b.StartTimer()
	for i := 0; i < b.N; i++ { //use b.N for looping
		ch <- pkg
	}
	b.SetBytes(int64(b.N))
}

func BenchmarkChanRawRecv(b *testing.B) {
	ch := make(chan Log, 1024)
	pkg := Log{}
	var wg sync.WaitGroup
	wg.Add(15)
	for i := 0; i < 8; i++ {
		go func() {
			wg.Done()
			for {
				ch <- pkg
				ch <- pkg
				ch <- pkg
				ch <- pkg
				ch <- pkg
				ch <- pkg
				//runtime.Gosched()
			}
		}()
	}
	for i := 0; i < 7; i++ {
		go func() {
			wg.Done()
			for {
				<-ch
				<-ch
				<-ch
				<-ch
				<-ch
				<-ch
				<-ch
				<-ch
				<-ch
				<-ch
				<-ch
				runtime.Gosched()
			}
		}()
	}
	wg.Wait()
	for i := 0; i < b.N; i++ { //use b.N for looping
		//单协程写，单协程读：
		//BenchmarkChanRawRecv-4   	20000000	       104 ns/op	       1 B/op	       0 allocs/op
		//BenchmarkChanRawRecv-4   	20000000	       100 ns/op	       1 B/op	       0 allocs/op
		//8协程写，8协程读：
		//BenchmarkChanRawRecv-4   	 2000000	       854 ns/op	      16 B/op	       0 allocs/op
		//BenchmarkChanRawRecv-4   	20000000	       106 ns/op	       1 B/op	       0 allocs/op//非强竞争状态
		//64协程写，64协程读：
		//BenchmarkChanRawRecv-4   	  500000	      4979 ns/op	      65 B/op	       0 allocs/op
		<-ch

	}
}
func BenchmarkChanRawRecvSelcet(b *testing.B) {
	ch := make(chan *Log, 1024)
	pkg := &Log{} //大点，减少cache影响
	var wg sync.WaitGroup
	wg.Add(7)
	for i := 0; i < 7; i++ {
		go func() {
			wg.Done()
			for {
				ch <- pkg
				ch <- pkg
				ch <- pkg
				ch <- pkg
				ch <- pkg
				ch <- pkg
				//runtime.Gosched()
			}
		}()
	}
	for i := 0; i < 0; i++ {
		go func() {
			wg.Done()
			for {
				<-ch
				<-ch
				<-ch
				<-ch
				<-ch
				<-ch
				<-ch
				<-ch
				<-ch
				<-ch
				<-ch
				runtime.Gosched()
			}
		}()
	}
	wg.Wait()
	for i := 0; i < b.N; i++ { //use b.N for looping
		/*
			总体来说，仅仅加入select+default，不会增加多少的时间消耗，如果探测到没有数据，
			则只会消耗一个和原子操作差不多的时间5ns左右
			单线程读，select一个，大量写入保证数据：
			BenchmarkChanRawRecvSelcet-4   	20000000	        91.2 ns/op	       1 B/op	       0 allocs/op
			BenchmarkChanRawRecvSelcet-4   	20000000	        87.1 ns/op	       1 B/op	       0 allocs/op
			BenchmarkChanRawRecvSelcet-4   	20000000	        97.3 ns/op	       1 B/op	       0 allocs/op
			单线程读，select 8个，大量写入保证数据：
			BenchmarkChanRawRecvSelcet-4   	 3000000	       532 ns/op	      10 B/op	       0 allocs/op
			BenchmarkChanRawRecvSelcet-4   	 3000000	       498 ns/op	      10 B/op	       0 allocs/op
			BenchmarkChanRawRecvSelcet-4   	 3000000	       524 ns/op	      10 B/op	       0 allocs/op
			BenchmarkChanRawRecvSelcet-4   	 3000000	       532 ns/op	      10 B/op	       0 allocs/op
		*/
		select {
		case <-ch:
		case <-ch:
		case <-ch:
		case <-ch:
		case <-ch:
		case <-ch:
		case <-ch:
		case <-ch:
		default:
		}

	}
}
func BenchmarkChanRawSend(b *testing.B) {
	ch := make(chan Log, 1024)
	pkg := Log{}
	var wg sync.WaitGroup
	wg.Add(1)
	for i := 0; i < 0; i++ {
		go func() {
			wg.Done()
			for {
				ch <- pkg
				runtime.Gosched()
			}
		}()
	}
	for i := 0; i < 1; i++ {
		go func() {
			wg.Done()
			for {
				<-ch
				<-ch
				<-ch
				<-ch
				<-ch
				<-ch
				<-ch
			}
		}()
	}

	//wg.Wait()
	j := 0
	for i := 0; i < b.N; i++ { //use b.N for looping
		j++
		j = j % 1024000
		//单协程写，单协程读：
		//BenchmarkChanRawSend-4   	20000000	       103 ns/op	       1 B/op	       0 allocs/op
		//BenchmarkChanRawSend-4   	20000000	       102 ns/op	       1 B/op	       0 allocs/op
		//8协程写，8协程读：
		//BenchmarkChanRawSend-4   	 2000000	       721 ns/op	      16 B/op	       0 allocs/op
		//64协程写，64协程读：
		//BenchmarkChanRawSend-4   	 1000000	      1562 ns/op	      32 B/op	       0 allocs/op
		//BenchmarkChanRawSend-4   	 5000000	      2889 ns/op	       6 B/op	       0 allocs/op
		ch <- pkg
	}
}

type chanWrap struct {
	Ch  chan Log
	N   int32
	Max int32
}

func NewChanWrap(n int) *chanWrap {
	m := chanWrap{
		Ch:  make(chan Log, n+8),
		Max: int32(n),
	}
	return &m
}
func (p *chanWrap) Recv() *Log {
	m := <-p.Ch
	atomic.AddInt32(&p.N, -1)
	return &m
}
func (p *chanWrap) Send(m *Log) {
	if atomic.AddInt32(&p.N, 1) > p.Max {
		// atomic.AddInt32(&p.N, -1)
		// return
		select {
		case <-p.Ch:
			//如果超出容量，则销毁最早的数据
			atomic.AddInt32(&p.N, -1)
		default:
		}
	}
	p.Ch <- *m

}

func BenchmarkChanWrapSend(b *testing.B) {
	ch := NewChanWrap(1024)
	pkg := Log{}
	var wg sync.WaitGroup
	wg.Add(127)
	for i := 0; i < 63; i++ {
		go func() {
			wg.Done()
			for {
				ch.Send(&pkg)
				ch.Send(&pkg)
				ch.Send(&pkg)
				ch.Send(&pkg)
				ch.Send(&pkg)
				ch.Send(&pkg)
				ch.Send(&pkg)
				ch.Send(&pkg)
				ch.Send(&pkg)
				ch.Send(&pkg)
				ch.Send(&pkg)
				ch.Send(&pkg)
				runtime.Gosched()
			}
		}()
	}
	for i := 0; i < 64; i++ {
		go func() {
			wg.Done()
			for {
				ch.Recv()
			}
		}()
	}

	wg.Wait()
	j := 0
	for i := 0; i < b.N; i++ { //use b.N for looping
		j++
		j = j % 1024000
		//单协程写，单协程读：
		//BenchmarkChanWrapSend-4   	10000000	       134 ns/op	       3 B/op	       0 allocs/op
		//BenchmarkChanWrapSend-4   	10000000	       128 ns/op	       3 B/op	       0 allocs/op
		//8协程写，8协程读：
		//BenchmarkChanWrapSend-4   	10000000	       206 ns/op	       3 B/op	       0 allocs/op
		//BenchmarkChanWrapSend-4   	10000000	       206 ns/op	       3 B/op	       0 allocs/op
		//64协程写，64协程读：
		//BenchmarkChanWrapSend-4   	10000000	       245 ns/op	       3 B/op	       0 allocs/op
		//BenchmarkChanWrapSend-4   	10000000	       221 ns/op	       3 B/op	       0 allocs/op
		ch.Send(&pkg)
	}
}
func BenchmarkChanWrapRecv(b *testing.B) {
	ch := NewChanWrap(1024)
	pkg := Log{}
	var wg sync.WaitGroup
	wg.Add(127)
	for i := 0; i < 64; i++ {
		go func() {
			wg.Done()
			j := 0
			for {
				j++
				j = j % 1024000
				for i := 0; i < 64; i++ {
					ch.Send(&pkg)
				}
				runtime.Gosched()
			}
		}()
	}
	for i := 0; i < 63; i++ {
		go func() {
			wg.Done()
			for {
				ch.Recv()
				//runtime.Gosched()
			}
		}()
	}

	wg.Wait()
	j := 0
	for i := 0; i < b.N; i++ { //use b.N for looping
		j++
		j = j % 1024000
		//单协程写，单协程读：
		//BenchmarkChanWrapRecv-4   	10000000	       280 ns/op	       3 B/op	       0 allocs/op
		//BenchmarkChanWrapRecv-4   	10000000	       218 ns/op	       3 B/op	       0 allocs/op
		//8协程写，8协程读：
		//BenchmarkChanWrapRecv-4   	 1000000	      2154 ns/op	      32 B/op	       0 allocs/op
		//BenchmarkChanWrapRecv-4   	 1000000	      2154 ns/op	      32 B/op	       0 allocs/op
		//64协程写，64协程读：
		//BenchmarkChanWrapRecv-4   	  100000	     20148 ns/op	     328 B/op	       0 allocs/op
		//BenchmarkChanWrapRecv-4   	 1000000	     15025 ns/op	      32 B/op	       0 allocs/op
		//BenchmarkChanWrapRecv-4   	  200000	     15855 ns/op	     164 B/op	       0 allocs/op
		ch.Recv()
	}
}

func BenchmarkAtomicRaw(b *testing.B) {
	x := int64(0)
	var wg sync.WaitGroup
	wg.Add(127)
	for i := 0; i < 127; i++ {
		go func() {
			wg.Done()
			for {
				atomic.AddInt64(&x, 1)
				runtime.Gosched()
				a := Log{}
				_ = a
			}
		}()
	}

	wg.Wait()
	for i := 0; i < b.N; i++ { //use b.N for looping
		//单协程写，单协程读：
		//BenchmarkAtomicRaw-4   	300000000	         5.50 ns/op	       0 B/op	       0 allocs/op
		//64协程写，64协程读：
		//BenchmarkAtomicRaw-4   	300000000	         5.50 ns/op	       0 B/op	       0 allocs/op
		//127操作：
		//BenchmarkAtomicRaw-4   	200000000	         6.32 ns/op	       0 B/op	       0 allocs/op
		atomic.AddInt64(&x, -1)
		a := Log{}
		_ = a
	}
}

func BenchmarkMutexRaw(b *testing.B) {
	lock := new(sync.Mutex)
	var wg sync.WaitGroup
	wg.Add(15)
	for i := 0; i < 15; i++ {
		go func() {
			wg.Done()
			for {
				for i := 0; i < 64; i++ {
					lock.Lock()
					a := Log{}
					_ = a
					lock.Unlock()
				}
				runtime.Gosched()
			}
		}()
	}

	wg.Wait()
	for i := 0; i < b.N; i++ { //use b.N for looping
		//开锁解锁操作，即无竞争状态下：
		//BenchmarkMutexRaw-4   	100000000	        15.4 ns/op	       0 B/op	       0 allocs/op
		//单协程写，单协程读，锁内无操作：
		//BenchmarkMutexRaw-4   	50000000	        49.5 ns/op	       0 B/op	       0 allocs/op
		//单协程写，单协程读，锁内分配一个结构 Log{}：
		//BenchmarkMutexRaw-4   	10000000	       150 ns/op	       0 B/op	       0 allocs/op
		//BenchmarkMutexRaw-4   	10000000	       177 ns/op	       0 B/op	       0 allocs/op
		//BenchmarkMutexRaw-4   	10000000	       130 ns/op	       0 B/op	       0 allocs/op
		//16协程操作，锁内分配一个结构 Log{}：
		//BenchmarkMutexRaw-4   	 5000000	       249 ns/op	       0 B/op	       0 allocs/op
		//BenchmarkMutexRaw-4   	10000000	       195 ns/op	       0 B/op	       0 allocs/op
		//BenchmarkMutexRaw-4   	50000000	       100 ns/op	       0 B/op	       0 allocs/op
		//BenchmarkMutexRaw-4   	10000000	       153 ns/op	       0 B/op	       0 allocs/op
		//128协程操作，锁内分配一个结构 Log{}：
		//BenchmarkMutexRaw-4   	 5000000	       411 ns/op	       0 B/op	       0 allocs/op
		//BenchmarkMutexRaw-4   	 3000000	       407 ns/op	       0 B/op	       0 allocs/op

		lock.Lock()
		a := Log{}
		_ = a
		lock.Unlock()
	}
}

//
//

func BenchmarkChanWrap2Send(b *testing.B) {
	ch := NewAsyncChan(1024)
	a := Log{}
	var wg sync.WaitGroup
	wg.Add(127)
	for i := 0; i < 64; i++ {
		go func() {
			wg.Done()
			for {
				for i := 0; i < 64; i++ {
					ch.Read(1, 0)
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
					ch.Write(a)
				}
				runtime.Gosched()
			}
		}()
	}

	wg.Wait()
	for i := 0; i < b.N; i++ { //use b.N for looping
		//单协程写，单协程读：
		//BenchmarkChanWrap2Send-4   	 5000000	       357 ns/op	     273 B/op
		//BenchmarkChanWrap2Send-4   	 5000000	       349 ns/op	     303 B/op	      17 allocs/op
		//16协程操作：
		//BenchmarkChanWrap2Send-4   	 1000000	      3887 ns/op	    2363 B/op	     146 allocs/op
		//BenchmarkChanWrap2Send-4   	  300000	      3837 ns/op	    2216 B/op	     137 allocs/op
		//128协程操作：
		//BenchmarkChanWrap2Send-4   	  200000	     22916 ns/op	   16659 B/op	     746 allocs/op
		//BenchmarkChanWrap2Send-4   	  300000	     21364 ns/op	   15134 B/op	     692 allocs/op
		//BenchmarkChanWrap2Send-4   	 1000000	     33925 ns/op	   17626 B/op	    1094 allocs/op
		//128弱竞争，写入加上 runtime.Gosched()：
		//BenchmarkChanWrap2Send-4   	 2000000	       846 ns/op	     386 B/op	      15 allocs/op
		ch.Write(a)
	}
}

func BenchmarkChanWrap2Recv(b *testing.B) {
	ch := NewAsyncChan(1024)
	a := Log{}
	var wg sync.WaitGroup
	wg.Add(127)
	for i := 0; i < 63; i++ {
		go func() {
			wg.Done()
			for {
				for i := 0; i < 64; i++ {
					ch.Read(1, 0)
				}
				runtime.Gosched()
			}
		}()
	}
	for i := 0; i < 64; i++ {
		go func() {
			wg.Done()
			for {
				for i := 0; i < 64; i++ {
					ch.Write(a)
				}
				runtime.Gosched()
			}
		}()
	}

	wg.Wait()
	for i := 0; i < b.N; i++ { //use b.N for looping
		//单协程写，单协程读：
		//BenchmarkChanWrap2Send-4   	 5000000	       357 ns/op	     273 B/op
		//BenchmarkChanWrap2Send-4   	 5000000	       349 ns/op	     303 B/op	      17 allocs/op
		//16协程操作：
		//BenchmarkChanWrap2Send-4   	 1000000	      3887 ns/op	    2363 B/op	     146 allocs/op
		//BenchmarkChanWrap2Send-4   	  300000	      3837 ns/op	    2216 B/op	     137 allocs/op
		//128协程操作：
		//BenchmarkChanWrap2Send-4   	  200000	     22916 ns/op	   16659 B/op	     746 allocs/op
		//BenchmarkChanWrap2Send-4   	  300000	     21364 ns/op	   15134 B/op	     692 allocs/op
		//BenchmarkChanWrap2Send-4   	 1000000	     33925 ns/op	   17626 B/op	    1094 allocs/op
		//128弱竞争，写入加上 runtime.Gosched()：
		//BenchmarkChanWrap2Recv-4   	 2000000	       813 ns/op	     370 B/op	      15 allocs/op
		ch.Read(1, 0)
	}
}

// chanWrap2
// 二元组运算的实现 max = b>a ? b : a -> max = CondV(b>a, b, a).(a.Type())
func CondV(cond bool, v1 interface{}, v2 interface{}) interface{} {
	if cond {
		return v1
	} else {
		return v2
	}
}

const (
	MIN_CHAN_SIZE = 1
)

const (
	ASYNC_CHAN_STATE_NIL   = 0
	ASYNC_CHAN_STATE_INIT  = 1
	ASYNC_CHAN_STATE_CLOSE = 2
)

type AsyncChanST struct {
	msgChan []interface{}
	size    int
	n       int
	head    int
	tail    int
	mutex   sync.Mutex
	state   int
}

func NewAsyncChan(size int) *AsyncChanST {
	var msg_chan AsyncChanST
	msg_chan.Init(size)
	return &msg_chan
}

func (p *AsyncChanST) Init(size int) {
	if size < MIN_CHAN_SIZE {
		size = MIN_CHAN_SIZE
	}

	p.msgChan = make([]interface{}, size)
	p.size = size
	p.n = 0
	p.head = 0
	p.tail = 0
	p.state = ASYNC_CHAN_STATE_INIT
}

func (p *AsyncChanST) Write(data interface{}) bool {
	result := false

	p.mutex.Lock()

	if p.state == ASYNC_CHAN_STATE_INIT && p.n < p.size {
		p.msgChan[p.tail] = data
		p.tail = (p.tail + 1) % p.size
		p.n += 1
		result = true
	}

	p.mutex.Unlock()

	return result
}

// out_ms 读超时的时间， 单位毫秒 Millisecond
func (p *AsyncChanST) Read(n int, out_ms int) ([]interface{}, bool) {
	result := make([]interface{}, 0, n)

	p.mutex.Lock()
	if ASYNC_CHAN_STATE_INIT != p.state {
		p.mutex.Unlock()
		return result, false
	}

	rn := CondV(p.n >= n, n, p.n).(int)
	result = p.readmsg(rn, result)
	p.mutex.Unlock()

	if rn >= n {
		return result, true
	} else {
		l := n - rn

		// 设置为0表示不超时，设置个很大的时间
		if 0 == out_ms {
			out_ms = 900000000
		}

		// 5ms一次的降低轮询的频率, +4防止不足5ms的没有次数
		//loop := (out_ms + 4) / 5
		//for i := 0; i < loop; i++ {
		//这里没必要用循环Sleep，如果真需要这个功能，
		//可以用官方的chan和select辅助来做停止等待嘛!
		if false {
			//time.Sleep(5 * time.Millisecond)

			p.mutex.Lock()
			if ASYNC_CHAN_STATE_INIT != p.state {
				p.mutex.Unlock()
				return result, false
			}

			rn = CondV(p.n >= l, l, p.n).(int)
			if rn > 0 {
				result = p.readmsg(rn, result)
			}
			p.mutex.Unlock()

			// l = l - rn
			// if l <= 0 {
			// 	return result, true
			// }
		}
	}

	return result, true
}

func (p *AsyncChanST) readmsg(n int, result []interface{}) []interface{} {
	for i := 0; i < n; i++ {
		result = append(result, p.msgChan[p.head])
		p.head = (p.head + 1) % p.size
		p.n -= 1
	}

	return result
}

func (p *AsyncChanST) Close() {
	p.mutex.Lock()
	if ASYNC_CHAN_STATE_INIT == p.state {
		p.Init(p.size)
		p.state = ASYNC_CHAN_STATE_CLOSE
	}

	p.mutex.Unlock()
}

//

//

//

type ChanWrap3 struct {
	queue  []interface{}
	qlen   uint32 //队列中的已写入的内容数量
	qcap   uint32 //队列的容量
	head   uint32
	tail   uint32
	closed uint32

	lock    sync.Mutex
	chRead  chan int //读阻塞，写满后会覆盖，读空后会阻塞，阻塞后监听这个chan
	waitLen uint32   //等待协程的shuliang
}

func NewChanWrap3(cap int) *ChanWrap3 {
	return &ChanWrap3{
		queue:  make([]interface{}, cap),
		qlen:   0,
		qcap:   uint32(cap),
		head:   0,
		tail:   0,
		closed: 0,
		chRead: make(chan int, 8),
	}
}

func (p *ChanWrap3) Send(data interface{}) (full bool, closed bool) {
	p.lock.Lock()
	//fmt.Println("send,now len:", p.qlen)
	if p.closed != 0 {
		closed = true
		p.lock.Unlock()
		return
	}

	//如果队列满了，则覆盖旧数据
	p.queue[p.tail] = data
	p.tail = (p.tail + 1) % p.qcap //尾巴后挪一位

	if p.qlen == p.qcap {
		p.head = (p.head + 1) % p.qcap
		full = true
	} else {
		p.qlen++
	}
	if p.waitLen > 0 {
		select {
		case p.chRead <- 1:
		default:
		}
	}
	p.lock.Unlock()
	return
}

func (p *ChanWrap3) SendN(datas []interface{}) (n int, closed bool) {
	p.lock.Lock()
	if p.closed != 0 {
		closed = true
		p.lock.Unlock()
		return
	}

	_n := uint32(len(datas))

	if p.qcap-p.qlen >= _n {
		p.qlen = p.qlen + _n
	} else if p.qcap < _n {
		_n = p.qcap //最多能写满并覆盖所有旧数据，如果比队列容量多，多出来部分就不写入
		p.qlen = p.qcap
		p.tail = 0
		p.head = 0
	} else if p.qcap-p.qlen < _n {
		p.head = (p.tail + _n) % p.qcap //会发生覆盖,所有head和tail位置会变成一样
		p.qlen = p.qcap
	}

	if _n+p.tail > p.qcap {
		//要分两段copy
		copy(p.queue[p.tail:], datas)
		copy(p.queue, datas[p.qcap-p.tail:])
	} else {
		copy(p.queue[p.tail:], datas)
	}
	p.tail = (p.tail + _n) % p.qcap //尾巴后挪一位

	if p.waitLen > 0 {
		//采用链式激活，每次激活一个请求
		select {
		case p.chRead <- 1:
		default:
		}
	}
	p.lock.Unlock()
	return int(_n), false
}

// out_ms 读超时的时间， 单位毫秒 Millisecond
func (p *ChanWrap3) Recv(n int, block int) (result []interface{}, closed bool) {
	p.lock.Lock()
	//fmt.Println("recv,now len:", p.qlen)
	if p.closed != 0 && p.qlen == 0 {
		closed = true
		p.lock.Unlock()
		return
	}
	if p.qlen == 0 {
		if block <= 0 {
			p.lock.Unlock()
			return
		}
		//监听等待chan，如果被唤醒，且有数据则退出等待，否则继续等待
		p.waitLen++
		for p.qlen == 0 {
			if p.closed != 0 {
				closed = true
				p.lock.Unlock()
				return
			}
			p.lock.Unlock()
			<-p.chRead
			p.lock.Lock()
		}
		p.waitLen--
		if p.waitLen > 0 && p.qlen > uint32(n) { //链式唤醒
			select {
			case p.chRead <- 1:
			default:
			}
		}
	}

	_n := uint32(n)
	if _n > p.qlen {
		_n = p.qlen
	}
	result = make([]interface{}, int(_n))
	if _n+p.head > p.qcap {
		//要分两段copy
		copy(result, p.queue[p.head:])
		copy(result[p.qcap-p.head:], p.queue[:p.head])
	} else {
		copy(result, p.queue[p.head:])
	}
	p.qlen -= _n
	p.head = (p.head + _n) % p.qcap

	//fmt.Println("recv,out, now len:", p.qlen)
	p.lock.Unlock()
	return
}
func (p *ChanWrap3) Close() {
	p.lock.Lock()
	p.closed = 1
	p.lock.Unlock()
	return
}

//

func TestChanWrap3(t *testing.T) {
	var wg sync.WaitGroup
	wg.Add(1)
	ch := NewChanWrap3(1024)
	a := Log{
		LogId: 111,
	}
	for i := 0; i < 1; i++ {
		go func() {
			for {
				msgs, closed := ch.Recv(4, 1)
				t.Logf("1:%v, len:%d", msgs, len(msgs))
				if !closed {
					t.Log("1:", msgs)
					for _, v := range msgs {
						m, ok := v.(Log)
						if !ok || m.LogId != 111 {
							t.Log("!ok--", m.LogId)
						} else {
							t.Log(v)
						}
					}
				} else {
					t.Log("closed")
					wg.Done()
					return
				}
			}
		}()
	}
	for i := 0; i < 16; i++ {
		ch.Send(a)
	}
	ch.Close()
	wg.Wait()
}
func TestChanWrap3SenN(t *testing.T) {
	ch := NewChanWrap3(1024)
	as := []interface{}{
		Log{LogId: 1}, Log{LogId: 2}, Log{LogId: 3}, Log{LogId: 4},
		Log{LogId: 5}, Log{LogId: 6}, Log{LogId: 7}, Log{LogId: 8},
	}

	n, closed := ch.SendN(as)
	if n != len(as) || closed {
		t.Error("err")
	}

	j := uint16(0)
	for i := 0; i < 2; i++ {
		msgs, closed := ch.Recv(4, 1)
		t.Logf("1:%v, len:%d", msgs, len(msgs))
		if closed {
			t.Error("err")
		}
		if len(msgs) != 4 {
			t.Error("err")
		}
		for _, v := range msgs {
			j++
			m, ok := v.(Log)
			if !ok || m.LogId != int64(j) {
				t.Errorf("ok:%v; or m.LogId:%d != j:%d", ok, m.LogId, j)
			} else {
				t.Log(v)
			}
		}
	}
	msgs, closed := ch.Recv(4, 0)
	if len(msgs) != 0 || closed == true {
		t.Error("err:", msgs, "closed:", closed)
	}
	ch.Close()
	msgs, closed = ch.Recv(4, 0)
	if len(msgs) != 0 || closed != true {
		t.Error("err:", msgs, "closed:", closed)
	}
}

// 额外收获，一次interface{}()强制转化要30ns,,,,,!
func BenchmarkChanWrap3Send(b *testing.B) {
	ch := NewChanWrap3(1024)
	a := Log{}
	ia := interface{}(a)
	_ = ia
	as := []interface{}{a}
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
		//单协程写，无读，仅覆盖旧数据,ch.SendN([]interface{}{a})：
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
		//ch.SendN([]interface{}{a})
		ch.Send(ia)
	}
}

func BenchmarkChanWrap3SendN(b *testing.B) {
	ch := NewChanWrap3(1024)
	a := Log{}
	as := make([]Log, 64)
	as1 := []interface{}{
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
	as := make([]interface{}, 64*16)
	for i, _ := range as {
		as[i] = Log{LogId: int64(i % 16)}
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
		//	// if !ok || pkg.LogId != 111 {
		//	// 	b.Errorf("error, ok:%v, pkg.LogId:%d!=111", ok, pkg.LogId)
		//	// }

	}
}
