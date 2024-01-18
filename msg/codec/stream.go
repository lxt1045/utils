package codec

import (
	"context"
	"io"
	"reflect"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gogo/protobuf/proto"
	"github.com/lxt1045/errors"
	"github.com/lxt1045/utils/log"
	"github.com/lxt1045/utils/msg/call/base"
)

type Stream struct {
	codec   *Codec
	channel uint16
	callID  uint16
	callSN  uint32
	cache   []Msg
	l       sync.Mutex
	c       chan struct{}

	caller         Caller
	connectAt      int64
	bSvc           bool
	bSvcStreamMode bool
	bClosed        bool
	bOld           bool
}

func (s *Stream) Close() {
	// 1. 先发送关闭请求

	// 2. 再删除
	c := s.codec

	key := respsKey(s.channel, s.callSN)
	c.streamsLock.Lock()
	defer c.streamsLock.Unlock()
	delete(c.streams, key)

	// 3. 关闭标志
	s.bClosed = true
}

func (s *Stream) Method() (method string) {
	s.codec.callIDsLock.Lock()
	defer s.codec.callIDsLock.Unlock()
	return s.codec.callIDs[s.callID]
}

func (s *Stream) Recv(ctx context.Context) (resp Msg, err error) {
	recv := func() (req Msg) {
		s.l.Lock()
		defer s.l.Unlock()
		if len(s.cache) > 0 {
			req = s.cache[0]
			s.cache = s.cache[1:]
			select {
			case s.c <- struct{}{}:
			default:
			}
			return
		}
		return
	}

	for {
		resp = recv()
		if resp != nil {
			return
		}
		if s.bClosed {
			err = io.EOF
			return
		}
		<-s.c
	}
}

func (s *Stream) write(m Msg) (err error) {
	s.l.Lock()
	defer s.l.Unlock()
	if len(s.cache) > 10240 {
		err = errors.Errorf("The cache size exceeds the limit:%d", len(s.cache))
		return
	}
	s.cache = append(s.cache, m)
	select {
	case s.c <- struct{}{}:
	default:
	}
	return
}

func (s *Stream) Send(ctx context.Context, req Msg) (err error) {
	ver := uint16(VerStreamReq)
	typ := s.caller.ReqType()
	if s.bSvc {
		ver = VerStreamResp
		typ = s.caller.RespType()
	}
	if t := reflect.TypeOf(req); req != nil && t.Elem() != typ {
		err = errors.Errorf("req type error: should be %s, not %s", typ.String(), t.Elem().String())
		return
	}

	err = s.codec.StreamCall(ctx, ver, 0, s.callID, s.callSN, req)
	if err != nil {
		return
	}
	return
}

func (c *Codec) Stream(ctx context.Context, channel uint16, callID uint16, caller Caller) (stream *Stream, err error) {
	stream = &Stream{
		codec:     c,
		callID:    callID,
		c:         make(chan struct{}),
		connectAt: time.Now().Unix(),
		caller:    caller,
	}
	func() {
		c.streamsLock.Lock()
		defer c.streamsLock.Unlock()
		for {
			callSN := atomic.AddUint32(&c.tmpCallSN, 1)
			key := respsKey(0, callSN)
			if _, ok := c.streams[key]; ok {
				continue
			}
			stream.callSN = callSN
			c.streams[key] = stream
			return
		}
	}()
	req := &base.CmdReq{Cmd: base.CmdReq_Stream}
	res := &base.CmdRsp{}
	done, err := c.clientCall(ctx, VerCmdReq, channel, 0, stream.callSN, req, res)
	if err != nil {
		return
	}
	if done != nil {
		err = <-done
	}
	return
}

type ctxStreamKey struct{}

func GetStream(ctx context.Context) (stream *Stream) {
	s := ctx.Value(ctxStreamKey{})
	if s == nil {
		return
	}
	stream, ok := s.(*Stream)
	if !ok {
		log.Ctx(ctx).Info().Caller().Msg("unkonw error")
	}
	stream.bSvcStreamMode = true // 开始启用
	return
}

func (c *Codec) VerStreamReq(ctx context.Context, header Header, bsBody []byte, fNewCaller func(callID uint16) Caller) (err error) {
	stream := func() (stream *Stream) {
		key := respsKey(header.Channel, header.CallSN)
		c.streamsLock.Lock()
		defer c.streamsLock.Unlock()
		stream = c.streams[key]
		if stream == nil {
			stream = &Stream{
				codec:     c,
				callID:    header.CallID,
				callSN:    header.CallSN,
				c:         make(chan struct{}),
				bSvc:      true,
				connectAt: time.Now().Unix(),
			}
		}
		return
	}()
	if stream.caller == nil {
		if fNewCaller == nil {
			log.Ctx(ctx).Error().Caller().Interface("header", header).Msg("drop, fNewCaller is nil")
			return
		}
		stream.caller = fNewCaller(header.CallID)
		if stream.caller == nil {
			log.Ctx(ctx).Error().Caller().Interface("header", header).Msg("drop, caller is nil")
			return
		}
	}

	req := stream.caller.NewReq()
	err = proto.Unmarshal(bsBody, req)
	if err != nil {
		err = errors.Errorf(err.Error())
		log.Ctx(ctx).Error().Caller().Interface("header", header).Err(err).Msg("drop")
		return
	}
	if stream.bSvcStreamMode {
		err = stream.write(req)
		if err != nil {
			log.Ctx(ctx).Error().Caller().Interface("header", header).Err(err).Msg("drop")
		}
		return
	}

	if !stream.bOld {
		// 避免刚连接时接收的数据丢失，新手保护20s
		if stream.connectAt+20 > time.Now().Unix() {
			err = stream.write(req)
		} else {
			stream.bOld = true
		}
	}

	ctx = context.WithValue(ctx, ctxStreamKey{}, stream)
	go c.Handler(ctx, stream.caller, header, req)
	return
}

func (c *Codec) VerStreamResp(ctx context.Context, header Header, bsBody []byte) (err error) {
	stream := func() *Stream {
		key := respsKey(header.Channel, header.CallSN)
		c.streamsLock.Lock()
		defer c.streamsLock.Unlock()
		return c.streams[key]
	}()
	if stream == nil || stream.caller == nil {
		err = errors.Errorf("CallSN is not exist, header.Channel: %d, header.CallSN: %d", header.Channel, header.CallSN)
		return
	}

	resp := stream.caller.NewResp()
	err = proto.Unmarshal(bsBody, resp)
	if err != nil {
		err = errors.Errorf(err.Error())
		log.Ctx(ctx).Error().Caller().Interface("header", header).Err(err).Msg("drop")
		return
	}
	stream.write(resp)
	return
}
