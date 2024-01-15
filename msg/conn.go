package msg

import (
	"context"
	"crypto/tls"
	"io"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/klauspost/compress/zstd"
	"github.com/lxt1045/errors"
	"github.com/lxt1045/utils/config"
	"github.com/lxt1045/utils/gid"
	"github.com/lxt1045/utils/log"
	"github.com/lxt1045/utils/msg/pb"
)

type Conn struct {
	ID   int64
	Conn net.Conn

	wLock  sync.Mutex // zstd 不是线程安全的，所以需要加锁
	rLock  sync.Mutex
	writer *zstd.Encoder // 需要注意保护，非协程安全
	reader *zstd.Decoder // 需要注意保护，非协程安全

	ConnState tls.ConnectionState // 含 ServerName，可以用于验证

	// 用于call
	callCache     map[int64]*call // rpc Call缓存： map[sn]call
	callCacheLock sync.Mutex
	Status        int32

	Owner interface{} // 拥有者，可能是agent也可能是broker； 方便callee使用
}

const (
	ConnStatusClosed         int32 = -1 // 连接以关闭
	ConnStatusAuthenticating int32 = 0  // 默认状态
	ConnStatusStarting       int32 = 1  // 认证中
	ConnStatusConnected      int32 = 2  // 认证完成后
)

var (
	ErrDecoderClosed = zstd.ErrDecoderClosed
	ErrUnexpectedEOF = io.ErrUnexpectedEOF
)

func (c *Conn) Read(data []byte) (n int, err error) {
	// if c.Status == ConnStatusClosed {
	// 	err = errno.UtilsConnClosed.Clone()
	// 	return
	// }
	// return c.Conn.Read(data)
	c.rLock.Lock()
	defer c.rLock.Unlock()
	n, err = c.reader.Read(data)
	if err != nil {
		if errors.Is(zstd.ErrDecoderClosed, err) {
			err = ErrDecoderClosed
		} else if err == io.ErrUnexpectedEOF || err == io.EOF {
			err = ErrUnexpectedEOF
		} else {
			err = errors.Errorf("err:%v, ip:%s", err.Error(), c.Conn.RemoteAddr())
		}
	}
	return
}
func (c *Conn) Write(data []byte) (n int, err error) {
	// if c.Status == ConnStatusClosed {
	// 	err = errno.UtilsConnClosed.Clone()
	// 	return
	// }
	// return c.Conn.Write(data)
	c.wLock.Lock()
	defer c.wLock.Unlock()
	n, err = c.writer.Write(data)
	if err != nil {
		err = errors.Errorf(err.Error())
		return
	}
	if c.directFlush {
		err = c.writer.Flush()
		return
	}

	c.wFlushCount++
	if c.wFlushCount > 100 {
		err = c.writer.Flush()
		c.wFlushCount = 0
		c.wFlushTime = time.Now().UnixNano()
	}

	return
}

func (c *Conn) Close() (err error) {
	defer func() {
		e := recover() // maybe panic
		if e != nil {
			err = errors.Errorf("recover:%+v", e)
		}
	}()

	atomic.StoreInt32(&c.Status, ConnStatusClosed)

	err1 := c.Conn.Close() // conn 关闭后， c.reader 和 c.writer 的阻塞点就会及时返回
	if err1 != nil {
		err = err1
	}
	if c.reader != nil {
		func() {
			c.rLock.Lock()
			defer c.rLock.Unlock()
			c.reader.Close()
		}()
	}

	if c.writer != nil {
		func() {
			c.wLock.Lock()
			defer c.wLock.Unlock()
			err = c.writer.Close()
		}()
	}

	return
}

