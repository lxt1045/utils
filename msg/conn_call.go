package msg

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/gogo/protobuf/proto"
	"github.com/lxt1045/errors"
	"github.com/lxt1045/utils/gid"
	"github.com/lxt1045/utils/log"
	"github.com/lxt1045/utils/msg/pb"
)

var (
	poolBuf = func() *sync.Pool {
		return &sync.Pool{
			New: func() any {
				bs := make([]byte, 0x7fff)
				return &bs
			},
		}
	}()
	// 用于call
	callees = []func(ctx context.Context, conn *Conn, m Msg) (Msg, error){
		pb.CallID_CAll_INVAL: nil,
		// pb.CallID_CALL_MAX:     nil,
	}
	calleesLock sync.Mutex
)

func RegisterCaller(callID pb.CallID, f func(ctx context.Context, conn *Conn, m Msg) (Msg, error)) (err error) {
	calleesLock.Lock()
	defer calleesLock.Unlock()
	for id, f := range callees {
		if id == int(callID) && f != nil {
			err = errors.Errorf("call_id already exists")
			return
		}
	}
	if need := int(callID) - len(callees) + 1; need > 0 {
		callees = append(callees, make([]func(ctx context.Context, conn *Conn, m Msg) (Msg, error), need)...)
	}
	callees[callID] = f
	return
}

type call struct {
	ch      chan *pb.CallResp
	req     *pb.CallReq
	resps   []*pb.CallResp
	respEnd bool
	lock    sync.Mutex
}

func (conn *Conn) Call(ctx context.Context, req Msg, logID int64, CallId pb.CallID, timeout time.Duration) (resp Msg, err error) {
	if conn == nil {
		err = errors.Errorf("conn == nil")
		return
	}
	pbuf := poolBuf.Get().(*[]byte)
	defer poolBuf.Put(pbuf)
	*pbuf, err = MarshalMsg(ctx, req, logID, *pbuf)
	if err != nil {
		return
	}

	callReq := call{
		ch: make(chan *pb.CallResp, 1),
		req: &pb.CallReq{
			LogId:  logID,
			CallId: CallId,
			Sn:     gid.GetGID(),
			Req:    *pbuf,
		},
	}

	conn.callCacheLock.Lock()
	conn.callCache[callReq.req.Sn] = &callReq
	conn.callCacheLock.Unlock()

	pbufC := poolBuf.Get().(*[]byte)
	*pbufC, err = conn.SendMsg(ctx, callReq.req, logID, *pbufC)
	poolBuf.Put(pbufC)
	if err != nil {
		return
	}

	if timeout <= 0 {
		timeout = time.Second * 10
	}
	ticker := time.NewTicker(timeout) // TODO 60
	defer func() {
		ticker.Stop()
		conn.callCacheLock.Lock()
		delete(conn.callCache, callReq.req.Sn)
		conn.callCacheLock.Unlock()
	}()

	select {
	case callResp := <-callReq.ch:
		if len(callResp.Resp) >= HeaderSize {
			resp, err = UnmarshalMsg(ctx, callResp.Resp)
		} else {
			str := "resp is empty"
			if callResp.ErrMsg != "" {
				str = callResp.ErrMsg
			} else if callResp.ErrCode != 0 {
				str = fmt.Sprintf("error code is %d", callResp.ErrCode)
			}
			if callResp.ErrCode != 0 {
				err = errors.NewErr(int(callResp.ErrCode), str)
			} else {
				err = errors.Errorf(str)
			}
		}
		if err != nil {
			return
		}
	case <-ticker.C:
		err = errors.Errorf("timeout")
	}

	return
}

func (conn *Conn) callResp(ctx context.Context, buf []byte) (err error) {
	resp := &pb.CallResp{}
	err = proto.Unmarshal(buf, resp)
	if err != nil {
		err = errors.Errorf(err.Error())
		return
	}
	if resp.CallId < 1 {
		return
	}
	if int(resp.CallId) > len(callees) {
		return
	}

	conn.callCacheLock.Lock()
	call := conn.callCache[resp.Sn]
	conn.callCacheLock.Unlock()
	if call != nil && call.ch != nil {
		call.resps = append(call.resps, resp)

		if resp.Status == pb.CallStatus_CALL_END {
			call.respEnd = true
		}
		if call.respEnd {
			resp = call.resps[0]
			bFull := true
			if len(call.resps) > 1 {
				sort.Slice(call.resps, func(i, j int) bool {
					return call.resps[i].RespSn < call.resps[j].RespSn
				})
				for i, r := range call.resps {
					if i != int(r.RespSn) {
						bFull = false
					}
				}
				if bFull {
					for _, r := range call.resps[1:] {
						resp.Resp = append(resp.Resp, r.Resp...)
					}
				}
			}
			if bFull {
				call.ch <- resp
			}
		}
	} else {
		err = errors.Errorf("call req not exist, sn:%d, call_id:%d", resp.Sn, resp.CallId)
	}
	return
}

