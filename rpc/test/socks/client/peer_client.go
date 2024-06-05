package main

import (
	"context"
	"io"
	"math"
	"net"
	"time"

	"github.com/lxt1045/errors"
	"github.com/lxt1045/utils/log"
	"github.com/lxt1045/utils/rpc"
	"github.com/lxt1045/utils/rpc/test/socks/pb"
	"github.com/lxt1045/utils/socks"
)

type socksCli struct {
	Name       string
	LocalAddr  string
	RemoteAddr string
	socksAddr  string
	Peer       rpc.Peer
}

func (p *socksCli) close(ctx context.Context) (err error) {
	err = p.Peer.Close(ctx)
	return
}

func (p *socksCli) Close(ctx context.Context, in *pb.CloseReq) (out *pb.CloseRsp, err error) {
	err = p.Peer.Close(ctx)
	return &pb.CloseRsp{}, err
}

// Listen on addr and proxy to server to reach target from getAddr.
func (p *socksCli) RunLocal(ctx context.Context, socksAddr string) {
	l, err := net.Listen("tcp", socksAddr)
	if err != nil {
		log.Ctx(ctx).Error().Caller().Err(err).Msgf("failed to listen on %s: %v", socksAddr, err)
		return
	}

	for {
		c, err := l.Accept()
		if err != nil {
			log.Ctx(ctx).Error().Caller().Err(err).Msg("failed to accept")
			continue
		}

		go func() {
			defer c.Close()
			c.(*net.TCPConn).SetKeepAlive(true)
			tgtAddr, err := socks.Handshake(c)
			if err != nil {
				// UDP: keep the connection until disconnect then free the UDP socket
				if err == socks.InfoUDPAssociate {
					buf := make([]byte, 1)
					// block here
					for {
						_, err := c.Read(buf)
						if err, ok := err.(net.Error); ok && err.Timeout() {
							continue
						}
						log.Ctx(ctx).Error().Caller().Err(err).Msgf("UDP Associate End.")
						return
					}
				}

				log.Ctx(ctx).Error().Caller().Err(err).Msgf("failed to get target address: %v", err)
				return
			}
			defer c.Close()

			err = p.connect(ctx, tgtAddr.String(), c)
			if err != nil {
				//
				log.Ctx(ctx).Error().Caller().Err(err).Send()
			}
		}()
	}
}

func (p *socksCli) connect(ctx context.Context, addr string, rc net.Conn) (err error) {
	defer rc.Close()
	stream, err := p.Peer.Stream(ctx, "Conn")
	if err != nil {
		log.Ctx(ctx).Error().Caller().Err(err).Send()
		return
	}

	// 建一个协程，处理返回数据
	go func() {
		defer func() {
			e := recover()
			if e != nil {
				err = errors.Errorf("recover : %v", e)
				log.Ctx(ctx).Error().Caller().Err(err).Send()
			}
			rc.SetDeadline(time.Now()) // wake up the other goroutine blocking on right
		}()
		var n int
		for {
			iface, err := stream.Recv(ctx)
			if err != nil {
				log.Ctx(ctx).Info().Caller().Err(err).Msg("err")
				return
			}
			rsp := iface.(*pb.ConnRsp)
			if l := len(rsp.Body); l > 0 {
				n, err = rc.Write(rsp.Body)
				if n < 0 || n < l {
					if err == nil {
						err = errors.Errorf(" n < 0 || n < l")
					}
				}
				if err != nil {
					log.Ctx(ctx).Error().Caller().Err(err).Send()
				}
			}
		}
	}()

	defer func() {
		e := recover()
		if e != nil {
			err = errors.Errorf("recover : %v", e)
			log.Ctx(ctx).Error().Caller().Err(err).Send()
		}
		rc.SetDeadline(time.Now()) // wake up the other goroutine blocking on right
	}()

	req := &pb.ConnReq{
		Addr: addr,
	}
	err = stream.Send(ctx, req)
	if err != nil {
		log.Ctx(ctx).Error().Caller().Err(err).Send()
		return
	}
	for {
		buf := make([]byte, math.MaxUint16/2)
		nr, er := rc.Read(buf)
		if er != nil {
			if er != io.EOF {
				err = er
				log.Ctx(ctx).Error().Caller().Err(err).Msgf("err : %v", err)
			}
			break
		}
		if nr <= 0 {
			continue
		}
		err1 := stream.Send(ctx, &pb.ConnReq{Body: buf[:nr]})
		if err1 != nil {
			log.Ctx(ctx).Info().Caller().Err(err1).Msg("err")
			return
		}
	}
	return
}
