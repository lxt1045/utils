package main

import (
	"context"
	"fmt"
	"net"
	"runtime"
	"strings"
	"time"

	"github.com/lxt1045/errors"
	"github.com/lxt1045/utils/log"
	"github.com/lxt1045/utils/rpc"
	"github.com/lxt1045/utils/rpc/socket"
	"github.com/lxt1045/utils/rpc/test/socks/pb"
)

type client struct {
	Name       string
	LocalAddr  string
	RemoteAddr string
	Peer       rpc.Peer
	*pb.ClientInfo
}

func (c *client) Latency(ctx context.Context, in *pb.LatencyReq) (out *pb.LatencyRsp, err error) {
	return &pb.LatencyRsp{
		Ts: time.Now().UnixNano(),
	}, nil
}

func (c *client) Close(ctx context.Context, in *pb.CloseReq) (out *pb.CloseRsp, err error) {
	err = c.Peer.Close(ctx)
	return &pb.CloseRsp{}, err
}

func (c *client) ConnTo(ctx context.Context, in *pb.ConnToReq) (resp *pb.ConnToRsp, err error) {
	// 先 listen

	// 再 连接
	tsDo := in.Timestamp
	tsNow := time.Now().UnixNano()
	if tsDo < tsNow {
		err = errors.Errorf("tsDo: %d, tsNow: %d, time out", tsDo, tsNow)
		return
	}

	tsDiff := tsNow - tsDo - int64(time.Millisecond)*1
	time.Sleep(time.Duration(tsDiff))
	for {
		tsNow := time.Now().UnixNano()
		if tsDo <= tsNow {
			break
		}
		runtime.Gosched()
	}

	conn, err := socket.Dial(ctx, "tcp4", c.LocalAddr, in.Client.Addr)
	if err != nil {
		log.Ctx(ctx).Error().Caller().Err(err).Msg("Dial failed")
		return
	}

	str := "Hello " + in.Client.Name
	_, err = conn.Write([]byte(str))
	if err != nil {
		log.Ctx(ctx).Error().Caller().Err(err).Msg("Dial failed")
		return
	}
	log.Ctx(ctx).Info().Caller().Str("Write", str).
		Str("local", conn.LocalAddr().String()).Str("remote", conn.RemoteAddr().String()).Send()

	for {
		buf := []byte("hello")
		n, err := conn.Write(buf)
		if err != nil {
			fmt.Println("write failed", err)
			break
		}
		log.Ctx(ctx).Info().Caller().Str("write", string(buf[:n])).
			Str("local", conn.LocalAddr().String()).Str("remote", conn.RemoteAddr().String()).Send()
		time.Sleep(time.Second)
	}
	err = connectPeer(ctx, conn)
	if err != nil {
		log.Ctx(ctx).Error().Caller().Err(err).Send()
		return
	}
	return
}

func (c *client) Auth(ctx context.Context) (err error) {
	req := &pb.AuthReq{
		Name: c.Name,
	}
	resp := &pb.AuthRsp{}
	err = c.Peer.Invoke(ctx, "Auth", req, resp)
	if err != nil {
		log.Ctx(ctx).Error().Caller().Err(err).Send()
		return
	}

	return
}

func (c *client) Clients(ctx context.Context) (clients []*pb.ClientInfo, err error) {
	req := &pb.ClientsReq{
		MyName: c.Name,
	}
	resp := &pb.ClientsRsp{}
	err = c.Peer.Invoke(ctx, "Clients", req, resp)
	if err != nil {
		log.Ctx(ctx).Error().Caller().Err(err).Send()
		return
	}

	clients = resp.Clients
	return
}

func localPort(conn net.Conn) string {
	addrs := strings.Split(conn.LocalAddr().String(), ":")
	return addrs[len(addrs)-1]
}
