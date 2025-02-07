package rsa

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"errors"

	log "github.com/sirupsen/logrus"
)

// 加密
func RsaEncrypt(origData, publicKey []byte) ([]byte, error) {
	//解密pem格式的公钥
	block, _ := pem.Decode(publicKey)
	if block == nil {
		return nil, errors.New("public key error")
	}
	// 解析公钥
	pubInterface, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, err
	}
	// 类型断言
	pub := pubInterface.(*rsa.PublicKey)
	//加密
	return rsa.EncryptPKCS1v15(rand.Reader, pub, origData)
}

// 解密
func RsaDecrypt(ciphertext, privateKey []byte) ([]byte, error) {
	//解密
	block, _ := pem.Decode(privateKey)
	if block == nil {
		return nil, errors.New("private key error!")
	}
	//解析PKCS1格式的私钥
	//priv, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	//priv, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	priv, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, err
	}
	// 解密
	return rsa.DecryptPKCS1v15(rand.Reader, priv.(*rsa.PrivateKey), ciphertext)
}

func GetPublicKey() (version int, publicKey []byte) {
	return
}
func New() {
	privateKey, publicKey, err := Generate(1024)
	if err != nil {
		log.Warn("密钥文件生成失败：", err)
	}

	data, _ := RsaEncrypt([]byte("hello world"), publicKey)
	str := base64.StdEncoding.EncodeToString(data)
	//v4CRbySoD/T5IOM/3iOHqTNgixSdR4YM7FV0TBUUnKKOcwwMZkXYg9eqyjGncai9Gwdp0K69dlI76M54Yu7oPdRfglOAKqmP7eSUvVfPEw7srQfF4BROMeeUK5OK6asRn/sOCGDuFZ8yYw5DK7G7KnrqKmg+7/dgj/L2NjEhKmQ=
	//riKSi39QVSVIJf+MSildNgewcKM2fHcSkVG6paRVYo+rO6FYTv+2salAXi9XJxq6I9c7w3Hffg5TiDEHL7JnuyqjkCfDI+iQL08bANV9gqGDbHAfqx0fUUd7zdFAOao3MeV5uUvC/SuEoxhInVoLyCN+jdxR8GmnhoHWiweBFIM=
	//eLETogGj98nR9EOEhYQm1XvGbU5N4C2wICCR7vpUUuN3Uu3TGDvDUWY3Tb6oX0EalSQ3jZPL0F+j1rrTpnvx674GBgZroX9HTOM5wByVEEqrHfccpq8xu+CrbZpyaASUc1oH82lMxSN6/Sw/Y09PqiSyo6zlGgbDCMJL4peHEliS/YRceCnwDPDfmF6De2vT/TiOLtIWV0TFrVxdg7yMSndo7oVH7l6Zq2ujq2Cpt8tvXzCGFRoUK8usDHTsaIKQmPwADxfOqr6LcXRM2f/MmfEdAU4rFSKqELdEGXLe9wX9JbY/w0lwNeCCsCspjT9hsUeMF998d1p3g9Qgq/fYfQ==
	log.Println(str)
	d, err := base64.StdEncoding.DecodeString(str)
	if err != nil {
		log.Panicln(err)
	}
	origData, _ := RsaDecrypt(d, privateKey)
	log.Println(string(origData))

	return
}
