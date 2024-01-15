package conn

import (
	"context"
	"io"
	"net"
	"sync"

	"github.com/klauspost/compress/zstd"
	"github.com/lxt1045/errors"
)

type Zip struct {
	net.Conn
	wLock  sync.Mutex // zstd 不是线程安全的，所以需要加锁
	rLock  sync.Mutex
	writer *zstd.Encoder // 需要注意保护，非协程安全
	reader *zstd.Decoder // 需要注意保护，非协程安全
}

func WrapCoder(ctx context.Context, conn net.Conn) (c *Zip, err error) {
	c = &Zip{
		Conn: conn,
	}

	wops := []zstd.EOption{
		zstd.WithEncoderLevel(zstd.SpeedFastest),
		zstd.WithEncoderConcurrency(1),
		zstd.WithWindowSize(1 << 20),
	}
	c.writer, err = zstd.NewWriter(c.Conn, wops...)
	if err != nil {
		err = errors.Errorf(err.Error())
		return
	}

	rops := []zstd.DOption{
		zstd.WithDecoderConcurrency(1),
		zstd.WithDecoderMaxWindow(1 << 20),
	}
	c.reader, err = zstd.NewReader(c.Conn, rops...)
	if err != nil {
		err = errors.Errorf(err.Error())
		return
	}
	return
}

var (
	ErrDecoderClosed = zstd.ErrDecoderClosed
	ErrUnexpectedEOF = io.ErrUnexpectedEOF
)

func (c *Zip) Read(data []byte) (n int, err error) {
	c.rLock.Lock()
	defer c.rLock.Unlock()
	n, err = c.reader.Read(data)
	if err != nil {
		if errors.Is(zstd.ErrDecoderClosed, err) {
			err = ErrDecoderClosed
		} else if errors.Is(io.ErrUnexpectedEOF, err) || errors.Is(io.EOF, err) {
			err = ErrUnexpectedEOF
		} else {
			err = errors.Errorf("err:%v, ip:%s", err.Error(), c.Conn.RemoteAddr())
		}
	}
	return
}

func (c *Zip) Write(data []byte) (n int, err error) {
	c.wLock.Lock()
	defer c.wLock.Unlock()
	n, err = c.writer.Write(data)
	if err != nil {
		err = errors.Errorf(err.Error())
		return
	}

	err = c.writer.Flush()
	return
}

func (c *Zip) WriteMul(datas [][]byte) (n int, err error) {
	c.wLock.Lock()
	defer c.wLock.Unlock()

	n1 := 0
	for _, data := range datas {
		n1, err = c.writer.Write(data)
		if err != nil {
			err = errors.Errorf(err.Error())
			return
		}
		n += n1
	}

	err = c.writer.Flush()
	return
}

func (c *Zip) Close() (err error) {
	defer func() {
		e := recover() // maybe panic
		if e != nil {
			err = errors.Errorf("recover:%+v", e)
		}
	}()
	err = c.Conn.Close() // conn 关闭后， c.reader 和 c.writer 的阻塞点就会及时返回

	if c.reader != nil {
		c.rLock.Lock()
		defer c.rLock.Unlock()
		c.reader.Close()
	}

	if c.writer != nil {
		c.wLock.Lock()
		defer c.wLock.Unlock()
		err1 := c.writer.Close()
		if err == nil && err1 != nil {
			err = err1
		}
	}

	return
}
