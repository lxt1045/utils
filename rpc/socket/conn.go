package socket

import (
	"context"
	"crypto/tls"
	"net"
	"syscall"

	"github.com/lxt1045/errors"
)

func control(network, address string, c syscall.RawConn) error {
	// windows
	if TCP_SYNCNT == 0 {
		return c.Control(func(fd uintptr) {
			h := newHandle(fd).H
			syscall.SetsockoptInt(h, syscall.SOL_SOCKET, SO_REUSEADDR, 1)
			syscall.SetsockoptInt(h, syscall.SOL_SOCKET, SO_REUSEPORT, 1)
		})
	}
	// linux
	return c.Control(func(fd uintptr) {
		h := newHandle(fd).H
		syscall.SetsockoptInt(h, syscall.SOL_SOCKET, SO_REUSEADDR, 1)
		syscall.SetsockoptInt(h, syscall.SOL_SOCKET, SO_REUSEPORT, 1)
		syscall.SetsockoptInt(h, syscall.IPPROTO_TCP, TCP_SYNCNT, 1)
	})
}

func Listen(ctx context.Context, network, addr string) (ln net.Listener, err error) {
	cfg := net.ListenConfig{
		Control: control,
	}
	ln, err = cfg.Listen(ctx, network, addr)
	if err != nil {
		err = errors.Errorf(err.Error())
		return
	}
	return
}

func Dial(ctx context.Context, network, addr string) (conn net.Conn, err error) {
	cfg := net.Dialer{
		Control: control,
	}
	conn, err = cfg.DialContext(ctx, network, addr)
	if err != nil {
		err = errors.Errorf(err.Error())
		return
	}
	return

}

func DialTLS(ctx context.Context, network, addr string, conf *tls.Config) (conn *tls.Conn, err error) {
	dialer := &tls.Dialer{
		NetDialer: &net.Dialer{
			Control: control,
		},
		Config: conf,
	}
	c, err := dialer.DialContext(ctx, network, addr)
	if err != nil {
		err = errors.Errorf(err.Error())
		return
	}
	conn = c.(*tls.Conn)
	return

}

func DialTLS1(ctx context.Context, network, addr string, conf *tls.Config) (conn *tls.Conn, err error) {
	dialer := net.Dialer{
		Control: control,
	}
	conn, err = tls.DialWithDialer(&dialer, network, addr, conf)
	if err != nil {
		err = errors.Errorf(err.Error())
		return
	}
	return

}
