package channel

import (
	"sync"
)

// ChanN 顾名思义，是一次可以发送，接收N个数据的chan，特性和原生的chan类似
// 通过原生chan实现Recv()的阻塞！写数据只覆盖，不阻塞！
type ChanN[T any] struct {
	queue  []T
	qlen   uint32 //队列中的已写入的内容数量
	qcap   uint32 //队列的容量
	head   uint32
	tail   uint32
	closed uint32

	lock    sync.Mutex
	chRead  chan int //读阻塞，写满后会覆盖，读空后会阻塞，阻塞后监听这个chan
	waitLen uint32   //等待协程的shuliang
}

func NewChanN[T any](cap int) *ChanN[T] {
	if cap == 0 {
		cap = 1
	}
	return &ChanN[T]{
		queue:  make([]T, cap),
		qlen:   0,
		qcap:   uint32(cap),
		head:   0,
		tail:   0,
		closed: 0,
		chRead: make(chan int, 8),
	}
}

// MustSend 超过容量后，扩充容量
func (p *ChanN[T]) MustSend(data T) (closed bool) {
	p.lock.Lock()
	defer p.lock.Unlock()
	if p.closed != 0 {
		closed = true
		return
	}
	if p.qlen >= p.qcap {
		queue := make([]T, 2*p.qcap)

		//要分两段copy
		copy(queue, p.queue[p.head:])
		copy(queue[p.qcap-p.head:], p.queue[:p.head])
		p.queue = queue
		p.qcap = 2 * p.qcap
		p.head = 0
		p.tail = p.qlen

		p.queue[p.tail] = data
		p.tail++
		p.qlen++
		return
	}

	//如果队列满了，则覆盖旧数据
	p.queue[p.tail] = data
	p.tail = (p.tail + 1) % p.qcap //尾巴后挪一位
	p.qlen++

	if p.waitLen > 0 {
		select {
		case p.chRead <- 1:
		default:
		}
	}
	return
}

func (p *ChanN[T]) MustSendN(datas []T) (n int, closed bool) {
	p.lock.Lock()
	defer p.lock.Unlock()
	if p.closed != 0 {
		closed = true
		return
	}

	_n := uint32(len(datas))
	if p.qlen+_n >= p.qcap {
		qcap := (p.qlen + _n) * 2
		queue := make([]T, qcap)

		//要分两段copy
		copy(queue, p.queue[p.head:])
		copy(queue[p.qcap-p.head:], p.queue[:p.head])
		p.queue = queue
		p.qcap = qcap
		p.head = 0
		p.tail = p.qlen

		copy(p.queue[p.tail:], datas)
		p.tail += _n
		p.qlen += _n
		return
	}

	if p.qcap-p.qlen < _n {
		_n = p.qcap - p.qlen
	}
	p.qlen += _n

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
	return int(_n), false
}

func (p *ChanN[T]) Send(data T) (full bool, closed bool) {
	p.lock.Lock()
	if p.closed != 0 || p.qlen >= p.qcap {
		if p.closed != 0 {
			closed = true
		} else {
			full = true
		}
		p.lock.Unlock()
		return
	}

	//如果队列满了，则覆盖旧数据
	p.queue[p.tail] = data
	p.tail = (p.tail + 1) % p.qcap //尾巴后挪一位
	p.qlen++

	if p.waitLen > 0 {
		select {
		case p.chRead <- 1:
		default:
		}
	}
	p.lock.Unlock()
	return
}

func (p *ChanN[T]) SendN(datas []T) (n int, closed bool) {
	p.lock.Lock()
	if p.closed != 0 || p.qlen >= p.qcap {
		if p.closed != 0 {
			closed = true
		} else {
			n = 0 //返回n==0表示队列满，无法再写入
		}
		p.lock.Unlock()
		return
	}

	_n := uint32(len(datas))
	if p.qcap-p.qlen < _n {
		_n = p.qcap - p.qlen
	}
	p.qlen += _n

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
func (p *ChanN[T]) Recv(n int, block int) (result []T, closed bool) {
	//*/
	result = make([]T, n)
	n, closed = p.Read(result, block)
	result = result[:n]
	return //*/
	/*
		if n <= 0 {
			return
		}
		p.lock.Lock()
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
					p.waitLen--
					if p.waitLen > 0 { //链式唤醒
						select {
						case p.chRead <- 1:
						default:
						}
					}
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
		result = make([]T, int(_n))
		if _n+p.head > p.qcap {
			//要分两段copy
			copy(result, p.queue[p.head:])
			copy(result[p.qcap-p.head:], p.queue[:p.head])
		} else {
			copy(result, p.queue[p.head:])
		}
		p.qlen -= _n
		p.head = (p.head + _n) % p.qcap

		p.lock.Unlock()
		return
	//*/
}

// out_ms 读超时的时间， 单位毫秒 Millisecond
func (p *ChanN[T]) Read(datas []T, block int) (ln int, closed bool) {
	n := len(datas)
	if n <= 0 {
		return
	}
	p.lock.Lock()
	// if p.closed != 0 && p.qlen == 0 {
	// 	closed = true
	// 	p.lock.Unlock()
	// 	return
	// }
	if p.qlen == 0 {
		if p.closed != 0 || block <= 0 {
			closed = p.closed != 0
			p.lock.Unlock()
			return
		}
		//监听等待chan，如果被唤醒，且有数据则退出等待，否则继续等待
		p.waitLen++
		for p.qlen == 0 {
			if p.closed != 0 {
				p.waitLen--
				if p.waitLen > 0 { //链式唤醒
					select {
					case p.chRead <- 1:
					default:
					}
				}
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
	//result = make([]T, int(_n))
	if _n+p.head > p.qcap {
		//要分两段copy
		copy(datas, p.queue[p.head:])
		copy(datas[p.qcap-p.head:], p.queue[:p.head])
	} else {
		copy(datas[:_n], p.queue[p.head:])
	}
	p.qlen -= _n
	p.head = (p.head + _n) % p.qcap

	ln = int(_n)
	p.lock.Unlock()
	return
}

func (p *ChanN[T]) Close() {
	p.lock.Lock()
	if p.closed == 1 {
		return
	}
	p.closed = 1
	if p.waitLen > 0 { //链式唤醒
		select {
		case p.chRead <- 1:
		default:
		}
	}
	//是否需要清空队列里的数据？
	p.lock.Unlock()
	return
}
