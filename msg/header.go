package msg

import (
	"encoding/binary"
)

const (
	HeaderSize = 14

	TypeCallReq  = 1
	TypeCallResp = 2
)

type Header struct {
	Ver     uint8  // 版本号, size: 1
	Channel uint16 // 消息类型, size: 2
	Len     uint32 // 消息长度, size: 4
	LogID   int64  // 消息日志ID, size: 8

	// 以下只有在 Call 的时候才有
	SegmentCount uint16 // 大包拆包总数量, size: 2
	SegmentIdx   uint16 // 大包拆包编号, size: 2
	CallID       uint16 // 调用ID, size: 2
	CallSN       uint64 // 调用序列号, size: 8
}

func ParseHeader(bs []byte) (h Header, l int) {
	_ = bs[HeaderSize-1]
	h.Ver = bs[0]
	h.Type = binary.LittleEndian.Uint16(bs[1:])
	h.Len = binary.LittleEndian.Uint32(bs[3:])
	h.LogID = int64(binary.LittleEndian.Uint64(bs[7:]))
	l = HeaderSize
	return
}

func (h Header) Format(bs []byte) {
	_ = bs[HeaderSize-1]
	bs[0] = 2 // h.Ver
	binary.LittleEndian.PutUint16(bs[1:], h.Type)
	binary.LittleEndian.PutUint32(bs[3:], h.Len)
	binary.LittleEndian.PutUint64(bs[7:], uint64(h.LogID))
	return
}