func NewConn(c net.Conn, id int64, conf *config.Conn, owner interface{}) (conn *Conn, err error) {
	conn = &Conn{
		ID:          id,
		Conn:        c,
		directFlush: conf.FlushTime == -1,
		callCache:   make(map[int64]*call),
		Owner:       owner,
	}

	cState, ok := conn.Conn.(interface {
		ConnectionState() tls.ConnectionState
	})
	if ok {
		conn.ConnState = cState.ConnectionState()
	}

	// return
	readConcurrency := conf.ReadConcurrency
	if readConcurrency <= 0 {
		readConcurrency = 1
	}

	wops := []zstd.EOption{
		zstd.WithEncoderLevel(zstd.SpeedFastest),
	}
	if conf.WriteConcurrency > 0 {
		wops = append(wops, zstd.WithEncoderConcurrency(conf.WriteConcurrency)) // default:  8 << 20
	}
	if conf.WriteWindow > 0 {
		wops = append(wops, zstd.WithWindowSize(1<<conf.WriteWindow)) // default:  8 << 20
	}
	conn.writer, err = zstd.NewWriter(conn.Conn, wops...)
	if err != nil {
		err = errors.Errorf(err.Error())
		return
	}

	rops := []zstd.DOption{}
	if conf.ReadConcurrency > 0 {
		rops = append(rops, zstd.WithDecoderConcurrency(1<<conf.ReadConcurrency))
	}
	if conf.ReadWindow > 0 {
		rops = append(rops, zstd.WithDecoderMaxWindow(1<<conf.ReadWindow))
	}
	conn.reader, err = zstd.NewReader(conn.Conn, rops...)
	if err != nil {
		err = errors.Errorf(err.Error())
		return
	}
	return
}

// Ack 有增量时给对端发送全量ACK，可以容忍ACK丢包
func Acks(ctx context.Context, ch chan *Conn, per time.Duration) {
	defer func() {
		e := recover()
		if e != nil {
			log.Ctx(ctx).Error().Interface("recover", e).Msg("msg.Acks")
		}
	}()
	logid := gid.GetGID()
	ctx, _ = log.WithLogid(ctx, logid)
	conns := make([]*Conn, 0, 2048)
	tLast := time.Now()
	wbuf := make([]byte, 0, 0x7fff)
	var err error
	for {
		tDiff := per - time.Since(tLast)
		if tDiff > time.Millisecond*10 {
			time.Sleep(tDiff)
		}
		tLast = time.Now()

		for more := true; more; {
			select {
			case <-ctx.Done():
				log.Ctx(ctx).Info().Caller().Msg("Ack() loop done")
				return
			case conn := <-ch:
				conns = append(conns, conn)
			default:
				more = false
			}
		}

		tsNow := tLast.UnixNano()
		for i := 0; i < len(conns); i++ {
			conn := conns[i]
			if atomic.LoadInt32(&conn.Status) == ConnStatusClosed {
				conns = append(conns[:i], conns[i+1:]...)
				i--
				// atomic.StoreInt32(&conn.Closed, 1)
				continue
			}
			if conn.wFlushCount >= 0 && tsNow-conn.wFlushTime > int64(per) {
				func() {
					if !conn.wLock.TryLock() {
						return
					}
					defer conn.wLock.Unlock()
					if conn.wFlushCount <= 0 || tsNow-conn.wFlushTime <= int64(per) {
						return
					}
					err := conn.writer.Flush()
					if err == nil {
						conn.wFlushCount = 0
						conn.wFlushTime = time.Now().UnixNano()
					}
				}()
			}

			// ACK
			ack := atomic.LoadInt64(&conn.NeedACKCount)
			if ack-conn.lastNeedACKCount <= 0 {
				continue
			}

			msg := &pb.Ack{
				Count: ack,
			}

			wbuf, err = conn.SendMsg(ctx, msg, logid, wbuf)
			if err != nil {
				log.Ctx(ctx).Error().Caller().Err(err).Interface("conn", conn).Msg("ack error")
				continue
			}
			conn.lastNeedACKCount = ack

			log.Ctx(ctx).Trace().Caller().Int64("id", conn.ID).Int64("ack", ack).
				Int64("ack-diff", ack-conn.lastNeedACKCount).Msg("ack")
		}
	}
}

// Ack 有增量时给对端发送全量ACK，可以容忍ACK丢包
func Ack(ctx context.Context, conn *Conn, cancel func(), needACKCount *int64, per time.Duration) {
	defer func() {
		e := recover()
		if e != nil {
			log.Ctx(ctx).Error().Interface("recover", e).Msg("msg.Ack")
		}
		cancel()
	}()

	logid := gid.GetGID()
	ctx, _ = log.WithLogid(ctx, logid)
	var lastNeedACKCount int64
	wbuf := make([]byte, 0, 0x7fff)
	var err error
	for {
		time.Sleep(per)

		select {
		case <-ctx.Done():
			log.Ctx(ctx).Info().Caller().Msg("Ack() loop done")
			return
		default:
		}

		ack := atomic.LoadInt64(needACKCount)
		if ack-lastNeedACKCount <= 0 {
			continue
		}

		msg := &pb.Ack{
			Count: ack,
		}

		wbuf, err = conn.SendMsg(ctx, msg, logid, wbuf[:0])
		if err != nil {
			log.Ctx(ctx).Error().Caller().Err(err).Msg("ack error")
			return
		}

		log.Ctx(ctx).Info().Caller().Int64("ack", ack).Int64("ack-diff", ack-lastNeedACKCount).Msg("ack")
		lastNeedACKCount = ack
	}
}

