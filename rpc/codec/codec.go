package codec

import (
	"context"
	stderrs "errors"
	"io"
	"math"
	"reflect"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gogo/protobuf/proto"
	"github.com/lxt1045/errors"
	"github.com/lxt1045/utils/delay"
	"github.com/lxt1045/utils/log"
	"github.com/lxt1045/utils/rpc/base"
)

var (
	ErrUnexpected = errors.NewCode(0, 1, "codec unexpected error")
)

type PassthroughKey struct{}
type LogidKey struct{}

var (
	bufPool = sync.Pool{
		New: func() any {
			return make([]byte, math.MaxUint16)
		},
	}
)

type Msg interface {
	proto.Message
}

// Method 表示一个 struct 的可以调用的方法
type Method interface {
	ReqType() reflect.Type
	RespType() reflect.Type
	NewReq() Msg
	NewResp() Msg
	SvcInvoke(ctx context.Context, req Msg) (resp Msg, err error)
	FuncName() string
}

type Codec struct {
	cancel    context.CancelFunc
	rwc       io.ReadWriteCloser
	tmpCallSN uint32

	respsLock sync.Mutex
	resps     map[uint64]resp
	segments  map[uint64][]byte // 分片
	delay     *delay.Queue[post]

	callers []Method

	streamsLock sync.Mutex
	streams     map[uint64]*Stream

	cliPassKeys []string // 客户端: ctx 透传 key
	svcPassKeys []string // 服务端: ctx 透传 key
}

type resp struct {
	r    Msg
	done chan<- error
}

type post struct {
	key      uint64
	Codec    *Codec
	segments bool
	done     chan error
}

func (d post) Post() {
	if d.segments {
		delete(d.Codec.segments, d.key)
	}

	if d.done != nil {
		d.done <- stderrs.New("timeout")
	}
}

// ctxPassKeys: 作为cli时, ctx 中需要透传给对方的key
func NewCodec(ctx context.Context, cancel context.CancelFunc, rwc io.ReadWriteCloser, callers []Method, ctxPassKeys []string, needHeartbeat bool) (c *Codec, err error) {
	c = &Codec{
		cancel:      cancel,
		rwc:         rwc,
		resps:       make(map[uint64]resp),
		segments:    make(map[uint64][]byte),
		delay:       delay.New[post](64, int64(time.Minute), false),
		streams:     make(map[uint64]*Stream),
		callers:     callers,
		cliPassKeys: ctxPassKeys,
	}
	go c.ReadLoop(ctx)
	if needHeartbeat {
		go c.Heartbeat(ctx)
	}
	return
}

func (c *Codec) Close() (err error) {
	c.streamsLock.Lock()
	defer c.streamsLock.Unlock()

	if c.rwc == nil {
		err = errors.Errorf("has been closed")
		return
	}
	err = c.rwc.Close()
	c.rwc = nil
	c.cancel()
	return
}

func (c *Codec) IsClosed() (yes bool) {
	return c == nil || c.rwc == nil
}

func (c *Codec) ClientCall(ctx context.Context, callID uint16, req, res Msg) (done <-chan error, err error) {
	return c.clientCall(ctx, VerCallReq, callID, 0, req, res)
}

func (c *Codec) StreamCall(ctx context.Context, ver, callID uint16, callSN uint32, req Msg) (err error) {
	yes := func() bool {
		key := respsKey(callSN)
		c.streamsLock.Lock()
		defer c.streamsLock.Unlock()
		stream := c.streams[key]
		return stream != nil
	}()
	if !yes {
		return errors.Errorf("stream has been closed by peer")
	}
	_, err = c.clientCall(ctx, ver, callID, callSN, req, nil)
	return
}

func respsKey(callSN uint32) uint64 {
	return uint64(callSN)
}