func (conn *Conn) callReq(ctx context.Context, buf []byte) (err error) {
	req := &pb.CallReq{}
	err = proto.Unmarshal(buf, req)
	if err != nil {
		err = errors.Errorf(err.Error()) // err 放到 conn.doCall 里处理，因为即使报错也要返回一个resp数据包
	}

	go conn.doCall(ctx, req, err)

	return
}

func cutStr(str, sub string) string {
	i := strings.Index(str, sub)
	if i <= 0 {
		return str
	}
	return str[:i]
}

var (
	UtilsConnClosed = errors.NewCode(0, 111, "UtilsConnClosed")
)

func (conn *Conn) doCall(ctx context.Context, req *pb.CallReq, err error) {
	resp := &pb.CallResp{
		LogId:  req.LogId,
		CallId: req.CallId,
		Sn:     req.Sn,
		Status: pb.CallStatus_CALL_END,
	}
	// conn.directFlush = true
	const lMaxSend = 32 * 1024
	var pbuf *[]byte
	defer func() {
		e := recover()
		if e != nil {
			err = errors.Errorf("%+v", e)
		}
		if err != nil {
			log.Ctx(ctx).Error().Err(err).Int16("AgentID", conn.AgentID).Int32("call", int32(req.CallId)).Caller().Msg("doCall")
			resp.ErrMsg = cutStr(err.Error(), "\n")
			// resp.ErrCode = int64(errno.Code(err))
		}

		pbufC := poolBuf.Get().(*[]byte)
		if len(resp.Resp) <= lMaxSend {
			*pbufC, err = conn.SendMsg(ctx, resp, req.LogId, *pbufC)
			if err != nil {
				if req.CallId == pb.CallID_CALL_CLOSE && errors.Is(UtilsConnClosed, err) {
					err = nil
				} else {
					log.Ctx(ctx).Error().Err(err).Int16("AgentID", conn.AgentID).
						Int32("call", int32(req.CallId)).Caller().Msg("doCall")
				}
			}
		} else {
			// 超出最大值，则分多次发送
			bsSend := resp.Resp
			for i := 0; ; i++ {
				if len(bsSend) > lMaxSend {
					resp.Resp, bsSend = bsSend[:lMaxSend], bsSend[lMaxSend:]
					resp.Status = pb.CallStatus_CALL_MORE
				} else if len(bsSend) > 0 {
					resp.Resp, bsSend = bsSend, bsSend[:0]
					resp.Status = pb.CallStatus_CALL_END
				} else {
					break
				}
				resp.RespSn = int64(i)
				*pbufC, err = conn.SendMsg(ctx, resp, req.LogId, *pbufC)
				if err != nil {
					log.Ctx(ctx).Error().Err(err).Int16("AgentID", conn.AgentID).Int32("call", int32(req.CallId)).Caller().Msg("doCall")
				}
			}
		}
		poolBuf.Put(pbufC)
		if pbuf != nil {
			poolBuf.Put(pbuf)
		}
	}()
	if err != nil {
		return
	}
	if req.CallId < 1 || int(req.CallId) > len(callees) {
		err = errors.Errorf("call_id[%d] is not exist", req.CallId)
		return
	}
	f := callees[req.CallId]
	if f == nil {
		err = errors.Errorf("call_id[%d] is not exist", req.CallId)
		return
	}

	mReq, err := UnmarshalMsg(ctx, req.Req)
	if err != nil {
		return
	}
	mResp, err := f(ctx, conn, mReq)
	if err != nil {
		return
	}
	if mResp != nil {
		pbuf = poolBuf.Get().(*[]byte)
		*pbuf, err = MarshalMsg(ctx, mResp, req.LogId, *pbuf)
		if err == nil {
			resp.Resp = *pbuf
		}
	}
	return
}
