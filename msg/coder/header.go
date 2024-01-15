package coder

import (
	"encoding/binary"
	"unsafe"
)

const (
	TypeCallReq  = 1
	TypeCallResp = 2
)

type Header struct {
	Ver     uint16 // 版本号, 0:raw bytes, 1:CallReq, 2:CallResp; size: 2
	Channel uint16 // 通道，通道见彼此独立, size: 2
	BodyLen uint16 // 消息长度, size: 2

	// Ver != 0 才需要
	SegmentCount uint16 // 大包拆包总数量, size: 2
	SegmentIdx   uint16 // 大包拆包编号, size: 2
	CallID       uint16 // 调用ID, size: 4; 连接建立后，将 caller func name 与id映射一次
	CallSN       uint32 // 调用序列号, size: 4
}

func ParseHeader(bs []byte) (h Header, l int) {
	_ = bs[unsafe.Sizeof(Header{})-1]
	h.Ver = binary.LittleEndian.Uint16(bs[0:])
	h.Channel = binary.LittleEndian.Uint16(bs[2:])
	h.BodyLen = binary.LittleEndian.Uint16(bs[4:])
	if h.Ver == 0 {
		l = 6
		return
	}
	h.SegmentCount = binary.LittleEndian.Uint16(bs[6:])
	h.SegmentIdx = binary.LittleEndian.Uint16(bs[8:])
	h.CallID = binary.LittleEndian.Uint16(bs[10:])
	h.CallSN = binary.LittleEndian.Uint32(bs[12:])
	l = 16
	return
}

func (h Header) Format(bs []byte) (out []byte) {
	_ = bs[unsafe.Sizeof(Header{})-1]
	binary.LittleEndian.PutUint16(bs[0:], h.Ver)
	binary.LittleEndian.PutUint16(bs[2:], h.Channel)
	binary.LittleEndian.PutUint16(bs[4:], h.BodyLen)
	if h.Ver == 0 {
		return bs[:6]
	}
	binary.LittleEndian.PutUint16(bs[6:], h.SegmentCount)
	binary.LittleEndian.PutUint16(bs[8:], h.SegmentIdx)
	binary.LittleEndian.PutUint16(bs[10:], h.CallID)
	binary.LittleEndian.PutUint32(bs[12:], h.CallSN)
	return bs[:16]
}
