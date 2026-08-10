package aes

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"

	"github.com/lxt1045/errors"
)

// AES-128。key长度：16, 24, 32 bytes 对应 AES-128, AES-192, AES-256
func extrPassword(pw []byte) []byte {
	l := len(pw)
	if l == 16 || l == 24 || l == 32 {
		return pw
	}

	if l < 16 {
		return PKCS5Padding(pw, 16)
	}
	if l < 24 {
		return PKCS5Padding(pw, 24)
	}
	if l < 32 {
		return PKCS5Padding(pw, 32)
	}

	return pw[:32]
}

// AES-128。key长度：16, 24, 32 bytes 对应 AES-128, AES-192, AES-256
// iv 长度：aes.BlockSize == 16
func AesEncrypt(origData, key, iv []byte) ([]byte, error) {
	if len(origData) == 0 {
		return nil, nil
	}
	key = extrPassword(key)
	block, err := aes.NewCipher(key)
	if err != nil {
		err = errors.Errorf(err.Error())
		return nil, err
	}

	blockSize := block.BlockSize() // blockSize := aes.BlockSize () // 16
	origData = PKCS5Padding(origData, blockSize)
	blockMode := cipher.NewCBCEncrypter(block, extrPassword(iv)[:blockSize])
	crypted := make([]byte, len(origData))
	blockMode.CryptBlocks(crypted, origData)
	return crypted, nil
}

func AesDecrypt(crypted, key, iv []byte) ([]byte, error) {
	if len(crypted) == 0 {
		return nil, nil
	}
	key = extrPassword(key)
	block, err := aes.NewCipher(key)
	if err != nil {
		err = errors.Errorf(err.Error())
		return nil, err
	}
	blockSize := block.BlockSize()
	blockMode := cipher.NewCBCDecrypter(block, extrPassword(iv)[:blockSize])
	origData := make([]byte, len(crypted))
	// origData := crypted
	blockMode.CryptBlocks(origData, crypted)
	origData = PKCS5UnPadding(origData)
	// origData = ZeroUnPadding(origData)

	return origData, nil
}

func PKCS5Padding(ciphertext []byte, blockSize int) []byte {
	padding := blockSize - len(ciphertext)%blockSize
	padtext := bytes.Repeat([]byte{byte(padding)}, padding)
	return append(ciphertext, padtext...)
}

func PKCS5UnPadding(origData []byte) []byte {
	length := len(origData)
	// 去掉最后一个字节 unpadding 次
	unpadding := int(origData[length-1])
	l := (length - unpadding)
	if l < 0 || l > len(origData) {
		return nil
	}
	return origData[:l]
}
