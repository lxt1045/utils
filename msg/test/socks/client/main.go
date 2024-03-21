package main

import (
	"context"
	"flag"
	"fmt"
	"io/fs"
	"net"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/lxt1045/errors"
	"github.com/lxt1045/utils/config"
	"github.com/lxt1045/utils/gid"
	"github.com/lxt1045/utils/log"
	"github.com/lxt1045/utils/msg/rpc"
	"github.com/lxt1045/utils/msg/socket"
	"github.com/lxt1045/utils/msg/test/filesystem"
	"github.com/lxt1045/utils/msg/test/pb"
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

	cli := NewClient("")
	cli.Peer, err = rpc.StartPeer(ctx, conn, cli, pb.RegisterClientServer, pb.NewServiceClient)
	if err != nil {
		log.Ctx(ctx).Error().Caller().Err(err).Send()
		return
	}

	req := &pb.ClientsReq{
		MyName: "client-" + strconv.Itoa(int(time.Now().UnixNano())),
	}
	resp := &pb.ClientsRsp{}
	err = cli.Peer.Invoke(ctx, "Clients", req, resp)
	if err != nil {
		log.Ctx(ctx).Error().Caller().Err(err).Send()
		return
	}
	log.Ctx(ctx).Error().Caller().Interface("resp", resp).Send()

	addrs := strings.Split(conn.LocalAddr().String(), ":")
	go func(addr string) {
		ln, err := socket.Listen(ctx, "tcp4", addr)
		if err != nil {
			log.Ctx(ctx).Error().Caller().Err(err).Msg("listen failed")
			return
		}

		buf := make([]byte, 1024)
		for {
			conn, err := ln.Accept()
			if err != nil {
				fmt.Println("accept failed", err)
				continue
			}
			for {
				n, err := conn.Read(buf)
				if err != nil {
					fmt.Println("read failed", err)
					break
				}
				log.Ctx(ctx).Info().Caller().Str("read", string(buf[:n])).
					Str("local", conn.LocalAddr().String()).Str("remote", conn.RemoteAddr().String()).Send()
			}
		}
	}(":" + addrs[len(addrs)-1])

	if len(resp.Clients) > 0 {
		target := resp.Clients[0]
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

		_, err = cli.ConnTo(ctx, &pb.ConnToReq{
			Client:    target,
			Timestamp: resp.Timestamp,
		})
		if err != nil {
			log.Ctx(ctx).Error().Caller().Err(err).Send()
			return
		}
	}

	select {}
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
