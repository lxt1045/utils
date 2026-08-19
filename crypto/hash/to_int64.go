package hash

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/binary"
	"encoding/hex"

	"github.com/lxt1045/errors"
)

// StringToInt64SHA256 将字符串使用 SHA256 哈希算法转换为 int64
// 该方法碰撞几率极低，适用于需要高安全性的场景
func StringToInt64SHA256(s string) int64 {
	h := sha256.New()
	h.Write([]byte(s))
	bs := h.Sum(nil) // bs 是一个 32 字节 (256 bit) 的切片

	// 取前 8 字节转换为 int64
	return int64(binary.BigEndian.Uint64(bs[:8]) & 0x7FFFFFFFFFFFFFFF)
}

func Hmac(pwd, UserKey string) (code string, err error) {
	mac := hmac.New(sha512.New, []byte(UserKey))
	_, err = mac.Write([]byte(pwd))
	if err != nil {
		err = errors.New(err.Error())
		return
	}

	strHMAC := mac.Sum(nil)
	code = hex.EncodeToString(strHMAC)
	return
}