func (c *Codec) Heartbeat(ctx context.Context) {
	// return
	tickerHeartbeat := time.NewTicker(time.Duration(time.Second * 100)) // 心跳包; client 发送
	defer func() {
		tickerHeartbeat.Stop()
		log.Ctx(ctx).Info().Caller().Msg("Heartbeat.defer()")
		err := c.Close()
		if err != nil {
			log.Ctx(ctx).Error().Caller().Err(err).Msg("Heartbeat.defer()")
		}
		c.Close()
	}()
	for {
		select {
		case <-ctx.Done():
			return
		case <-tickerHeartbeat.C:
		}

		err := c.SendHeartbeatMsg(ctx)
		// if ErrHasBeenClosed.Is(err) {
		// 	break
		// }
		if err != nil {
			log.Ctx(ctx).Error().Caller().Err(err).Msg("Heartbeat")
			continue
		}
	}
}

func (c *Codec) ReadLoop(ctx context.Context) {
	var err error
	defer func() {
		e := recover()
		if e != nil {
			err = ErrUnexpected.Clonef("recove:%v", e)
		}
		if err != nil {
			log.Ctx(ctx).Error().Caller().Err(err).Msg("defer")
		} else {
			log.Ctx(ctx).Debug().Caller().Err(err).Msg("defer")
		}

		if c.rwc != nil {
			c.SendCloseMsg(ctx)
			c.Close()
		}
	}()

	rbuf := make([]byte, 0, math.MaxUint16)
	for {
		select {
		case <-ctx.Done():
			return // 检查是否已经退出，如果退出则返回
		default:
		}
		ctxDo := ctx

		var header Header
		var bsBody []byte
		header, bsBody, err = ReadPack(ctx, c.rwc, rbuf)
		if err != nil {
			if err == io.EOF || err == io.ErrUnexpectedEOF {
				return
			}
			// err = errors.Errorf(err.Error())
			return
		}

		// 需要分片时
		if header.SegmentCount > 1 {
			key := respsKey(header.CallSN)
			bsOld, ok := c.segments[key]

			if header.SegmentIdx == 0 {
				c.delay.Push(post{Codec: c, key: key, segments: true})
			} else if !ok {
				log.Ctx(ctx).Error().Caller().Interface("header", header).Msg("drop")
				continue
			}

			bsBody = append(bsOld, bsBody...) // 都是再同一个 ClientReadLoop 协程中，无需加锁

			// 不是最后一个分片
			if header.SegmentCount-1 > header.SegmentIdx {
				c.segments[key] = bsBody
				continue
			}
			delete(c.segments, key) // 会不会出现意外而无法清理的情况
		}

		if header.CtxLen != 0 {
			m := base.Ctx{}
			err = proto.Unmarshal(bsBody[:header.CtxLen], &m)
			if err != nil {
				err = ErrUnexpected.WithErr(err)
				return
			}
			if m.LogID > 0 {
				ctxDo = context.WithValue(ctxDo, LogidKey{}, m.LogID)
			}
			if len(c.cliPassKeys) != len(m.Fields) {
				err = ErrUnexpected.Clonef("ctx keys len error, %d,%d", len(c.cliPassKeys), len(m.Fields))
				return
			}
			for i, k := range c.cliPassKeys {
				ctxDo = context.WithValue(ctxDo, k, m.Fields[i])
			}
			bsBody = bsBody[header.CtxLen:]
		}

		switch header.Ver {
		case VerHeartbeat:
			log.Ctx(ctxDo).Trace().Caller().Msg("heartbeat by peer")

		case VerCallReq:
			err = c.VerCallReq(ctxDo, header, bsBody)
			if err != nil {
				log.Ctx(ctxDo).Error().Caller().Err(err).Send()
			}
		case VerCallResp:
			err = c.VerCallResp(ctxDo, header, bsBody)
			if err != nil {
				log.Ctx(ctxDo).Error().Caller().Err(err).Send()
			}
		case VerClose:
			log.Ctx(ctxDo).Debug().Caller().Msg("close by peer")
			return
		case VerCmdReq:
			err = c.VerCmdReq(ctxDo, header, bsBody)
			if err != nil {
				log.Ctx(ctxDo).Error().Caller().Err(err).Send()
			}
		case VerCmdResp:
			err = c.VerCallResp(ctxDo, header, bsBody)
			if err != nil {
				log.Ctx(ctxDo).Error().Caller().Err(err).Send()
			}
		case VerStreamReq:
			err = c.VerStreamReq(ctxDo, header, bsBody)
			if err != nil {
				log.Ctx(ctxDo).Error().Caller().Err(err).Send()
			}
		case VerStreamResp:
			err = c.VerStreamResp(ctxDo, header, bsBody)
			if err != nil {
				log.Ctx(ctxDo).Error().Caller().Err(err).Send()
			}
		default:
			log.Ctx(ctxDo).Error().Caller().Interface("header", header).Send()
		}
	}
}