type Handler func(ctx context.Context, h Header, bs []byte) (Msg, error)

func MsgHandler(handler func(context.Context, Msg) (Msg, error)) Handler {
	return func(ctx context.Context, h Header, bs []byte) (resp Msg, err error) {
		req, err := ParseMsg(ctx, h, bs[HeaderSize:])
		if err != nil {
			return
		}

		resp, err = handler(ctx, req)
		if err != nil {
			return
		}
		return
	}
}

func (conn *Conn) ReadLoop(ctx context.Context, handler Handler) {
	rbuf := make([]byte, 0, 0x7fff*2) // 读缓存
	wbuf := make([]byte, 0, 0x7fff)   // 写缓存
	var err error
	var header Header
	// conn.directFlush = true

	// 读已经close的zstd可能回panic
	defer func() {
		e := recover()
		if e != nil {
			log.Ctx(ctx).Error().Interface("recover", e).Msg("ReadLoop")
		}
	}()

	for {
		select {
		case <-ctx.Done():
			log.Ctx(ctx).Info().Caller().Msg("ReadLoop Done")
			return
		default:
		}

		ctxInner := context.Background()
		header, rbuf, err = ReadPack(ctxInner, conn, rbuf[:0])
		if err != nil {
			if errors.Is(ErrDecoderClosed, err) || errors.Is(ErrUnexpectedEOF, err) {
				log.Ctx(ctxInner).Debug().Caller().Err(err).Send()
			} else {
				log.Ctx(ctxInner).Warn().Caller().Err(err).Send()
			}
			return
		}
		ctxInner, _ = log.WithLogid(ctxInner, header.LogID)

		// call step
		if t := pb.TYPE(header.Type); t == pb.TYPE_CALL_REQ {
			err := conn.callReq(ctxInner, rbuf[HeaderSize:])
			if err != nil {
				log.Ctx(ctxInner).Error().Caller().Err(err).Send()
				return
			}
			continue
		} else if t == pb.TYPE_CALL_RESP {
			err := conn.callResp(ctxInner, rbuf[HeaderSize:])
			if err != nil {
				log.Ctx(ctxInner).Error().Caller().Err(err).Send()
				return
			}
			continue
		}

		wbuf, err = do(ctxInner, conn, header, rbuf, wbuf[:0], handler)
		if err != nil {
			log.Ctx(ctxInner).Error().Caller().Err(err).Send()
			return
		}
	}
}

func do(ctx context.Context, conn *Conn, header Header, rbuf, wbuf []byte, handler Handler) (out []byte, err error) {
	if handler == nil {
		return
	}

	logid := int64(header.LogID)
	if logid == 0 {
		logid = gid.GetGID()
	}
	ctx, _ = log.WithLogid(ctx, logid)

	var resp Msg
	start := time.Now()
	defer func() {
		e := recover()
		loss := int64(time.Since(start))
		l := log.DeferLogger(ctx, loss, err, e)
		if resp != nil {
			l = l.Interface("resp", resp)
		}
		// l.Caller().Interface("header", header).Msg("end")
	}()

	resp, err = handler(ctx, header, rbuf)
	if err != nil {
		return
	}
	if resp != nil {
		wbuf, err = conn.SendMsg(ctx, resp, logid, wbuf)
		if err != nil {
			return
		}
	}

	return
}

// SetReadWriteBuff 设置 conn 读写缓存的大小
func SetReadWriteBuff(ctx context.Context, conn net.Conn, read, write int) (err error) {
	netConn, ok := conn.(interface {
		NetConn() net.Conn
	})
	if ok {
		connTCP := netConn.NetConn()
		setBuffer, ok := connTCP.(interface {
			SetReadBuffer(bytes int) error
			SetWriteBuffer(bytes int) error
		})
		if ok {
			if read != 0 {
				err = setBuffer.SetReadBuffer(read)
				if err != nil {
					log.Ctx(ctx).Error().Err(err).Send()
				}
			}
			if write != 0 {
				err = setBuffer.SetWriteBuffer(write)
				if err != nil {
					log.Ctx(ctx).Error().Err(err).Send()
				}
			}
		}
	}
	return
}
