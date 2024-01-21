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
	"github.com/lxt1045/utils/msg/rpc/base"
)

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

type Caller interface {
	ReqType() reflect.Type
	RespType() reflect.Type
	NewReq() Msg
	NewResp() Msg
	SvcInvoke(ctx context.Context, req Msg) (resp Msg, err error)
}

type Codec struct {
	rwc       io.ReadWriteCloser
	tmpCallSN uint32

	respsLock sync.Mutex
	resps     map[uint64]resp
	segments  map[uint64][]byte // 分片
	delay     *delay.Queue[post]

	callIDsLock sync.Mutex
	callIDs     []string

	streamsLock sync.Mutex
	streams     map[uint64]*Stream
}

type resp struct {
	r    Msg
	done chan<- error
}

type post struct {
	key      uint64
	Codec    *Codec
	resps    bool
	segments bool
}

func (d post) Post() {
	if d.segments {
		delete(d.Codec.segments, d.key)
	}

	if d.resps {
		old := func() (old resp) {
			d.Codec.respsLock.Lock()
			defer d.Codec.respsLock.Unlock()
			old = d.Codec.resps[d.key]
			delete(d.Codec.resps, d.key)
			return
		}()
		if old.done != nil {
			old.done <- stderrs.New("timeout")
		}
	}
}

func NewCodec(ctx context.Context, rwc io.ReadWriteCloser) (c *Codec, err error) {
	c = &Codec{
		rwc:      rwc,
		resps:    make(map[uint64]resp),
		segments: make(map[uint64][]byte),
		delay:    delay.New[post](64, int64(time.Minute), false),
		streams:  make(map[uint64]*Stream),
	}
	return
}

func (c *Codec) Close() (err error) {
	err = c.rwc.Close()
	return
}

func (c *Codec) SetCallIDs(callIDs []string) {
	c.callIDsLock.Lock()
	defer c.callIDsLock.Unlock()
	c.callIDs = callIDs
}

func (c *Codec) ClientCall(ctx context.Context, channel, callID uint16, req, res Msg) (done <-chan error, err error) {
	return c.clientCall(ctx, VerCallReq, channel, callID, 0, req, res)
}

func (c *Codec) StreamCall(ctx context.Context, ver, channel, callID uint16, callSN uint32, req Msg) (err error) {
	_, err = c.clientCall(ctx, ver, channel, callID, callSN, req, nil)
	return
}

func respsKey(callSN uint32) uint64 {
	return uint64(callSN)
}

func (c *Codec) ReadLoop(ctx context.Context, fNewCaller func(callID uint16) Caller) {
	var err error
	defer func() {
		e := recover()
		if e != nil {
			err = errors.Errorf("recove:%v", e)
		}
		if err != nil {
			log.Ctx(ctx).Error().Err(err).Caller().Msg("defer")
		} else {
			log.Ctx(ctx).Debug().Caller().Msg("defer")
		}

		if c.rwc != nil {
			c.SendCloseMsg(ctx)
			c.Close()
		}
	}()

	rbuf := make([]byte, 0, math.MaxUint16)
	for {
		header, bsBody, err1 := ReadPack(ctx, c.rwc, rbuf)
		if err1 != nil {
			if err1 == io.EOF || err1 == io.ErrUnexpectedEOF {
				return
			}
			err = err1
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
			delete(c.segments, key)
		}

		switch header.Ver {
		case VerCallReq:
			err = c.VerCallReq(ctx, header, bsBody, fNewCaller)
			if err != nil {
				return
			}
		case VerCallResp:
			err = c.VerCallResp(ctx, header, bsBody)
			if err != nil {
				return
			}
		case VerClose:
			log.Ctx(ctx).Debug().Caller().Msg("close by peer")
			return
		case VerCmdReq:
			err = c.VerCmdReq(ctx, header, bsBody)
			if err != nil {
				return
			}
		case VerCmdResp:
			err = c.VerCallResp(ctx, header, bsBody)
			if err != nil {
				return
			}
		case VerStreamReq:
			err = c.VerStreamReq(ctx, header, bsBody, fNewCaller)
			if err != nil {
				return
			}
		case VerStreamResp:
			err = c.VerStreamResp(ctx, header, bsBody)
			if err != nil {
				return
			}
		default:
		}
	}
}

