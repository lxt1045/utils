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
	"github.com/lxt1045/utils/rpc/base"
)

type Stream struct {
	codec     *Codec
	channel   uint16
	callID    uint16
	callSN    uint32
	cache     []Msg
	cacheLock sync.Mutex
	cacheCh   chan struct{}

	caller     Method
	connectAt  int64
	deadline   int64 // 超时时间，超时后删除
	bSvc       bool
	bClosed    bool
	bFirstCall bool // 流的第一次调用
}

func (s *Stream) Close(ctx context.Context) {
	// 1. 先发送关闭请求
	req := &base.CmdReq{Cmd: base.CmdReq_StreamClose}
	res := &base.CmdRsp{}
	done, err := s.codec.clientCall(ctx, VerCmdReq, s.channel, s.callID, s.callSN, req, res)
	if err != nil {
		return
	}
	if done != nil {
		err = <-done
		if err != nil {
			log.Ctx(ctx).Warn().Caller().Err(err).Msg("Stream.close")
		}
	}

	// 2. 再删除
	c := s.codec
	key := respsKey(s.callSN)
	c.streamsLock.Lock()
	defer c.streamsLock.Unlock()
	delete(c.streams, key)

	// 3. 关闭标志
	s.close()
}

func (s *Stream) close() {
	s.bClosed = true
	select {
	case s.cacheCh <- struct{}{}:
	default:
	}
}

func (s *Stream) Method() (method string) {
	return s.codec.callers[s.callID].FuncName()
}

func (s *Stream) Recv(ctx context.Context) (resp Msg, err error) {
	recv := func() (req Msg) {
		s.cacheLock.Lock()
		defer s.cacheLock.Unlock()
		if len(s.cache) > 0 {
			req = s.cache[0]
			s.cache = s.cache[1:]
			select {
			case s.cacheCh <- struct{}{}:
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
		<-s.cacheCh
	}
}

// Clear 清空以前接收的数据（准备好接收下一个数据包）
func (s *Stream) Clear(ctx context.Context) {
	s.cacheLock.Lock()
	defer s.cacheLock.Unlock()
	s.cache = s.cache[:0]
}

func (s *Stream) write(m Msg) (err error) {
	s.cacheLock.Lock()
	defer s.cacheLock.Unlock()
	if len(s.cache) > 10240 {
		err = errors.Errorf("The cache size exceeds the limit:%d", len(s.cache))
		return
	}
	s.cache = append(s.cache, m)
	select {
	case s.cacheCh <- struct{}{}:
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

func (c *Codec) Stream(ctx context.Context, channel uint16, callID uint16, caller Method) (stream *Stream, err error) {
	stream = &Stream{
		codec:     c,
		callID:    callID,
		cacheCh:   make(chan struct{}),
		connectAt: time.Now().Unix(),
		caller:    caller,
	}
	func() {
		c.streamsLock.Lock()
		defer c.streamsLock.Unlock()
		for {
			callSN := atomic.AddUint32(&c.tmpCallSN, 1)
			key := respsKey(callSN)
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
	done, err := c.clientCall(ctx, VerCmdReq, channel, callID, stream.callSN, req, res)
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
	stream, ok := s.(*Stream)
	if !ok {
		log.Ctx(ctx).Info().Caller().Msg("unkonw error")
	}
	return
}

func (c *Codec) VerStreamReq(ctx context.Context, header Header, bsBody []byte) (err error) {
	stream, bFirstCall := func() (stream *Stream, bFirstCall bool) {
		key := respsKey(header.CallSN)
		c.streamsLock.Lock()
		defer c.streamsLock.Unlock()
		stream = c.streams[key]
		if stream != nil && stream.bFirstCall {
			bFirstCall = true
			stream.bFirstCall = false
		}
		return
	}()
	if stream == nil || stream.caller == nil {
		err = errors.Errorf("CallSN is not exist, header.Channel: %d, header.CallSN: %d", header.Channel, header.CallSN)
		log.Ctx(ctx).Error().Caller().Err(err).Interface("header", header).Msg("drop, stream is nil")
		return
	}
	stream.deadline = time.Now().Unix() + 1*60*60 // 最新一个请求之后一个小时都没有新的请求就删除

	req := stream.caller.NewReq()
	err = proto.Unmarshal(bsBody, req)
	if err != nil {
		err = errors.Errorf(err.Error())
		log.Ctx(ctx).Error().Caller().Interface("header", header).Err(err).Msg("drop")
		return
	}

	if bFirstCall {
		ctx = context.WithValue(ctx, ctxStreamKey{}, stream)
		go c.Handler(ctx, stream.caller, header, req)
		return
	}

	err = stream.write(req)
	if err != nil {
		log.Ctx(ctx).Error().Caller().Interface("header", header).Err(err).Msg("drop")
	}
	return
}

func (c *Codec) VerStreamResp(ctx context.Context, header Header, bsBody []byte) (err error) {
	stream := func() *Stream {
		key := respsKey(header.CallSN)
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
	err = stream.write(resp)
	if err != nil {
		log.Ctx(ctx).Error().Caller().Interface("header", header).Err(err).Msg("drop")
	}
	return
}
