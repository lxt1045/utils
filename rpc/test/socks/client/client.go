package main

import (
	"context"
	"runtime"
	"time"

	"github.com/lxt1045/errors"
	"github.com/lxt1045/utils/log"
	"github.com/lxt1045/utils/rpc"
	"github.com/lxt1045/utils/rpc/socket"
	"github.com/lxt1045/utils/rpc/test/pb"
)

type client struct {
	MyName     string
	RemoteAddr string
	Peer       rpc.Peer
	*pb.ClientInfo
}

func NewClient(remoteAddr string) *client {
	return &client{
		RemoteAddr: remoteAddr,
		// ClientInfo
	}
}

func (s *client) Latency(ctx context.Context, in *pb.LatencyReq) (out *pb.LatencyRsp, err error) {
	return &pb.LatencyRsp{
		Ts: time.Now().UnixNano(),
	}, nil
}
func (s *client) Close(ctx context.Context, in *pb.CloseReq) (out *pb.CloseRsp, err error) {
	return &pb.CloseRsp{}, nil
}
func (s *client) ConnTo(ctx context.Context, in *pb.ConnToReq) (out *pb.ConnToRsp, err error) {
	// 先 listen

	// 再 连接
	tsDo := in.Timestamp
	tsNow := time.Now().UnixNano()
	if tsDo < tsNow {
		err = errors.Errorf("tsDo: %d, tsNow: %d, time out", tsDo, tsNow)
		return
	}

	tsDiff := tsNow - tsDo - int64(time.Millisecond)*1
	time.Sleep(time.Duration(tsDiff))
	for {
		tsNow := time.Now().UnixNano()
		if tsDo <= tsNow {
			break
		}
		runtime.Gosched()
	}

	conn, err := socket.Dial(ctx, "tcp4", in.Client.Addr)
	if err != nil {
		log.Ctx(ctx).Error().Caller().Err(err).Msg("Dial failed")
		return
	}

	str := "Hello " + in.Client.Name
	_, err = conn.Write([]byte(str))
	if err != nil {
		log.Ctx(ctx).Error().Caller().Err(err).Msg("Dial failed")
		return
	}
	log.Ctx(ctx).Info().Caller().Str("read", str).
		Str("local", conn.LocalAddr().String()).Str("remote", conn.RemoteAddr().String()).Send()

	out = &pb.ConnToRsp{
		Status: pb.ConnToRsp_Succ,
	}
	return
}

// deta == 对方时钟 - 我方时钟； 用于对时
func getLatency(ctx context.Context, peer rpc.Peer) (deta int64, err error) {
	reqLatency := &pb.LatencyReq{
		Ts: time.Now().UnixNano(),
	}
	respLatency := &pb.LatencyRsp{}

	err = peer.Invoke(ctx, "Latency", reqLatency, respLatency)
	if err != nil {
		return
	}
	tsNow := time.Now().UnixNano()

	half := (tsNow + reqLatency.Ts) / 2
	deta = respLatency.Ts - half
	return
}
