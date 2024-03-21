package codec

import (
	"context"
	"time"

	"github.com/gogo/protobuf/proto"
	"github.com/lxt1045/errors"
	"github.com/lxt1045/utils/log"
	"github.com/lxt1045/utils/msg/rpc/base"
)

func (c *Codec) Handler(ctx context.Context, caller Caller, header Header, req Msg) (resp Msg, err error) {
	defer func() {
		// resp 发送放 defer 中，及时panic也有返回值
		e := recover()
		if e != nil {
			err = errors.Errorf("func: %s, recover: %v", caller.FuncName(), e)
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

	ver := uint16(VerCallResp)
	if header.Ver == VerStreamReq {
		ver = VerStreamResp
	}

	err = c.SendMsg(ctx, ver, header.Channel, header.CallID, header.CallSN, resp)
	if err != nil {
		return
	}
	return
}

// 收到 Req
func (c *Codec) VerCallReq(ctx context.Context, header Header, bsBody []byte, svcMethods []Caller) (err error) {
	if uint16(len(svcMethods)) <= header.CallID {
		log.Ctx(ctx).Error().Caller().Interface("header", header).Msg("drop, caller is nil")
		return
	}
	caller := svcMethods[header.CallID]
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
		key := respsKey(header.CallSN)
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
			key := respsKey(header.CallSN)
			c.streamsLock.Lock()
			defer c.streamsLock.Unlock()
			c.streams[key] = &Stream{
				// reqType:  caller.ReqType(),
				// respType: caller.RespType(),
				codec:     c,
				callID:    header.CallID,
				callSN:    header.CallSN,
				cacheCh:   make(chan struct{}),
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
	case base.CmdReq_StreamClose:
		func() {
			key := respsKey(header.CallSN)
			c.streamsLock.Lock()
			defer c.streamsLock.Unlock()
			delete(c.streams, key)
		}()
		res := &base.CmdRsp{
			Status: base.CmdRsp_Succ,
		}
		err = c.SendMsg(ctx, VerCmdResp, header.Channel, header.CallID, header.CallSN, res)
		if err != nil {
			return
		}
		log.Ctx(ctx).Info().Caller().Interface("header", header).Msg("stream close by peer")
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