// ReadPack 读一个裸消息
func ReadPack(ctx context.Context, r io.Reader, buf []byte) (header Header, bsBody []byte, err error) {
	n, err := io.ReadFull(r, buf[:2]) // 先读长度
	if err != nil {
		if err == io.EOF || err == io.ErrUnexpectedEOF {
			return
		}
		err = errors.Errorf("%d: %s", n, err.Error())
		return
	}
	if n != 2 {
		err = errors.Errorf("n != 2,%d", n)
		return
	}
	lenNeed := ParseHeaderLen(buf[:2])
	if cap(buf) < int(lenNeed) {
		bufnew := make([]byte, lenNeed)
		bufnew[0], bufnew[1] = buf[0], buf[1] // copy(bufnew, bsHeader[:2])
		buf = bufnew
	}
	n, err = io.ReadFull(r, buf[2:lenNeed]) // 读取剩下的部分
	if err != nil {
		err = errors.Errorf(err.Error())
		return
	}
	if n != int(lenNeed-2) {
		err = errors.Errorf("n != 2,%d", n)
		return
	}

	header, lHeader := ParseHeader(buf[:HeaderSize])
	bsBody = buf[lHeader:lenNeed]
	return
}

func (c *Codec) SendCloseMsg(ctx context.Context) (err error) {
	wbuf := make([]byte, HeaderSize)
	h := Header{
		Ver:          VerClose,
		CtxLen:       0,
		Len:          uint16(len(wbuf)),
		SegmentCount: 0,
		SegmentIdx:   0,
		CallID:       0,
		CallSN:       0,
	}
	h.Format(wbuf)
	if c.rwc == nil {
		err = errors.Errorf("has been closed")
		return
	}
	c.rwc.Write(wbuf)
	return
}

var ErrHasBeenClosed = errors.NewCode(0, 0x1111, "has been closed")

func (c *Codec) SendHeartbeatMsg(ctx context.Context) (err error) {
	wbuf := make([]byte, HeaderSize)
	h := Header{
		Ver:          VerHeartbeat,
		CtxLen:       0,
		Len:          uint16(len(wbuf)),
		SegmentCount: 0,
		SegmentIdx:   0,
		CallID:       0,
		CallSN:       0,
	}
	h.FormatRaw(wbuf)
	if c.rwc == nil {
		err = ErrHasBeenClosed.Clone("SendHeartbeatMsg")
		return
	}
	_, err = c.rwc.Write(wbuf)
	return
}

func (c *Codec) SendCmd(ctx context.Context, req *base.CmdReq) (res *base.CmdRsp, err error) {
	res = &base.CmdRsp{}
	if base.CmdReq_CallIDs == req.Cmd {
		if len(req.Fields) != 0 {
			err = ErrUnexpected.Clonef("CmdReq_CallIDs's Fields must be empty")
			return
		}
		req.Fields = c.cliPassKeys
	}
	done, err := c.clientCall(ctx, VerCmdReq, 0, 0, req, res)
	if err != nil {
		return
	}
	if done != nil {
		err = <-done
	}
	return
}

func (c *Codec) clientCall(ctx context.Context, ver, callID uint16, callSN uint32, req, res Msg) (done <-chan error, err error) {
	if callSN == 0 {
		callSN = atomic.AddUint32(&c.tmpCallSN, 1)
	}

	// 先安排好返回路径，再发送请求
	if res != nil {
		func() {
			ch := make(chan error, 1)
			done = ch
			res := resp{
				r:    res,
				done: ch,
			}
			key := respsKey(callSN)
			c.delay.Push(post{Codec: c, key: key, done: ch}) // 写入超时队列

			c.respsLock.Lock()
			defer c.respsLock.Unlock()
			c.resps[key] = res
		}()
	}

	err = c.SendMsg(ctx, ver, callID, callSN, req)
	return
}

