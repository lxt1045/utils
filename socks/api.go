package socks

import (
	"context"
	"io"
	"net"
	"sync"
	"time"

	"github.com/lxt1045/errors"
	"github.com/lxt1045/utils/log"
)

// Listen on addr and proxy to server to reach target from getAddr.
func TCPLocalOnly(ctx context.Context, socksAddr string, getAddr func(net.Conn) (Addr, error)) {
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
			tgt, err := getAddr(c)
			if err != nil {
				// UDP: keep the connection until disconnect then free the UDP socket
				if err == InfoUDPAssociate {
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
			rc, err := net.Dial("tcp", tgt.String())
			if err != nil {
				log.Ctx(ctx).Error().Caller().Err(err).Msgf("failed to connect to target: %v", err)
				return
			}
			defer rc.Close()
			rc.(*net.TCPConn).SetKeepAlive(true)

			log.Ctx(ctx).Error().Caller().Err(err).Msgf("proxy %s <-> %s", c.RemoteAddr(), tgt)
			_, _, err = Relay(ctx, c, rc)
			if err != nil {
				if err, ok := err.(net.Error); ok && err.Timeout() {
					return // ignore i/o timeout
				}
				log.Ctx(ctx).Error().Caller().Err(err).Msgf("relay error: %v", err)
			}
		}()
	}
}

// Relay copies between left and right bidirectionally. Returns number of
// bytes copied from right to left, from left to right, and any error occurred.
func Relay(ctx context.Context, left, right net.Conn) (int64, int64, error) {
	type res struct {
		N   int64
		Err error
	}
	ch := make(chan res)

	go func() {
		n, err := Copy(ctx, right, left)
		right.SetDeadline(time.Now()) // wake up the other goroutine blocking on right
		left.SetDeadline(time.Now())  // wake up the other goroutine blocking on left
		ch <- res{n, err}
	}()

	n, err := Copy(ctx, left, right)
	right.SetDeadline(time.Now()) // wake up the other goroutine blocking on right
	left.SetDeadline(time.Now())  // wake up the other goroutine blocking on left
	rs := <-ch

	if err == nil {
		err = rs.Err
	}
	return n, rs.N, err
}

var (
	errInvalidWrite = errors.New("invalid write result")
	bsPool          = sync.Pool{
		New: func() interface{} {
			return make([]byte, 1024*16)
		},
	}
)

func Copy(ctx context.Context, dst io.Writer, src io.Reader) (written int64, err error) {
	defer func() {
		e := recover()
		if e != nil {
			err = errors.Errorf("recover : %v", e)
			log.Ctx(ctx).Error().Caller().Err(err).Send()
		}
	}()
	// return io.Copy(dst, src)
	ch := make(chan []byte, 1024*32)

	go func() {
		for {
			bs, ok := <-ch
			if !ok && len(bs) == 0 {
				break
			}
			nw, ew := dst.Write(bs)
			nr := len(bs)
			bsPool.Put(bs)
			if nw < 0 || nr < nw {
				nw = 0
				if ew == nil {
					ew = errInvalidWrite
				}
			}
			written += int64(nw)
			if ew != nil {
				err = ew
				log.Ctx(ctx).Error().Caller().Err(err).Msgf("err : %v", err)
				break
			}
			if nr != nw {
				err = io.ErrShortWrite
				log.Ctx(ctx).Error().Caller().Err(err).Msgf("err : %v", err)
				break
			}

		}
	}()
	for {
		buf := bsPool.Get().([]byte)
		nr, er := src.Read(buf)
		if nr > 0 {
			ch <- buf[:nr]
		}
		if er != nil {
			if er != io.EOF {
				err = er
				log.Ctx(ctx).Error().Caller().Err(err).Msgf("err : %v", err)
			}
			break
		}
	}
	close(ch)
	return written, err
}

func Copy1(dst io.Writer, src io.Reader) (written int64, err error) {
	return io.Copy(dst, src)
}
