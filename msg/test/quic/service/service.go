package main

import (
	"context"
	"io/fs"
	"strings"

	"github.com/lxt1045/errors"
	"github.com/lxt1045/utils/config"
	"github.com/lxt1045/utils/gid"
	"github.com/lxt1045/utils/log"
	"github.com/lxt1045/utils/msg"
	"github.com/lxt1045/utils/msg/pb"
	"github.com/lxt1045/utils/msg/test/filesystem"
	quic "github.com/quic-go/quic-go"
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
	listener, err := quic.ListenAddr(conf.Conn.TCP, tlsConfig, nil)
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

		qc, err := msg.NewQuicConn(ctx, c)
		if err != nil {
			log.Ctx(ctx).Error().Caller().Err(err).Send()
			continue
		}

		// msg.SetReadWriteBuff(ctx, qc, 1024*1024*8, 0)

		conf.Conn.FlushTime = -1 // Write 的时候刷新
		conn, err := msg.NewConn(qc, int64(i), &conf.Conn, nil)
		if err != nil {
			log.Ctx(ctx).Error().Caller().Err(err).Send()
			conn.Close()
			continue
		}

		func(ctx context.Context, conn *msg.Conn) {

			defer func() {
				conn.Close()
			}()

			conn.ReadLoop(ctx, msg.MsgHandler(func(ctx context.Context, in Msg) (out Msg, err error) {
				log.Ctx(ctx).Info().Err(err).Caller().Interface("in", in).Send()

				req := &pb.Notice{
					Msg:   "hello",
					LogId: 11111,
				}
				_, err = conn.SendMsg(ctx, req, 222222, nil)
				if err != nil {
					log.Ctx(ctx).Error().Err(err).Caller().Send()
					c.Close()
					return
				}

				_, err = conn.SendMsg(ctx, req, 222222, nil)
				if err != nil {
					log.Ctx(ctx).Error().Err(err).Caller().Send()
					c.Close()
					return
				}

				// _, err = conn.SendMsgHaft(ctx, req, 222222, nil)
				// if err != nil {
				// 	log.Ctx(ctx).Error().Err(err).Caller().Send()
				// 	c.Close()
				// 	return
				// }
				conn.Close()

				return
			}))
		}(ctx, conn)

		continue
	}
}
