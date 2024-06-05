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
)

type socksSvc struct {
	Name       string
	LocalAddr  string
	RemoteAddr string
	Peer       rpc.Peer
}

func (p *socksSvc) close(ctx context.Context) (err error) {
	err = p.Peer.Close(ctx)
	return
}

func (p *socksSvc) Close(ctx context.Context, in *pb.CloseReq) (out *pb.CloseRsp, err error) {
	err = p.close(ctx)
	return &pb.CloseRsp{}, err
}

func (s *socksSvc) Auth(ctx context.Context, req *pb.AuthReq) (resp *pb.AuthRsp, err error) {

	return
}

func (p *socksSvc) Conn(ctx context.Context, req *pb.ConnReq) (resp *pb.ConnRsp, err error) {
	// defer p.close(ctx)
	if stream := codec.GetStream(ctx); stream != nil {
		rc, err1 := net.Dial("tcp", req.Addr)
		if err1 != nil {
			err = err1
			log.Ctx(ctx).Error().Caller().Err(err).Msgf("failed to connect to target: %v", err)
			return
		}
		rc.(*net.TCPConn).SetKeepAlive(true)

		log.Ctx(ctx).Info().Caller().Err(err).Msgf("proxy %s <-> %s", p.RemoteAddr, req.Addr)

		go func() {
			defer func() {
				e := recover()
				if e != nil {
					err = errors.Errorf("recover : %v", e)
					log.Ctx(ctx).Error().Caller().Err(err).Send()
				}
				rc.SetDeadline(time.Now()) // wake up the other goroutine blocking on right
				// p.close(ctx)
			}()
			var n int
			for {
				if l := len(req.Body); l > 0 {
					n, err = rc.Write(req.Body)
					if n < 0 || n < l {
						if err == nil {
							err = errors.Errorf(" n < 0 || n < l")
						}
					}
					if err != nil {
						log.Ctx(ctx).Error().Caller().Err(err).Send()
					}
				}
				iface, err := stream.Recv(ctx)
				if err != nil {
					log.Ctx(ctx).Info().Caller().Err(err).Msg("err")
					return
				}
				req = iface.(*pb.ConnReq)
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
				// p.close(ctx)
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
				err1 := stream.Send(ctx, &pb.ConnRsp{Body: buf[:nr]})
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
