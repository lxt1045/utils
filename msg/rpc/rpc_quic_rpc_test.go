package rpc

import (
	"context"
	"io/fs"
	"strings"
	"testing"
	"time"

	"github.com/lxt1045/errors"
	"github.com/lxt1045/utils/config"
	"github.com/lxt1045/utils/log"
	"github.com/lxt1045/utils/msg/conn"
	base "github.com/lxt1045/utils/msg/rpc/base"
	"github.com/lxt1045/utils/msg/test/filesystem"
	"github.com/quic-go/quic-go"
)

func TestQuicRPC(t *testing.T) {
	ctx := context.Background()
	addr := ":18080"
	ch := make(chan struct{})
	go func() {
		rpcService(ctx, t, addr, ch)
	}()

	<-ch
	time.Sleep(time.Second * 1)
	rpcClient(ctx, t, addr)
}

func rpcService(ctx context.Context, t *testing.T, addr string, ch chan struct{}) {
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

	ch <- struct{}{}
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

		if err != nil {
			t.Fatal(err)
		}
		svc, err := conn.WrapQuic(ctx, c)
		if err != nil {
			t.Fatal(err)
		}
		zsvc, err := conn.NewZip(ctx, svc)
		if err != nil {
			t.Fatal(err)
		}

		_, err = NewPeer(ctx, zsvc, &server{Str: "test"}, nil, []interface{}{base.RegisterHelloServer})
		if err != nil {
			t.Fatal(err)
		}
	}
}

func rpcClient(ctx context.Context, t *testing.T, addr string) {
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

	c, err := quic.DialAddr(ctx, conf.ClientConn.Addr, tlsConfig, nil)
	if err != nil {
		log.Ctx(ctx).Fatal().Caller().Err(err).Send()
		return
	}
	cli, err := conn.WrapQuicClient(ctx, c)
	if err != nil {
		t.Fatal(err)
	}
	zcli, err := conn.NewZip(ctx, cli)
	if err != nil {
		t.Fatal(err)
	}

	client, err := NewPeer(ctx, zcli, nil, []interface{}{base.NewHelloClient}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close(ctx)
	if err != nil {
		t.Fatal(err)
	}

	req := base.HelloReq{
		Name: "call by quic 007",
	}

	ir, err := client.Invoke(ctx, "SayHello", &req)
	if err != nil {
		t.Fatal(err)
	}

	resp, ok := ir.(*base.HelloRsp)
	if !ok {
		t.Fatal("!ok")
	}
	t.Logf("resp.Msg:\"%s\"", resp.Msg)
}
