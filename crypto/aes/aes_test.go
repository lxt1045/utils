package aes

import (
	"encoding/base64"
	"math/rand"
	"testing"
)

func TestAesEncrypt(t *testing.T) {
	src := []byte("qEol9i2PpxzptxOC")
	pw := []byte("rWQ%P5fOSuo3B:_0")
	iv := []byte("Kg3Jp$)z.X%UA*w!")
	dst, err := AesEncrypt(src, pw, iv)
	if err != nil {
		t.Fatal(err)
	}

	encode := base64.StdEncoding.EncodeToString(dst)

	t.Logf("encode:%s", encode)

	encode = "bDAv9E4M6RLFmf6BQVF8l8JSpCTLhhMCe6tukq7iSWM="
	decode, err := base64.StdEncoding.DecodeString(encode)
	if err != nil {
		t.Fatal(err)
	}
	src2, err := AesDecrypt(decode, pw, iv)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("src:%s", string(src2))
}

func Test_extrPassword(t *testing.T) {
	pw := []byte("12334567890123456789012345678901234567890")
	extrPassword(pw)
	f := func(x int) {
		str := append(pw[:0:0], pw[:x]...)
		pw2 := extrPassword(str)
		t.Logf("%d, len(pw):%d, src:%s, dst:%s", x, len(pw2), string(pw[:len(pw2)]), string(pw2))
	}
	f(1)
	f(11)
	f(16)
	f(17)
	f(31)
	f(32)
	f(33)
}

func TestRandPassword(t *testing.T) {
	pools := []string{
		`01234567890abcdefghijklmmopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ~!@$%^&*():,./<>?|\-=_+`,
	}

	n := 16

	pw := make([]byte, n)
	for i := range pw {
		idx := rand.Intn(len(pools))
		pool := pools[idx]
		x := rand.Intn(len(pool))
		pw[i] = pool[x]
	}

	t.Logf("pw: %s", string(pw))
}
