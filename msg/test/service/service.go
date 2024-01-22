package main

import (
	"context"
	"crypto/tls"
	"io/fs"
	"strings"
	"sync"
	"time"

	"github.com/lxt1045/errors"
	"github.com/lxt1045/utils/config"
	"github.com/lxt1045/utils/gid"
	"github.com/lxt1045/utils/log"
	"github.com/lxt1045/utils/msg/codec"
	"github.com/lxt1045/utils/msg/rpc"
	"github.com/lxt1045/utils/msg/rpc/base"
	"github.com/lxt1045/utils/msg/test/filesystem"
	"github.com/lxt1045/utils/msg/test/pb"
)

type Config struct {
	Debug bool
	Pprof bool
	Dev   bool
	Conn  config.Conn
	Log   config.Log
}

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ctx, _ = log.WithLogid(ctx, gid.GetGID())

	// 解析配置文件
	conf := &Config{}
	file := "static/conf/default.yml"
	bs, err := fs.ReadFile(filesystem.Static, file)
	if err != nil {
		err = errors.Errorf(err.Error())
		log.Ctx(ctx).Fatal().Caller().Err(err).Send()
	}
	err = config.Unmarshal(bs, conf)
	if err != nil {
		log.Ctx(ctx).Fatal().Caller().Err(err).Send()
	}

	cmtls := conf.Conn.TLS
	tlsConfig, err := config.LoadTLSConfig(filesystem.Static, cmtls.ServerCert, cmtls.ServerKey, cmtls.CACert)
	if err != nil {
		log.Ctx(ctx).Fatal().Caller().Err(err).Send()
		return
	}
	listener, err := tls.Listen("tcp", conf.Conn.TCP, tlsConfig)
	if err != nil {
		log.Ctx(ctx).Fatal().Caller().Err(err).Send()
		return
	}
	defer listener.Close()

	for i := 0; ; i++ {
		select {
		case <-ctx.Done():
			return
		default:
		}
		conn, err := listener.Accept()
		if err != nil {
			if strings.Contains(err.Error(), "use of closed network connection") {
				log.Ctx(ctx).Error().Caller().Err(errors.Errorf(err.Error())).Send()
				// panic(err)
			} else {
				log.Ctx(ctx).Error().Caller().Err(err).Send()
			}
			continue
		}

		ctx := context.TODO()
		ctx = context.WithValue(ctx, connRemoteAddr{}, conn.RemoteAddr().String())
		_, err = rpc.NewService(ctx, conn, NewServer(), base.RegisterHelloServer)
		if err != nil {
			log.Ctx(ctx).Fatal().Caller().Err(err).Send()
		}
		continue
	}
}

type connRemoteAddr struct{}

type ClientInfo struct {
	RemoteAddr string
	*codec.Stream
	*pb.ClientInfo
}

type server struct {
	clients map[string]ClientInfo
	lock    sync.Mutex
}

func NewServer() *server {
	return &server{
		clients: make(map[string]ClientInfo),
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
func (s *server) Clients(ctx context.Context, in *pb.ClientsReq) (out *pb.ClientsRsp, err error) {
	clients := func() []*pb.ClientInfo {
		s.lock.Lock()
		defer s.lock.Unlock()
		m := make([]*pb.ClientInfo, len(s.clients))
		for _, v := range s.clients {
			m = append(m, v.ClientInfo)
		}
		return m
	}()
	return &pb.ClientsRsp{
		Clients: clients,
	}, nil
}
func (s *server) ConnTo(ctx context.Context, in *pb.ConnToReq) (out *pb.ConnToRsp, err error) {
	target := func() ClientInfo {
		s.lock.Lock()
		defer s.lock.Unlock()
		return s.clients[in.Client.Name]
	}()
	if target.Stream == nil || target.ClientInfo == nil {
		out = &pb.ConnToRsp{
			Status: pb.ConnToRsp_Fail,
			Err: &pb.Err{
				Msg: in.Client.Name + " not exist",
			},
		}
		return
	}

	// 这里注意可以先探测一下网络延时
	go func() {
		target.Stream.Clear(ctx)
		req := &pb.WaitConnRsp{}
		err = target.Stream.Send(ctx, req)
		if err != nil {
			log.Ctx(ctx).Error().Caller().Err(err).Msg("ConnTo")
			return
		}
		r, err := target.Stream.Recv(ctx)
		if err != nil {
			log.Ctx(ctx).Error().Caller().Err(err).Msg("ConnTo")
			return
		}
		resp, ok := r.(*pb.WaitConnReq)
		if !ok {
			log.Ctx(ctx).Error().Caller().Err(err).Msg("ConnTo")
			return
		}
	}()

	return &pb.ConnToRsp{}, nil
}

func (s *server) WaitConn(ctx context.Context, in *pb.WaitConnReq) (out *pb.WaitConnRsp, err error) {
	// stream 模式的返回值由己方随意控制
	if stream := codec.GetStream(ctx); stream != nil {
		iface, err1 := stream.Recv(ctx)
		if err1 != nil {
			log.Ctx(ctx).Info().Caller().Err(err1).Msg("err")
			return
		}
		in = iface.(*pb.WaitConnReq)
		func() {
			remoteAddr, _ := ctx.Value(connRemoteAddr{}).(string)
			client := ClientInfo{
				RemoteAddr: remoteAddr,
				Stream:     stream,
				ClientInfo: in.Client,
			}
			s.lock.Lock()
			defer s.lock.Unlock()
			s.clients[in.Client.Name] = client
		}()
		return
	}

	log.Ctx(ctx).Error().Caller().Msg("WaitConn should be call by stream mode")
	return
}
