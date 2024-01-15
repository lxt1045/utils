package msg

import (
	"context"
	"io"
	"net"
	"reflect"
	"sync"

	"github.com/gogo/protobuf/proto"
	"github.com/lxt1045/errors"
	"github.com/lxt1045/utils/log"
)

type MsgType uint16

type Msg interface {
	proto.Message
	MsgType() MsgType
}

var (
	msgsLock sync.Mutex
	msgs     = [0xffff]reflect.Type{}
)

// 注册消息类型
func Rigester(msg Msg) (err error) {
	typ := reflect.TypeOf(msg)
	if typ.Kind() == reflect.Ptr {
		typ = typ.Elem()
		if typ.Kind() == reflect.Ptr {
			err = errors.Errorf("msg type error, should be T{} or &T{}")
			return
		}
	}
	t := msg.MsgType()
	msgsLock.Lock()
	defer msgsLock.Unlock()
	if msgs[t] != nil {
		err = errors.Errorf("The msgType[%d] is already occupied by %s", t, msgs[t])
		return
	}
	msgs[t] = typ
	return
}

func NewMsg(t MsgType) Msg {
	typ := msgs[t]
	if typ == nil {
		return nil
	}
	return reflect.New(typ).Interface().(Msg)
}

func ParseMsg(ctx context.Context, header Header, bsBody []byte) (req Msg, err error) {
	req = NewMsg(MsgType(header.Type))
	if req == nil {
		err = errors.Errorf("type[%d] error", header.Type)
		return
	}
	err = proto.Unmarshal(bsBody[:header.Len], req)
	if err != nil {
		err = errors.Errorf(err.Error())
		return
	}
	return
}

func UnmarshalMsg(ctx context.Context, rbuf []byte) (m Msg, err error) {
	bsHeader := rbuf[:HeaderSize]
	header, lHeader := ParseHeader(bsHeader)
	lenNeed := int(header.Len) + lHeader
	if len(rbuf) < lenNeed {
		err = errors.Errorf("len(rbuf):%d, lenNeed:%d", len(rbuf), lenNeed)
		return
	}
	bsBody := rbuf[lHeader:lenNeed]

	return ParseMsg(ctx, header, bsBody)
}

func MarshalMsg(ctx context.Context, req Msg, logid int64, in []byte) (wbuf []byte, err error) {
	if req == nil {
		err = errors.Errorf("req is nil")
		return
	}
	if len(in) < HeaderSize {
		in = make([]byte, 0x7fff)
	}
	wbuf = in[:HeaderSize]

	buf := proto.NewBuffer(wbuf[HeaderSize:])
	err = buf.Marshal(req)
	if err != nil {
		log.Ctx(ctx).Error().Err(err).Caller().Send()
		return
	}
	bsBody := buf.Bytes()
	need := len(bsBody) + HeaderSize
	if need > cap(wbuf) {
		wbuf = make([]byte, need)
		copy(wbuf[HeaderSize:], bsBody)
	}
	wbuf = wbuf[:need]

	h := Header{
		Type:  uint16(req.MsgType()),
		Len:   0,
		LogID: logid,
	}
	h.Len = uint32(len(bsBody))

	h.Format(wbuf)

	return
}

func (c *Conn) SendMsg(ctx context.Context, req Msg, logid int64, in []byte) (wbuf []byte, err error) {
	wbuf, err = MarshalMsg(ctx, req, logid, in)
	if err != nil {
		return
	}
	nb := net.Buffers{wbuf}
	_, err = nb.WriteTo(c)
	return
}

func (c *Conn) RecvMsg(ctx context.Context, in []byte) (msg Msg, err error) {
	header, buf, err := ReadPack(ctx, c, in[:0])
	if err != nil {
		return
	}
	msg, err = ParseMsg(ctx, header, buf[HeaderSize:])
	if err != nil {
		return
	}
	return
}

// ReadPack 读一个裸消息
func ReadPack(ctx context.Context, r io.Reader, buf []byte) (header Header, pack []byte, err error) {
	if cap(buf) < HeaderSize {
		buf = make([]byte, 0x7fff)
	}
	bsHeader := buf[:HeaderSize]
	n, err := io.ReadFull(r, bsHeader)
	if err != nil {
		_ = n
		err = errors.Errorf(err.Error())
		// log.Ctx(ctx).Error().Caller().Err(err).Send()
		return
	}
	if n != HeaderSize {
		err = errors.Errorf("n != HeaderSize,%d ,%d", n, HeaderSize)
		log.Ctx(ctx).Error().Caller().Int("n", n).Err(err).Send()
		return
	}
	header, lHeader := ParseHeader(bsHeader)
	lenNeed := int(header.Len) + lHeader
	if cap(buf) < lenNeed {
		bufnew := make([]byte, lenNeed)
		copy(bufnew, buf[:HeaderSize])
		buf = bufnew
	}
	pack = buf[:lenNeed]
	bsBody := pack[HeaderSize:]
	// n, err = io.ReadFull(conn.Conn, bsBody)
	n, err = io.ReadFull(r, bsBody)
	if err != nil {
		err = errors.Errorf(err.Error())
		log.Ctx(ctx).Error().Caller().Err(err).Send()
		return
	}
	if n != int(header.Len) {
		log.Ctx(ctx).Error().Caller().Int("n", n).Err(err).Send()
		return
	}

	return
}
