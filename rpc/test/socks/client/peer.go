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
	"github.com/lxt1045/utils/rpc/codec"
	"github.com/lxt1045/utils/rpc/test/socks/pb"
	"github.com/lxt1045/utils/socks"
)

type peer struct {
	Peer rpc.Peer
}

func (p *peer) close(ctx context.Context) (err error) {
	err = p.Peer.Close(ctx)
	return
}

func (p *peer) Close(ctx context.Context, in *pb.CloseReq) (out *pb.CloseRsp, err error) {
	err = p.Peer.Close(ctx)
	return &pb.CloseRsp{}, err
}

func (s *peer) Auth(ctx context.Context, req *pb.AuthReq) (resp *pb.AuthRsp, err error) {

	return
}

func (p *peer) Connect(ctx context.Context, req *pb.ConnectReq) (resp *pb.ConnectRsp, err error) {
	defer p.close(ctx)

	if stream := codec.GetStream(ctx); stream != nil {
		iface, err1 := stream.Recv(ctx)
		if err1 != nil {
			err = err1
			log.Ctx(ctx).Info().Caller().Err(err1).Msg("err")
			return
		}
		req = iface.(*pb.ConnectReq)

		rc, err1 := net.Dial("tcp", req.Addr)
		if err1 != nil {
			log.Ctx(ctx).Error().Caller().Err(err).Msgf("failed to connect to target: %v", err)
			return
		}
		defer rc.Close()
		rc.(*net.TCPConn).SetKeepAlive(true)

		log.Ctx(ctx).Error().Caller().Err(err).Msgf("proxy %s <-> %s", req.Addr, req.Addr)

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
				if l := len(req.Body); l > 0 {
					n, err = rc.Write(req.Body)
					if n < 0 || n < l {

					}
				}
				iface, err = stream.Recv(ctx)
				if err != nil {
					log.Ctx(ctx).Info().Caller().Err(err).Msg("err")
					return
				}
				req = iface.(*pb.ConnectReq)
			}
		}()

		go func() {
			defer func() {
				e := recover()
				if e != nil {
					err = errors.Errorf("recover : %v", e)
					log.Ctx(ctx).Error().Caller().Err(err).Send()
				}
				rc.SetDeadline(time.Now()) // wake up the other goroutine blocking on right
			}()
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
				err1 := stream.Send(ctx, &pb.ConnectRsp{Body: buf})
				if err1 != nil {
					log.Ctx(ctx).Info().Caller().Err(err1).Msg("err")
					return
				}
			}
		}()
		return
	}

	return
}

func (p *peer) connect(ctx context.Context, addr string, rc net.Conn) (err error) {
	defer rc.Close()
	stream, err := p.Peer.Stream(ctx, "Connect")
	if err != nil {
		return
	}

	req := &pb.ConnectReq{
		Addr: addr,
	}
	err = stream.Send(ctx, req)
	if err != nil {
		return
	}

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
			if l := len(req.Body); l > 0 {
				n, err = rc.Write(req.Body)
				if n < 0 || n < l {

				}
			}
			iface, err := stream.Recv(ctx)
			if err != nil {
				log.Ctx(ctx).Info().Caller().Err(err).Msg("err")
				return
			}
			req = iface.(*pb.ConnectReq)
		}
	}()

	go func() {
		defer func() {
			e := recover()
			if e != nil {
				err = errors.Errorf("recover : %v", e)
				log.Ctx(ctx).Error().Caller().Err(err).Send()
			}
			rc.SetDeadline(time.Now()) // wake up the other goroutine blocking on right
		}()
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
			err1 := stream.Send(ctx, &pb.ConnectRsp{Body: buf})
			if err1 != nil {
				log.Ctx(ctx).Info().Caller().Err(err1).Msg("err")
				return
			}
		}
	}()
	return
}

// Listen on addr and proxy to server to reach target from getAddr.
func TCPLocal(ctx context.Context, socksAddr string, p peer) {
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
			}
		}()
	}
}
