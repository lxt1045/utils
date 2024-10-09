package main

import (
	"context"
	"strings"

	"github.com/lxt1045/errors"
	"github.com/lxt1045/utils/config"
	"github.com/lxt1045/utils/gid"
	"github.com/lxt1045/utils/log"
	"github.com/lxt1045/utils/rpc"
	"github.com/lxt1045/utils/rpc/conn"
	socks "github.com/lxt1045/utils/rpc/test/socks_quic2"
	"github.com/lxt1045/utils/rpc/test/socks_quic2/filesystem"
	"github.com/lxt1045/utils/rpc/test/socks_quic2/pb"
	"github.com/quic-go/quic-go"
	_ "go.uber.org/automaxprocs"
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
	err := config.UnmarshalFS(file, filesystem.Static, conf)
	if err != nil {
		log.Ctx(ctx).Fatal().Caller().Err(err).Send()
		return
	}

	cmtls := conf.Conn.TLS
	tlsConfig, err := config.LoadTLSConfig(filesystem.Static, cmtls.ServerCert, cmtls.ServerKey, cmtls.CACert)
	if err != nil {
		log.Ctx(ctx).Fatal().Caller().Err(err).Send()
		return
	}
	// listener, err := tls.Listen("tcp", conf.Conn.Addr, tlsConfig)
	listener, err := quic.ListenAddr(conf.Conn.Addr, tlsConfig, nil)
	if err != nil {
		log.Ctx(ctx).Fatal().Caller().Err(err).Send()
		return
	}
	defer listener.Close()
	log.Ctx(ctx).Info().Caller().Str("Listen", conf.Conn.Addr).Send()

	gPeer, err := rpc.StartPeer(ctx, nil, &socks.SocksSvc{}, pb.NewSocksCliClient, pb.RegisterSocksSvcServer)
	if err != nil {
		log.Ctx(ctx).Fatal().Caller().Err(err).Send()
		return
	}
	for i := 0; ; i++ {
		select {
		case <-ctx.Done():
			return
		default:
		}
		ctx := context.TODO()
		c, err := listener.Accept(ctx)
		if err != nil {
			if strings.Contains(err.Error(), "use of closed network connection") {
				log.Ctx(ctx).Error().Caller().Err(errors.Errorf(err.Error())).Send()
				// panic(err)
			} else {
				log.Ctx(ctx).Error().Caller().Err(err).Send()
			}
			continue
		}
		svcConn, err := conn.WrapQuic(ctx, c)
		if err != nil {
			log.Ctx(ctx).Fatal().Caller().Err(err).Send()
		}
		zsvc, err := conn.NewZip(ctx, svcConn)
		if err != nil {
			log.Ctx(ctx).Fatal().Caller().Err(err).Send()
		}

		log.Ctx(ctx).Info().Caller().Str("local", svcConn.LocalAddr().String()).Str("remote", svcConn.RemoteAddr().String()).Send()

		svc := &socks.SocksSvc{
			RemoteAddr: svcConn.RemoteAddr().String(),
		}
		peer, err := gPeer.Clone(ctx, zsvc, svc)
		if err != nil {
			log.Ctx(ctx).Error().Caller().Err(err).Send()
			continue
		}
		svc.Peer = peer

		continue
	}
}
