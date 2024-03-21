package main

import (
	"context"
	"crypto/tls"
	"io/fs"
	"strings"

	"github.com/lxt1045/errors"
	"github.com/lxt1045/utils/config"
	"github.com/lxt1045/utils/gid"
	"github.com/lxt1045/utils/log"
	"github.com/lxt1045/utils/rpc"
	"github.com/lxt1045/utils/rpc/test/filesystem"
	"github.com/lxt1045/utils/rpc/test/pb"
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
	log.Ctx(ctx).Info().Caller().Str("Listen", conf.Conn.TCP).Send()

	for i := 0; ; i++ {
		select {
		case <-ctx.Done():
			return
		default:
		}
		ctx := context.TODO()
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

		svc := NewServer(conn.RemoteAddr().String())
		svc.Peer, err = rpc.StartPeer(ctx, conn, svc, pb.NewClientClient, pb.RegisterServiceServer)
		if err != nil {
			log.Ctx(ctx).Fatal().Caller().Err(err).Send()
		}
		continue
	}
}
