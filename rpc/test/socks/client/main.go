package main

import (
	"context"
	"flag"
	"fmt"
	"io/fs"
	"net"
	"os"
	"os/signal"
	"sync/atomic"
	"syscall"

	"github.com/lxt1045/errors"
	"github.com/lxt1045/utils/config"
	"github.com/lxt1045/utils/gid"
	"github.com/lxt1045/utils/log"
	"github.com/lxt1045/utils/rpc"
	"github.com/lxt1045/utils/rpc/socket"
	"github.com/lxt1045/utils/rpc/test/filesystem"
	"github.com/lxt1045/utils/rpc/test/socks/pb"
	"github.com/lxt1045/utils/socks"
)

type Config struct {
	Debug      bool
	Pprof      bool
	Dev        bool
	Conn       config.Conn
	ClientConn config.Conn
	Log        config.Log
}

var (
	nConnect uint32
)

func main() {
	var flags struct {
		Client  string
		Server  string
		Verbose bool
		Socks   string
		Proxy   bool
	}

	flag.BoolVar(&flags.Verbose, "verbose", true, "verbose mode")
	flag.BoolVar(&flags.Proxy, "proxy", false, "verbose mode")
	flag.StringVar(&flags.Server, "s", "", "server listen address or url")
	flag.StringVar(&flags.Client, "c", "client-9527", "client connect address or url")
	flag.StringVar(&flags.Socks, "socks", ":10086", "(client-only) SOCKS listen address")
	flag.Parse()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ctx, _ = log.WithLogid(ctx, gid.GetGID())

	// 解析配置文件
	conf := &Config{}
	file := "static/conf/default.yml"
	bs, err := fs.ReadFile(filesystem.Static, file)
	if err != nil {
		err = errors.Errorf(err.Error())
		log.Ctx(ctx).Error().Caller().Err(err).Send()
	}
	err = config.Unmarshal(bs, conf)
	if err != nil {
		log.Ctx(ctx).Error().Caller().Err(err).Send()
	}

	if err != nil {
		log.Ctx(ctx).Error().Caller().Err(err).Send()
		return
	}

	cmtls := conf.ClientConn.TLS
	tlsConfig, err := config.LoadTLSConfig(filesystem.Static, cmtls.ClientCert, cmtls.ClientKey, cmtls.CACert)
	if err != nil {
		log.Ctx(ctx).Error().Caller().Err(err).Send()
		return
	}
	tlsConfig.ServerName = conf.ClientConn.Host

	// conn, err := tls.Dial("tcp", conf.ClientConn.Addr, tlsConfig)
	conn, err := socket.DialTLS(ctx, "tcp", conf.ClientConn.Addr, tlsConfig)
	if err != nil {
		log.Ctx(ctx).Error().Caller().Err(err).Send()
		return
	}
	log.Ctx(ctx).Info().Caller().Str("local", conn.LocalAddr().String()).Str("remote", conn.RemoteAddr().String()).Send()

	cli := &client{
		Name:       flags.Client,
		LocalAddr:  conn.LocalAddr().String(),
		RemoteAddr: conn.RemoteAddr().String(),
	}
	var _ pb.ClientServer = cli
	cli.Peer, err = rpc.StartPeer(ctx, conn, cli, pb.RegisterClientServer, pb.NewServiceClient)
	if err != nil {
		log.Ctx(ctx).Error().Caller().Err(err).Send()
		return
	}

	err = cli.Auth(ctx)
	if err != nil {
		log.Ctx(ctx).Error().Caller().Err(err).Send()
		return
	}

	clients, err := cli.Clients(ctx)
	if err != nil {
		log.Ctx(ctx).Error().Caller().Err(err).Send()
		return
	}
	if len(clients) == 0 {
		log.Ctx(ctx).Error().Caller().Msg("peer client is not exist")

		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh
		return
	}

	log.Ctx(ctx).Info().Caller().Interface("clients", clients).Send()

	go func(addr string) {
		ln, err := socket.Listen(ctx, "tcp4", addr)
		if err != nil {
			log.Ctx(ctx).Error().Caller().Err(err).Msg("listen failed")
			return
		}

		buf := make([]byte, 1024)
		for {
			connPeer, err := ln.Accept()
			if err != nil {
				fmt.Println("accept failed", err)
				continue
			}
			log.Ctx(ctx).Info().Caller().Str("local", connPeer.LocalAddr().String()).Str("remote", connPeer.RemoteAddr().String()).Send()
			for {
				n, err := connPeer.Read(buf)
				if err != nil {
					fmt.Println("read failed", err)
					break
				}
				log.Ctx(ctx).Info().Caller().Str("read", string(buf[:n])).
					Str("local", connPeer.LocalAddr().String()).Str("remote", connPeer.RemoteAddr().String()).Send()
			}
			err = connectPeer(ctx, connPeer)
			if err != nil {
				log.Ctx(ctx).Error().Caller().Err(err).Send()
				return
			}
		}
	}(":" + localPort(conn))

	target := clients[0]
	req := &pb.ConnToReq{
		Client: target,
	}
	resp := &pb.ConnToRsp{}
	err = cli.Peer.Invoke(ctx, "ConnTo", req, resp)
	if err != nil {
		log.Ctx(ctx).Error().Caller().Err(err).Send()
		return
	}
	log.Ctx(ctx).Error().Caller().Interface("resp", resp).Send()

	go func() {
		req := &pb.ConnToReq{
			Timestamp: resp.Timestamp,
			Client:    target,
		}
		resp, err := cli.ConnTo(ctx, req)
		if err != nil {
			log.Ctx(ctx).Error().Caller().Err(err).Send()
			return
		}
		_ = resp

	}()

	log.Ctx(ctx).Error().Caller().Msgf("SOCKS proxy on %s", flags.Socks)
	go socks.TCPLocalOnly(ctx, flags.Socks, func(c net.Conn) (socks.Addr, error) {
		return socks.Handshake(c)
	})

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh
}

func connectPeer(ctx context.Context, conn net.Conn) (err error) {
	me := &peer{}
	me.Peer, err = rpc.StartPeer(ctx, conn, me, pb.RegisterPeerServer, pb.NewPeerClient)
	if err != nil {
		return
	}

	n := atomic.AddUint32(&nConnect, 1)
	if n > 1 {
		err = me.Peer.Close(ctx)
		if err != nil {
			log.Ctx(ctx).Error().Caller().Err(err).Send()
			return
		}
	}

	return
}

func main1() {

	var flags struct {
		Client  string
		Server  string
		Verbose bool
		Socks   string
		Proxy   bool
	}

	flag.BoolVar(&flags.Verbose, "verbose", true, "verbose mode")
	flag.BoolVar(&flags.Proxy, "proxy", false, "verbose mode")
	flag.StringVar(&flags.Server, "s", "", "server listen address or url")
	flag.StringVar(&flags.Client, "c", "", "client connect address or url")
	flag.StringVar(&flags.Socks, "socks", ":10086", "(client-only) SOCKS listen address")
	flag.Parse()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ctx, _ = log.WithLogid(ctx, gid.GetGID())

	log.Ctx(ctx).Error().Caller().Msgf("SOCKS proxy on %s", flags.Socks)
	go socks.TCPLocalOnly(ctx, flags.Socks, func(c net.Conn) (socks.Addr, error) {
		return socks.Handshake(c)
	})

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh
}
