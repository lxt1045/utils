package msg

import (
	"context"

	"github.com/lxt1045/errors"
	"github.com/quic-go/quic-go"
)

type QuicConn struct {
	quic.Connection
	quic.Stream
}

func NewQuicConn(ctx context.Context, c quic.Connection) (qc *QuicConn, err error) {
	stream, err := c.AcceptStream(ctx)
	if err != nil {
		err = errors.Errorf(err.Error())
		return
	}
	qc = &QuicConn{
		Connection: c,
		Stream:     stream,
	}
	return
}

func (c *QuicConn) Close() (err error) {
	if c.Stream != nil {
		return c.Close()
	}
	return
}
