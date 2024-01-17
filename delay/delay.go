package delay

import (
	"context"
	"sync"
	"time"

	"github.com/lxt1045/utils/log"
)

const (
	defaultPopNSleep = time.Millisecond * 100

	ModeDelay = 0
	ModeWait  = 1
)

type Delay interface {
	Post()
}

// Queue 顾名思义，是一次可以发送，接收N个数据的chan，特性和原生的chan类似
// 通过原生chan实现Recv()的阻塞！写数据只覆盖，不阻塞！
type Queue[T Delay] struct {
	queue  []Data[T]
	qlen   int32  // 队列中的已写入的内容数量
	qcap   uint32 // 队列的容量
	head   uint32 // 队列正在读取的位置,该位置有数据可以直接读
	tail   uint32 // 队列正在写入的位置,该位置没有数据可以直接写
	closed uint32

	wlock    sync.Mutex
	poplock  sync.Mutex
	rLock    sync.RWMutex  // 读锁；读过程中不允许扩容
	popSleep time.Duration // popN 睡眠时间
	// mode      uint8         // popN 睡眠时间

	//由于是延时队列，所以reader也是按时长排序，最长的在最后
	timeWindow int64 // 延时长度, 单位: ns
}

type Data[T Delay] struct {
	deadline int64 // 截止时间, 单位: s
	data     T
}

// New asynPop 表示是否异步弹出
func New[T Delay](initCap int, timeWindow int64, asynPops ...bool) (p *Queue[T]) {
	popSleep := defaultPopNSleep
	if len(asynPops) > 0 && !asynPops[0] {
		popSleep = 0
	}
	return newQueue[T](initCap, timeWindow, nil, popSleep)
}

// NewWaitQueue
func NewWaitQueue[T Delay](initCap int, timeWindow int64, fOK func(t T) bool) (p *Queue[T]) {
	if fOK == nil {
		return
	}
	return newQueue[T](initCap, timeWindow, fOK, defaultPopNSleep)
}

func newQueue[T Delay](initCap int, timeWindow int64, fOK func(t T) bool, popSleep time.Duration) (p *Queue[T]) {
	if initCap == 0 {
		initCap = 1024 * 16
	}
	p = &Queue[T]{
		queue:      make([]Data[T], initCap),
		qlen:       0,
		qcap:       uint32(initCap),
		head:       0,
		tail:       0,
		closed:     0,
		timeWindow: timeWindow,
		popSleep:   popSleep,
	}
	if p.popSleep != 0 {
		go func() {
			for {
				p.pop(fOK)
				time.Sleep(p.popSleep)
			}
		}()
	}
	return
}

// MustSend 超过容量后，扩充容量
func (p *Queue[T]) Push(t T) (closed bool) {
	p.wlock.Lock()
	// defer p.lock.Unlock()
	defer func() {
		if p.popSleep != 0 {
			p.wlock.Unlock()
			return
		}
		deadline := p.queue[p.head].deadline
		p.wlock.Unlock()

		tNow := time.Now().UnixNano()
		if tNow > deadline {
			locked := p.poplock.TryLock()
			if !locked {
				return // 有其他 gorutone 正在执行，直接退出; 保证 popN 不能并发执行，并发执行会破坏 p.qlen ！！！
			}
			defer p.poplock.Unlock()
			p.pop(nil)
		}
	}()

	if p.closed != 0 {
		closed = true
		return
	}
	d := Data[T]{
		deadline: time.Now().UnixNano() + p.timeWindow,
		data:     t,
	}

	// 为了方便遍历，避免写满时 p.tail==p.head，这里永远不能写满
	if p.qlen+1 >= int32(p.qcap) {
		locked := p.rLock.TryLock()
		if !locked {
			// 这里之所以要解锁后重新给 p.lock 加锁，是为了避免死锁，保证 p.rLock 和 p.lock 的加锁顺序
			// 采用退避策略； 注意加锁顺序: p.rLock -> r.lock -> p.lock
			p.wlock.Unlock()
			p.rLock.Lock()
			p.wlock.Lock()
		}
		defer p.rLock.Unlock()
		if p.closed != 0 {
			closed = true
			return
		}
		cap := p.qcap * 2
		if cap > (1 << 30) {
			cap = p.qcap + (1 << 30)
		}
		queue := make([]Data[T], cap)

		//要分两段copy
		rCount := p.qcap - p.head
		copy(queue, p.queue[p.head:])
		copy(queue[rCount:], p.queue[:p.head])

		p.queue = queue
		p.qcap = cap
		p.head = 0
		p.tail = uint32(p.qlen)

		p.queue[p.tail] = d
		p.tail++
		p.qlen++
		return
	}

	//如果队列满了，则覆盖旧数据
	p.queue[p.tail] = d
	p.tail = (p.tail + 1) % p.qcap //尾巴后挪一位
	p.qlen++

	return
}

// pop 只能单进程执行
func (p *Queue[T]) pop(fOK func(t T) bool) {
	p.rLock.RLock()         // 注意加锁顺序: p.rLock -> r.lock -> p.lock
	defer p.rLock.RUnlock() //

	tNow := time.Now().UnixNano()

	p.wlock.Lock()
	tail := p.tail // 统一获取，避免不一致
	head := p.head // head 在读过程中的临时cache，读完再lock后修改 head 减少 lock 占用，以减少与写的冲突
	p.wlock.Unlock()

	ctx := context.TODO()

	rindexMove := 0 // 移动了多少位置
	for {
		// 追上写说明没数据了，不再处理
		if head == tail /*|| rindexMove == int(p.qlen)*/ {
			break
		}
		if fOK == nil {
			deadline := p.queue[head].deadline
			if deadline >= tNow {
				break
			}

			// 超时大于 3秒的话，可能会造成窗口过大，做个告警日志
			if deadline-tNow > 3*int64(time.Second) {
				log.Ctx(ctx).Warn().Caller().Int64("deadline", deadline).
					Int64("p.timeWindow", p.timeWindow).Msg("DelayQueue.popN()")
			}
		} else {
			ok := fOK(p.queue[head].data)
			if !ok {
				break
			}
		}

		d := p.queue[head].data
		d.Post()

		// 移动偏移量
		head = (head + 1) % uint32(p.qcap)
		rindexMove++
	}

	// 最后一个，需要看看是否要移动 p.head 了
	if rindexMove != 0 {
		p.wlock.Lock()
		p.qlen -= int32(rindexMove)
		p.head = head // 必须在两个所得加持下： r.lock.Lock() && p.lock.Lock()
		p.wlock.Unlock()
	}

	return
}

func (p *Queue[T]) Range(do func(t T)) {
	p.rLock.RLock()         // 避免扩容： rIndex 和 p.cap p.tail
	defer p.rLock.RUnlock() // 注意加锁顺序: p.rLock p.lock

	p.wlock.Lock()
	tail := p.tail // 统一获取，避免不一致
	i := p.head
	p.wlock.Unlock()

	for ; i != tail; i = (i + 1) % p.qcap {
		do(p.queue[i].data)
	}
}

func (p *Queue[T]) Close() {
	p.rLock.RLock()         // 注意加锁顺序: p.rLock -> r.lock -> p.lock
	defer p.rLock.RUnlock() //

	p.wlock.Lock()
	defer p.wlock.Unlock()

	if p.closed == 1 {
		return
	}
	p.closed = 1
	return
}
