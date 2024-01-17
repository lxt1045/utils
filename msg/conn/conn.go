package conn

import (
	"context"
	"net"

	"github.com/lxt1045/utils/log"
)

type Conn struct {
	net.Conn
}

func Wrap(ctx context.Context, c net.Conn) (conn *Conn, err error) {
	conn = &Conn{Conn: c}
	return
}

// func (c *Conn) Read(data []byte) (n int, err error) {
// 	return c.Conn.Read(data)
// }

// func (c *Conn) Write(data []byte) (n int, err error) {
// 	return c.Conn.Write(data)
// }

func (c *Conn) WriteMul(datas [][]byte) (n int, err error) {
	n1 := 0
	for _, data := range datas {
		n1, err = c.Conn.Write(data)
		if err != nil {
			return
		}
		n += n1
	}
	return
}

// func (c *Conn) Close() (err error) {
// 	return c.Conn.Close()
// }

// SetReadWriteBuff 设置 conn 读写缓存的大小
func SetReadWriteBuff(ctx context.Context, conn net.Conn, read, write int) (err error) {
	netConn, ok := conn.(interface {
		NetConn() net.Conn
	})
	if ok {
		connTCP := netConn.NetConn()
		setBuffer, ok := connTCP.(interface {
			SetReadBuffer(bytes int) error
			SetWriteBuffer(bytes int) error
		})
		if ok {
			if read != 0 {
				err = setBuffer.SetReadBuffer(read)
				if err != nil {
					log.Ctx(ctx).Error().Err(err).Send()
				}
			}
			if write != 0 {
				err = setBuffer.SetWriteBuffer(write)
				if err != nil {
					log.Ctx(ctx).Error().Err(err).Send()
				}
			}
		}
	}
	return
}
