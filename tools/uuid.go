package tools

import (
	"time"
	"uuid"
)

func UUIDv7StrToTs(uuidStr string) (ts int64, err error) {
	id, err := uuid.Parse(uuidStr)
	if err != nil {
		return
	}
	ts = UUIDv7ToTs(id)
	return
}
func UUIDv7PToTs(p *uuid.UUID) int64 {
	if p == nil {
		return 0
	}
	return UUIDv7ToTs(*p)
}
func UUIDv7ToTs(id uuid.UUID) int64 {
	// ts := binary.BigEndian.Uint64(id[:8]) >> 16
	b := id[:6]
	ts := int64(b[5]) | int64(b[4])<<8 | int64(b[3])<<16 | int64(b[2])<<24 | int64(b[1])<<32 | int64(b[0])<<40
	return int64(ts)
}
func UUIDv7StrToTime(uuidStr string) (t time.Time, err error) {
	ts, err := UUIDv7StrToTs(uuidStr)
	if err != nil {
		return
	}
	return time.UnixMilli(ts), nil
}
func UUIDv7ToTime(id uuid.UUID) time.Time {
	return time.UnixMilli(UUIDv7ToTs(id))
}

// extractTimestampFromUUIDv7 从 UUID v7 字符串中提取时间戳
func extractTimeFromUUIDv7(id uuid.UUID) (time.Time, error) {
	bytes := id[:]
	// 3. 提取前 6 个字节 (48 位) 作为时间戳
	// 注意：这里的字节序是大端序 (network byte order)
	var ts uint64
	for i := 0; i < 6; i++ {
		ts = (ts << 8) | uint64(bytes[i])
	}

	// 4. 将毫秒时间戳转换为 time.Time
	return time.UnixMilli(int64(ts)), nil
}

// extractTimestampFromUUIDv7 从 UUID v7 字符串中提取时间戳
func extractTimeFromUUIDv7Str(uuidStr string) (t time.Time, err error) {
	id, err := uuid.Parse(uuidStr)
	if err != nil {
		return
	}
	return extractTimeFromUUIDv7(id)
}