// ReadPack 读一个裸消息
func ReadPack(ctx context.Context, r io.Reader, buf []byte) (header Header, bsBody []byte, err error) {
	bsHeader := buf[:HeaderSize]
	n, err := io.ReadFull(r, bsHeader[:2]) // 先读长度
	if err != nil {
		if err != io.EOF || err != io.ErrUnexpectedEOF {
			return
		}
		err = errors.Errorf(err.Error())
		return
	}
	if n != 2 {
		err = errors.Errorf("n != 2,%d", n)
		return
	}
	lenNeed := ParseHeaderLen(bsHeader)
	if cap(buf) < int(lenNeed) {
		bufnew := make([]byte, lenNeed)
		copy(bufnew, bsHeader[:2])
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

	header, lHeader := ParseHeader(bsHeader)
	bsBody = buf[lHeader:lenNeed]
	return
}

func (c *Codec) SendCloseMsg(ctx context.Context) (err error) {
	wbuf := make([]byte, HeaderSize)
	h := Header{
		Ver:          VerClose,
		Channel:      0,
		Len:          uint16(len(wbuf)),
		SegmentCount: 0,
		SegmentIdx:   0,
		CallID:       0,
		CallSN:       0,
	}
	h.FormatCall(wbuf)
	c.rwc.Write(wbuf)
	return
}

func (c *Codec) SendCmd(ctx context.Context, channel uint16, req *base.CmdReq) (res *base.CmdRsp, err error) {
	res = &base.CmdRsp{}
	done, err := c.clientCall(ctx, VerCmdReq, channel, 0, 0, req, res)
	if err != nil {
		return
	}
	if done != nil {
		err = <-done
	}
	return
}

func (c *Codec) clientCall(ctx context.Context, ver, channel, callID uint16, callSN uint32, req, res Msg) (done <-chan error, err error) {
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
			c.delay.Push(post{Codec: c, key: key, resps: true}) // 写入超时队列

			c.respsLock.Lock()
			defer c.respsLock.Unlock()
			c.resps[key] = res
		}()
	}

	err = c.SendMsg(ctx, ver, channel, callID, callSN, req)
	if err != nil {
		return
	}
	return
}
func (c *Codec) SendMsg(ctx context.Context, ver, channel, callID uint16, callSN uint32, msg Msg) (err error) {
	bs := bufPool.Get().([]byte)
	wbuf := bs[:HeaderSize]      // 给 Header 预留足够的内存
	buf := proto.NewBuffer(wbuf) //
	if msg != nil {
		err = buf.Marshal(msg)
		if err != nil {
			bufPool.Put(bs)
			return
		}
		wbuf = buf.Bytes()
	}
	defer bufPool.Put(wbuf)
	return c.Send(ctx, wbuf, ver, channel, callID, callSN)
}

func (c *Codec) Send(ctx context.Context, wbuf []byte, ver, channel, callID uint16, callSN uint32) (err error) {
	if len(wbuf) <= math.MaxUint16 {
		h := Header{
			Ver:          ver,
			Channel:      channel,
			Len:          uint16(len(wbuf)),
			SegmentCount: 0,
			SegmentIdx:   0,
			CallID:       callID,
			CallSN:       callSN,
		}
		h.FormatCall(wbuf)
		_, err = c.rwc.Write(wbuf)
		return
	}

	Count, idx := uint16(len(wbuf)/math.MaxUint16), uint16(0)
	wbuf0 := wbuf
	for i := uint16(0); i < Count; i, idx = i+math.MaxUint16, idx+1 {
		h := Header{
			Ver:          ver,
			Channel:      channel,
			Len:          uint16(len(wbuf0)),
			SegmentCount: Count,
			SegmentIdx:   idx,
			CallID:       callID,
			CallSN:       callSN,
		}
		h.FormatCall(wbuf0)
		_, err = c.rwc.Write(wbuf0[:math.MaxUint16]) // 原子写，内部有锁
		if err != nil {
			return
		}
		wbuf0 = wbuf0[math.MaxUint16:]
	}
	// 最后一个数据包
	h := Header{
		Ver:          ver,
		Channel:      channel,
		Len:          uint16(len(wbuf0)),
		SegmentCount: Count,
		SegmentIdx:   idx,
		CallID:       callID,
		CallSN:       callSN,
	}
	h.FormatCall(wbuf0)
	_, err = c.rwc.Write(wbuf0)
	return
}
