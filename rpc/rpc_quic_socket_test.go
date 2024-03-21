package rpc

import (
	"context"
	"crypto/tls"
	"io/fs"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/lxt1045/errors"
	"github.com/lxt1045/utils/config"
	"github.com/lxt1045/utils/log"
	base "github.com/lxt1045/utils/rpc/base"
	"github.com/lxt1045/utils/rpc/conn"
	"github.com/lxt1045/utils/rpc/socket"
	"github.com/lxt1045/utils/rpc/test/filesystem"
	"github.com/quic-go/quic-go"
)

func TestQuicSocket(t *testing.T) {
	ctx := context.Background()
	ch := make(chan struct{})
	go func() {
		quicServiceSocket(ctx, t, ch)
	}()

	<-ch
	time.Sleep(time.Second * 1)
	quicClientSocket(ctx, t)
}

type ConfigSocket struct {
	Debug      bool
	Pprof      bool
	Dev        bool
	Conn       config.Conn
	ClientConn config.Conn
	Log        config.Log
}

func quicServiceSocket(ctx context.Context, t *testing.T, ch chan struct{}) {
	conf := &ConfigSocket{}
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
	// listener, err := quic.ListenAddr(conf.Conn.Addr, tlsConfig, nil)
	listener, err := quicListenAddr(ctx, conf.Conn.Addr, tlsConfig, nil)
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

		svc, err := conn.WrapQuic(ctx, c)
		if err != nil {
			t.Fatal(err)
		}
		zsvc, err := conn.NewZip(ctx, svc)
		if err != nil {
			t.Fatal(err)
		}

		_, err = StartService(ctx, zsvc, &server{Str: "test"}, base.RegisterHelloServer)
		if err != nil {
			t.Fatal(err)
		}
	}
}

func quicClientSocket(ctx context.Context, t *testing.T) {
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

	// c, err := quic.DialAddr(ctx, conf.ClientConn.Addr, tlsConfig, nil)
	c, err := quicDialAddr(ctx, conf.ClientConn.Addr, tlsConfig, nil)
	if err != nil {
		log.Ctx(ctx).Fatal().Caller().Err(err).Send()
		return
	}
	log.Ctx(ctx).Info().Caller().Str("local", c.LocalAddr().String()).Str("remote", c.RemoteAddr().String()).Send()

	cli, err := conn.WrapQuicClient(ctx, c)
	if err != nil {
		t.Fatal(err)
	}
	zcli, err := conn.NewZip(ctx, cli)
	if err != nil {
		t.Fatal(err)
	}

	client, err := StartClient(ctx, zcli, base.NewHelloClient)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close(ctx)

	req := base.HelloReq{
		Name: "call by quic 007",
	}
	resp := base.HelloRsp{}

	err = client.Invoke(ctx, "SayHello", &req, &resp)
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("resp.Msg:\"%s\"", resp.Msg)
}

func quicListenAddr(ctx context.Context, addr string, tlsConf *tls.Config, config *quic.Config) (ln *quic.Listener, err error) {
	conn, err := socket.ListenPacket(ctx, "udp4", addr)
	if err != nil {
		return
	}
	ln, err = quic.Listen(conn, tlsConf, config)
	if err != nil {
		err = errors.Errorf(err.Error())
		return
	}
	return
}

// listener, err := quic.ListenAddr(conf.Conn.TCP, tlsConfig, nil)
func quicListenAddr1(addr string, tlsConf *tls.Config, config *quic.Config) (ln *quic.Listener, err error) {
	udpAddr, err := net.ResolveUDPAddr("udp", addr)
	if err != nil {
		err = errors.Errorf(err.Error())
		return
	}
	conn, err := net.ListenUDP("udp", udpAddr)
	if err != nil {
		err = errors.Errorf(err.Error())
		return
	}
	ln, err = quic.Listen(conn, tlsConf, config)
	if err != nil {
		err = errors.Errorf(err.Error())
		return
	}
	return
}

// c, err := quicDialAddr(ctx, conf.ClientConn.Addr, tlsConfig, nil)
func quicDialAddr(ctx context.Context, addr string, tlsConf *tls.Config, conf *quic.Config) (conn quic.Connection, err error) {
	// udpConn, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.IPv4zero, Port: 0})
	// if err != nil {
	// 	return nil, err
	// }

	// udpConn, err := socket.ListenPacket(ctx, "udp4", "0.0.0.0:0")  // 端口 0 会自动分配一个
	udpConn, err := socket.ListenPacket(ctx, "udp4", "0.0.0.0:8081")
	if err != nil {
		return
	}
	udpAddr, err := net.ResolveUDPAddr("udp", addr)
	if err != nil {
		return
	}
	conn, err = quic.Dial(ctx, udpConn, udpAddr, tlsConf, conf)
	if err != nil {
		return
	}
	return
}

// c, err := quicDialAddr(ctx, conf.ClientConn.Addr, tlsConfig, nil)
func quicDialAddr1(ctx context.Context, addr string, tlsConf *tls.Config, conf *quic.Config) (conn quic.Connection, err error) {
	udpConn, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.IPv4zero, Port: 0})
	if err != nil {
		return nil, err
	}
	udpAddr, err := net.ResolveUDPAddr("udp", addr)
	if err != nil {
		return
	}
	conn, err = quic.Dial(ctx, udpConn, udpAddr, tlsConf, conf)
	if err != nil {
		return
	}
	return
}