func (c *Codec) SendMsg(ctx context.Context, ver, callID uint16, callSN uint32, msg Msg) (err error) {
	bs := bufPool.Get().([]byte)
	wbuf := bs[:HeaderSize]      // 给 Header 预留足够的内存
	buf := proto.NewBuffer(wbuf) //
	ctxlen := uint16(0)
	{
		m := base.Ctx{}
		m.LogID, _ = ctx.Value(LogidKey{}).(uint64)
		if len(c.cliPassKeys) > 0 {
			for _, k := range c.cliPassKeys {
				v, _ := ctx.Value(k).(string)
				m.Fields = append(m.Fields, v)
			}
		}
		if m.LogID > 0 || len(c.cliPassKeys) > 0 {
			l := len(buf.Bytes())
			err = buf.Marshal(&m)
			if err != nil {
				bufPool.Put(bs)
				return
			}
			if len(buf.Bytes()) > math.MaxUint16 {
				bufPool.Put(bs)
				err = errors.Errorf("the length of the ctx value exceeds %d: %d", math.MaxUint16-l, len(buf.Bytes()))
				return
			}
			ctxlen = uint16(len(buf.Bytes()) - l)
		}
	}
	if msg != nil {
		err = buf.Marshal(msg)
		if err != nil {
			bufPool.Put(bs)
			return
		}
		wbuf = buf.Bytes()
	}

	err = c.Send(ctx, wbuf, ver, callID, ctxlen, callSN)
	bufPool.Put(wbuf)
	return
}

func (c *Codec) Send(ctx context.Context, wbuf []byte, ver, callID, ctxlen uint16, callSN uint32) (err error) {
	defer func() {
		if e := recover(); e != nil {
			err = errors.Errorf("%+v", e)
		}
	}()
	if len(wbuf) <= math.MaxUint16 {
		h := Header{
			Ver:          ver,
			CtxLen:       ctxlen,
			Len:          uint16(len(wbuf)),
			SegmentCount: 0,
			SegmentIdx:   0,
			CallID:       callID,
			CallSN:       callSN,
		}
		h.FormatCall(wbuf)
		if c.rwc == nil {
			err = ErrHasBeenClosed
			return
		}
		_, err = c.rwc.Write(wbuf)
		return
	}

	wbuf0, lBodyAll, lBodyOne := wbuf, len(wbuf)-HeaderSize, int(math.MaxUint16-HeaderSize)
	idx, maxIdx := uint16(0), uint16(lBodyAll/lBodyOne)
	for ; idx < maxIdx; idx++ {
		h := Header{
			Ver:          ver,
			CtxLen:       ctxlen,
			Len:          math.MaxUint16, // uint16(len(wbuf0)),
			SegmentCount: maxIdx + 1,
			SegmentIdx:   idx,
			CallID:       callID,
			CallSN:       callSN,
		}
		h.FormatCall(wbuf0)
		if c.rwc == nil {
			err = errors.Errorf("has been closed")
			return
		}
		_, err = c.rwc.Write(wbuf0[:math.MaxUint16]) // 原子写，内部有锁
		if err != nil {
			return
		}
		wbuf0 = wbuf0[lBodyOne:] // 这样 header 部分还有
	}

	// 刚好发完，没有遗漏
	if len(wbuf0) <= HeaderSize {
		return
	}

	// 最后一个数据包
	h := Header{
		Ver:          ver,
		CtxLen:       ctxlen,
		Len:          uint16(len(wbuf0)),
		SegmentCount: maxIdx + 1,
		SegmentIdx:   idx,
		CallID:       callID,
		CallSN:       callSN,
	}
	h.FormatCall(wbuf0)
	if c.rwc == nil {
		err = errors.Errorf("has been closed")
		return
	}
	_, err = c.rwc.Write(wbuf0)
	if err != nil {
		err = errors.New(err.Error())
	}
	return
}
