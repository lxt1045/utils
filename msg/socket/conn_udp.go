package socket

import (
	"context"
	"fmt"
	"net"
	"syscall"

	"github.com/lxt1045/utils/log"
)

func ListenPacket(ctx context.Context, network, addr string) {
	cfg := net.ListenConfig{
		Control: func(network, address string, c syscall.RawConn) error {
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
		},
	}
	udp, err := cfg.ListenPacket(ctx, network, addr)
	if err != nil {
		log.Ctx(ctx).Error().Caller().Err(err).Msg("listen failed")
		return
	}

	buf := make([]byte, 1024)
	for {
		n, caddr, err := udp.ReadFrom(buf)
		if err != nil {
			fmt.Println("read failed", err)
			continue
		}

		fmt.Println(addr, caddr, string(buf[:n]))
	}
}

func ConnectPacket(ctx context.Context, network, addr string) {
	cfg := net.Dialer{
		Control: func(network, address string, c syscall.RawConn) error {
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
		},
	}
	// net.DialUDP("udp", nil, addr)
	conn, err := cfg.DialContext(ctx, network, addr)
	if err != nil {
		log.Ctx(ctx).Error().Caller().Err(err).Msg("listen failed")
		return
	}

	udpconn := conn.(*net.UDPConn)
	buf := make([]byte, 1024)
	for {
		n, caddr, err := udpconn.ReadFrom(buf)
		if err != nil {
			fmt.Println("read failed", err)
			break
		}

		fmt.Println(addr, caddr, string(buf[:n]))
	}
}
