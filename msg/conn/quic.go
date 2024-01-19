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

	// qc.ConnectionState()
	return
}

func WrapQuicClient(ctx context.Context, c quic.Connection) (qc *QuicConn, err error) {
	stream, err := c.OpenStreamSync(ctx)
	if err != nil {
		err = errors.Errorf(err.Error())
		return
	}
	qc = &QuicConn{
		Connection: c,
		Stream:     stream,
	}

	// qc.ConnectionState()
	return
}
