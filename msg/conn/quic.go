package conn

import (
	"context"

	"github.com/lxt1045/errors"
	"github.com/quic-go/quic-go"
)

type QuicConn struct {
	quic.Connection
	quic.Stream
}

func WrapQuic(ctx context.Context, c quic.Connection) (qc *QuicConn, err error) {
	stream, err := c.AcceptStream(ctx)
	if err != nil {
		err = errors.Errorf(err.Error())
		return
	}
	qc = &QuicConn{
		Connection: c,
		Stream:     stream,
	}

	qc.ConnectionState()
	return
}

func (c *QuicConn) WriteMul(datas [][]byte) (n int, err error) {
	n1 := 0
	for _, data := range datas {
		n1, err = c.Stream.Write(data)
		if err != nil {
			return
		}
		n += n1
	}
	return
}
