package main

import (
	"context"
	"runtime"
	"sync/atomic"
	"time"

	"github.com/lxt1045/errors"
	"github.com/lxt1045/utils/log"
	"github.com/lxt1045/utils/rpc"
	"github.com/lxt1045/utils/rpc/socket"
	"github.com/lxt1045/utils/rpc/test/socks/pb"
)

type client struct {
	Name       string
	LocalAddr  string
	RemoteAddr string
	Peer       rpc.Peer
	ClientInfo *pb.ClientInfo

	bClient   bool
	peerCli   *peerCli
	peerSvc   *peerSvc
	socksAddr string
}

func (c *client) Latency(ctx context.Context, in *pb.LatencyReq) (out *pb.LatencyRsp, err error) {
	return &pb.LatencyRsp{
		Ts: time.Now().UnixNano(),
	}, nil
}

func (c *client) Close(ctx context.Context, in *pb.CloseReq) (out *pb.CloseRsp, err error) {
	err = c.Peer.Close(ctx)
	return &pb.CloseRsp{}, err
}

func (c *client) ConnPeer(ctx context.Context, in *pb.ConnPeerReq) (resp *pb.ConnPeerRsp, err error) {
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

	conn, err := socket.Dial(ctx, "tcp4", c.LocalAddr, in.Client.Addr)
	if err != nil {
		log.Ctx(ctx).Error().Caller().Err(err).Msg("Dial failed")
		return
	}

	log.Ctx(ctx).Info().Caller().Str("local", conn.LocalAddr().String()).
		Str("remote", conn.RemoteAddr().String()).Msg("connected")

	n := atomic.AddUint32(&nStream, 1)
	if n > 1 {
		log.Ctx(ctx).Error().Caller().Str("local", conn.LocalAddr().String()).
			Str("remote", conn.RemoteAddr().String()).Msgf("conn alredy connected")
		// return
	}

	if c.bClient {
		me := &peerCli{
			LocalAddr:  conn.LocalAddr().String(),
			RemoteAddr: conn.RemoteAddr().String(),
		}
		me.Peer, err = rpc.StartPeer(ctx, conn, me, pb.RegisterPeerCliServer, pb.NewPeerSvcClient)
		if err != nil {
			log.Ctx(ctx).Error().Caller().Err(err).Send()
			return
		}
		c.peerCli = me

		log.Ctx(ctx).Error().Caller().Msgf("SOCKS proxy on %s", c.socksAddr)
		TCPLocal(ctx, c.socksAddr, c.peerCli)

	} else {
		me := &peerSvc{
			LocalAddr:  conn.LocalAddr().String(),
			RemoteAddr: conn.RemoteAddr().String(),
		}
		me.Peer, err = rpc.StartPeer(ctx, conn, me, pb.RegisterPeerSvcServer, pb.NewPeerCliClient)
		if err != nil {
			log.Ctx(ctx).Error().Caller().Err(err).Send()
			return
		}

		c.peerSvc = me
	}
	return
}

func (c *client) auth(ctx context.Context) (err error) {
	req := &pb.AuthReq{
		Name: c.Name,
	}
	resp := &pb.AuthRsp{}
	err = c.Peer.Invoke(ctx, "Auth", req, resp)
	if err != nil {
		log.Ctx(ctx).Error().Caller().Err(err).Send()
		return
	}

	return
}

func (c *client) clients(ctx context.Context) (clients []*pb.ClientInfo, err error) {
	req := &pb.ClientsReq{
		MyName: c.Name,
	}
	resp := &pb.ClientsRsp{}
	err = c.Peer.Invoke(ctx, "Clients", req, resp)
	if err != nil {
		log.Ctx(ctx).Error().Caller().Err(err).Send()
		return
	}

	clients = resp.Clients
	return
}

var (
	nStream uint32
)
