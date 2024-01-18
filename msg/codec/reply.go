package codec

import (
	"context"
	"time"

	"github.com/gogo/protobuf/proto"
	"github.com/lxt1045/errors"
	"github.com/lxt1045/utils/log"
	"github.com/lxt1045/utils/msg/call/base"
)

func (c *Codec) Handler(ctx context.Context, caller Caller, header Header, req Msg) (resp Msg, err error) {
	defer func() {
		// resp 发送放 defer 中，及时panic也有返回值
		e := recover()
		if e != nil {
			err = errors.Errorf("recover: %v", e)
		}
		if err != nil {
			log.Ctx(ctx).Error().Caller().Err(err).Send()
		}
	}()
	resp, err = caller.SvcInvoke(ctx, req)
	if err != nil {
		return
	}
	if resp == nil || caller.RespType() == nil {
		return
	}

	bs := bufPool.Get().([]byte)
	wbuf := bs[:HeaderSize]      // 给 Header 预留足够的内存
	buf := proto.NewBuffer(wbuf) //
	err = buf.Marshal(resp)
	if err != nil {
		bufPool.Put(bs)
		return
	}
	wbuf = buf.Bytes()
	defer bufPool.Put(wbuf)

	ver := uint16(VerCallResp)
	if header.Ver == VerStreamReq {
		ver = VerStreamResp
	}
	c.Send(ctx, wbuf, ver, header.Channel, header.CallID, header.CallSN)
	return
}

// 收到 Req
func (c *Codec) VerCallReq(ctx context.Context, header Header, bsBody []byte, fNewCaller func(callID uint16) Caller) (err error) {
	if fNewCaller == nil {
		return
	}
	caller := fNewCaller(header.CallID)
	if caller == nil {
		log.Ctx(ctx).Error().Caller().Interface("header", header).Msg("drop, caller is nil")
		return
	}
	req := caller.NewReq()
	err = proto.Unmarshal(bsBody, req)
	if err != nil {
		err = errors.Errorf(err.Error())
		log.Ctx(ctx).Error().Caller().Interface("header", header).Err(err).Msg("drop")
		return
	}

	go c.Handler(ctx, caller, header, req)
	return
}

// 收到 Resp
func (c *Codec) VerCallResp(ctx context.Context, header Header, bsBody []byte) (err error) {
	res := func() resp {
		key := respsKey(header.Channel, header.CallSN)
		c.respsLock.Lock()
		defer c.respsLock.Unlock()
		return c.resps[key]
	}()
	if res.r == nil {
		log.Ctx(ctx).Error().Caller().Interface("header", header).Msg("drop, res.r is nil")
		return
	}

	err = proto.Unmarshal(bsBody, res.r)
	if err != nil {
		err = errors.Errorf(err.Error())
		log.Ctx(ctx).Error().Caller().Interface("header", header).Err(err).Msg("drop")
		return
	}
	res.done <- nil
	return
}

// 收到 Req
func (c *Codec) VerCmdReq(ctx context.Context, header Header, bsBody []byte) (err error) {
	req := &base.CmdReq{}
	err = proto.Unmarshal(bsBody, req)
	if err != nil {
		err = errors.Errorf(err.Error())
		log.Ctx(ctx).Error().Caller().Interface("header", header).Err(err).Msg("drop")
		return
	}

	switch req.Cmd {
	case base.CmdReq_Stream:
		func() {
			key := respsKey(header.Channel, header.CallSN)
			c.streamsLock.Lock()
			defer c.streamsLock.Unlock()
			c.streams[key] = &Stream{
				// reqType:  caller.ReqType(),
				// respType: caller.RespType(),
				codec:     c,
				callID:    header.CallID,
				callSN:    header.CallSN,
				c:         make(chan struct{}),
				bSvc:      true,
				connectAt: time.Now().Unix(),
			}
		}()
		res := &base.CmdRsp{
			Status: base.CmdRsp_Succ,
		}
		err = c.SendMsg(ctx, VerCmdResp, header.Channel, header.CallID, header.CallSN, res)
		if err != nil {
			return
		}
	case base.CmdReq_Auth:
	case base.CmdReq_CallIDs:
		res := &base.CmdRsp{
			Status: base.CmdRsp_Succ,
			// Fields: c.callIDs,
		}
		c.callIDsLock.Lock()
		res.Fields = c.callIDs
		c.callIDsLock.Unlock()

		err = c.SendMsg(ctx, VerCmdResp, header.Channel, header.CallID, header.CallSN, res)
		if err != nil {
			return
		}
	default:
	}
	return
}