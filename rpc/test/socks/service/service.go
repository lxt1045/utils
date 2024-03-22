package main

import (
	"context"
	"sync"
	"time"

	"github.com/lxt1045/utils/log"
	"github.com/lxt1045/utils/rpc"
	"github.com/lxt1045/utils/rpc/test/socks/pb"
)

var (
	svcs     = make(map[string]*service)
	svcsLock sync.RWMutex
)

func AddSvc(svc *service) {
	svcsLock.Lock()
	defer svcsLock.Unlock()
	svcs[svc.Name] = svc
}

type service struct {
	Name       string
	RemoteAddr string
	Network    pb.Network
	Peer       rpc.Peer
}

func (s *service) Close(ctx context.Context, req *pb.CloseReq) (resp *pb.CloseRsp, err error) {
	svcsLock.Lock()
	defer svcsLock.Unlock()
	delete(svcs, s.Name)
	return &pb.CloseRsp{}, nil
}

func (s *service) Auth(ctx context.Context, req *pb.AuthReq) (resp *pb.AuthRsp, err error) {
	s.Name = req.Name
	resp = &pb.AuthRsp{}
	svcsLock.Lock()
	defer svcsLock.Unlock()
	svcs[s.Name] = s
	return
}

func (s *service) Clients(ctx context.Context, req *pb.ClientsReq) (resp *pb.ClientsRsp, err error) {
	clients := func() []*pb.ClientInfo {
		m := make([]*pb.ClientInfo, 0, len(svcs))
		svcsLock.RLock()
		defer svcsLock.RUnlock()
		for k, v := range svcs {
			if v.Peer.Client.Codec.IsClosed() {
				delete(svcs, k)
				continue
			}
			if s.Name == v.Name {
				continue
			}
			m = append(m, &pb.ClientInfo{
				Name:    v.Name,
				Addr:    v.RemoteAddr,
				Network: pb.Network_TCP,
			})
		}
		return m
	}()

	return &pb.ClientsRsp{Clients: clients}, nil
}

func (s *service) Latency(ctx context.Context, req *pb.LatencyReq) (resp *pb.LatencyRsp, err error) {
	return &pb.LatencyRsp{
		Ts: time.Now().UnixNano(),
	}, nil
}

func (s *service) ConnTo(ctx context.Context, req *pb.ConnToReq) (resp *pb.ConnToRsp, err error) {
	target := func() *service {
		svcsLock.Lock()
		defer svcsLock.Unlock()
		return svcs[req.Client.Name]
	}()
	if target == nil {
		resp = &pb.ConnToRsp{
			Status: pb.ConnToRsp_Fail,
			Err: &pb.Err{
				Msg: req.Client.Name + " not exist",
			},
		}
		return
	}

	// 这里注意可以先探测一下网络延时
	latencys := getLatencys(ctx, target.Peer, s.Peer)
	for _, latency := range latencys {
		if latency.err != nil {
			log.Ctx(ctx).Error().Caller().Err(latency.err).Send()
			return
		}
		if latency.deta == 0 {
			log.Ctx(ctx).Error().Caller().Msgf("deta is %d", latency.deta)
			return
		}
	}
	detaTar, detaSvs := latencys[0].deta, latencys[1].deta
	tsNow := time.Now().UnixNano() + int64(time.Second)
	tsTar, tsSvs := tsNow+detaTar, tsNow+detaSvs
	go func() {
		req := &pb.ConnToReq{
			Timestamp: tsTar,
			Client: &pb.ClientInfo{
				Name:    s.Name,
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

	resp = &pb.ConnToRsp{
		Timestamp: tsSvs,
		Client: &pb.ClientInfo{
			Name:    target.Name,
			Addr:    target.RemoteAddr,
			Network: target.Network,
		},
	}
	return
}

type respLatency struct {
	deta int64
	err  error
}

// deta == 对方时钟 - 我方时钟； 用于对时
func getLatencys(ctx context.Context, peers ...rpc.Peer) (resps []respLatency) {

	resps = make([]respLatency, len(peers))
	wg := sync.WaitGroup{}

	for i := range peers {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			resps[i].deta, resps[i].err = getLatency(ctx, peers[i])
		}(i)
	}
	wg.Wait()
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
