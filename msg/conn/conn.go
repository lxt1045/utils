package conn

import (
	"context"
	"net"
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
