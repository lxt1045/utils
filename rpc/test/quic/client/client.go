package main

import (
	"context"
	"crypto/tls"
	"io/fs"
	"time"

	"github.com/lxt1045/errors"
	"github.com/lxt1045/utils/config"
	"github.com/lxt1045/utils/gid"
	"github.com/lxt1045/utils/log"
	msg "github.com/lxt1045/utils/rpc"
	"github.com/lxt1045/utils/rpc/pb"
	"github.com/lxt1045/utils/rpc/test/filesystem"
)

type Config struct {
	Debug      bool
	Pprof      bool
	Dev        bool
	Conn       config.Conn
	ClientConn config.Conn
	Log        config.Log
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

	cmtls := conf.ClientConn.TLS
	tlsConfig, err := config.LoadTLSConfig(filesystem.Static, cmtls.ClientCert, cmtls.ClientKey, cmtls.CACert)
	if err != nil {
		log.Ctx(ctx).Fatal().Caller().Err(err).Send()
		return
	}
	tlsConfig.ServerName = conf.ClientConn.Host

	c, err := tls.Dial("tcp", conf.ClientConn.Addr, tlsConfig)
	if err != nil {
		log.Ctx(ctx).Fatal().Caller().Err(err).Send()
		return
	}

	err = msg.SetReadWriteBuff(ctx, c, 0, 1024*1024*8)
	if err != nil {
		log.Ctx(ctx).Error().Err(err).Caller().Send()
	}

	conf.ClientConn.FlushTime = -1 // Write 的时候刷新
	conn, err := msg.NewConn(c, 996, &conf.ClientConn, nil)
	if err != nil {
		log.Ctx(ctx).Error().Err(err).Caller().Send()
		c.Close()
		return
	}
	defer func() {
		cancel()
		go conn.Close()
	}()

	ch := make(chan bool, 1)
	go func(ctx context.Context, conn *msg.Conn) {
		defer func() {
			log.Ctx(ctx).Error().Caller().Msg("defer")
		}()
		conn.ReadLoop(ctx, msg.MsgHandler(func(ctx context.Context, in Msg) (out Msg, err error) {
			log.Ctx(ctx).Info().Err(err).Caller().Interface("in", in).Send()

			ch <- true
			return
		}))
	}(ctx, conn)

	req := &pb.Notice{
		Msg:   "hello",
		LogId: 6666666,
	}
	_, err = conn.SendMsg(ctx, req, 999999, nil)
	if err != nil {
		log.Ctx(ctx).Error().Err(err).Caller().Send()
		c.Close()
		return
	}

	<-ch
	time.Sleep(time.Second * 3)

	// // c.Close()
	// _, err = conn.SendMsg(ctx, req, 999999, nil)
	// if err != nil {
	// 	log.Ctx(ctx).Error().Err(err).Caller().Send()
	// 	// c.Close()
	// 	// return
	// }

	// ch2 := make(chan bool, 1)
	// go func(ctx context.Context, conn *msg.Conn) {
	// 	ch2 <- true
	// 	conn.ReadLoop(ctx, msg.MsgHandler(func(ctx context.Context, in Msg) (out Msg, err error) {
	// 		log.Ctx(ctx).Info().Err(err).Caller().Interface("in", in).Send()
	// 		return
	// 	}))
	// }(ctx, conn)
	// <-ch2
	cancel()
	time.Sleep(time.Second * 3)
	conn.Close()

	select {}
	return
}
