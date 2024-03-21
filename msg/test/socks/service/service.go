package main

import (
	"context"
	"sync"
	"time"

	"github.com/lxt1045/utils/log"
	"github.com/lxt1045/utils/msg/rpc"
	"github.com/lxt1045/utils/msg/test/pb"
)

var (
	clients     = make(map[string]*server)
	clientsLock sync.Mutex
)

type server struct {
	MyName     string
	RemoteAddr string
	Network    pb.Network
	Peer       rpc.Peer
}

func NewServer(remoteAddr string) *server {
	return &server{
		RemoteAddr: remoteAddr,
		// ClientInfo
	}
}

func (s *server) Latency(ctx context.Context, in *pb.LatencyReq) (out *pb.LatencyRsp, err error) {
	return &pb.LatencyRsp{
		Ts: time.Now().UnixNano(),
	}, nil
}
func (s *server) Auth(ctx context.Context, in *pb.AuthReq) (out *pb.AuthRsp, err error) {
	return &pb.AuthRsp{}, nil
}
func (s *server) Close(ctx context.Context, in *pb.CloseReq) (out *pb.CloseRsp, err error) {
	clientsLock.Lock()
	defer clientsLock.Unlock()
	delete(clients, s.MyName)
	return &pb.CloseRsp{}, nil
}
func (s *server) Clients(ctx context.Context, in *pb.ClientsReq) (out *pb.ClientsRsp, err error) {
	s.MyName = in.MyName

	clients := func() []*pb.ClientInfo {
		clientsLock.Lock()
		defer clientsLock.Unlock()
		m := make([]*pb.ClientInfo, 0, len(clients))
		for k, v := range clients {
			if v.Peer.Client.Codec.IsClosed() {
				delete(clients, k)
				continue
			}
			m = append(m, &pb.ClientInfo{
				Name:    v.MyName,
				Addr:    v.RemoteAddr,
				Network: pb.Network_TCP,
			})
		}
		clients[s.MyName] = s
		return m
	}()

	return &pb.ClientsRsp{Clients: clients}, nil
}

func (s *server) ConnTo(ctx context.Context, in *pb.ConnToReq) (out *pb.ConnToRsp, err error) {
	target := func() *server {
		clientsLock.Lock()
		defer clientsLock.Unlock()
		return clients[in.Client.Name]
	}()
	if target == nil {
		out = &pb.ConnToRsp{
			Status: pb.ConnToRsp_Fail,
			Err: &pb.Err{
				Msg: in.Client.Name + " not exist",
			},
		}
		return
	}

	// 这里注意可以先探测一下网络延时
	detaTar, err := getLatency(ctx, target.Peer)
	if err != nil {
		log.Ctx(ctx).Error().Caller().Err(err).Send()
		return
	}
	detaSvs, err := getLatency(ctx, s.Peer)
	if err != nil {
		log.Ctx(ctx).Error().Caller().Err(err).Send()
		return
	}
	tsNow := time.Now().UnixNano() + int64(time.Second)
	tsTar, tsSvs := tsNow+detaTar, tsNow+detaSvs
	go func() {
		req := &pb.ConnToReq{
			Timestamp: tsTar,
			Client: &pb.ClientInfo{
				Name:    s.MyName,
				Addr:    s.RemoteAddr,
				Network: s.Network,
			},
		}
		resp := &pb.ConnToRsp{}
		err := target.Peer.Invoke(ctx, "ConnTo", req, resp)
		if err != nil {
			return
		}
		log.Ctx(ctx).Info().Caller().Interface("resp", resp).Send()
	}()

	out = &pb.ConnToRsp{
		Timestamp: tsSvs,
		Client: &pb.ClientInfo{
			Name:    target.MyName,
			Addr:    target.RemoteAddr,
			Network: target.Network,
		},
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
