package cert

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"log"
	"math/big"
	"os"
	"strings"

	"github.com/lxt1045/errors"
)

func GenerateEcdsaKey() {
	//1.使用ecdsa生成秘钥对
	privateKey, err := ecdsa.GenerateKey(elliptic.P521(), rand.Reader)
	if err != nil {
		panic(err)
	}
	//2.将私钥写入磁盘
	//* 使用x509进行反序列化
	ecPrivateKey, err := x509.MarshalECPrivateKey(privateKey)
	if err != nil {
		panic(err)
	}
	//* 将得到的切片字符串放到pem.Block结构体中
	block := pem.Block{
		Type:    "ecdsa private key",
		Headers: nil,
		Bytes:   ecPrivateKey,
	}
	//* 使用pem编码
	file, err := os.Create("ecPrivate.pem")
	if err != nil {
		panic(err)
	}
	defer file.Close()
	err = pem.Encode(file, &block)
	if err != nil {
		panic(err)
	}
	//3.将公钥写入磁盘
	//* 从私钥中得到公钥
	publicKey := privateKey.PublicKey
	//* 使用x509进行序列化
	ecPublicKey, err := x509.MarshalPKIXPublicKey(&publicKey)
	if err != nil {
		panic(err)
	}
	//* 将得到的切片字符串放入pem.Block结构体中
	block = pem.Block{
		Type:    "ecdsa public key",
		Headers: nil,
		Bytes:   ecPublicKey,
	}
	//* 使用pem编码
	file, err = os.Create("ecPublic.pem")
	if err != nil {
		panic(err)
	}
	defer file.Close()
	pem.Encode(file, &block)
}

// 签名
func SignECDSA(plainText []byte, priFileName string) (rText, sText []byte) {
	//1.打开私钥文件，将内容读出来
	file, err := os.Open(priFileName)
	if err != nil {
		panic(err)
	}
	defer file.Close()
	fileInfo, err := file.Stat()
	if err != nil {
		panic(err)
	}
	buf := make([]byte, fileInfo.Size())
	_, err = file.Read(buf)
	if err != nil {
		panic(err)
	}
	//2.使用pem进行数据解码
	block, _ := pem.Decode(buf)
	//3.使用x509对数据还原
	privateKey, err := x509.ParseECPrivateKey(block.Bytes)
	if err != nil {
		panic(err)
	}
	//4.对原始数据进行哈希运算
	hashText := sha256.Sum256(plainText)
	//5.进行数字签名
	var r, s *big.Int //注意这里
	r, s, err = ecdsa.Sign(rand.Reader, privateKey, hashText[:])
	if err != nil {
		panic(err)
	}
	//6.返回值为指针，因此需要将该地址指向内存中的数据进行序列话
	rText, err = r.MarshalText()
	if err != nil {
		panic(err)
	}
	sText, err = s.MarshalText()
	if err != nil {
		panic(err)
	}
	return rText, sText
}

// 验签
func VerifyECDSA(plainText, rText, sText []byte, pubFileName string) bool {
	//1.打开公钥文件，读出数据
	file, err := os.Open(pubFileName)
	if err != nil {
		panic(err)
	}
	defer file.Close()
	fileInfo, err := file.Stat()
	if err != nil {
		panic(err)
	}
	buf := make([]byte, fileInfo.Size())
	_, err = file.Read(buf)
	if err != nil {
		panic(err)
	}
	//2.使用pem进行解码
	block, _ := pem.Decode(buf)
	//3.使用x509进行公钥数据还原
	pubInterface, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		panic(err)
	}
	//4.由于上一步返回的是一个接口类型，因此需要进行类型断言，将接口类型转换为公钥
	publicKey := pubInterface.(*ecdsa.PublicKey)
	//5.对原始数据进行哈希运算
	hashText := sha256.Sum256(plainText)
	//6.验签
	var r, s big.Int
	r.UnmarshalText(rText)
	s.UnmarshalText(sText)
	res := ecdsa.Verify(publicKey, hashText[:], &r, &s)
	return res
}

// 签名
func X509SignECDSA(plainText []byte, keyPem []byte) (signature string) {
	//1.打开私钥文件，将内容读出来

	keyDERBlock, _ := pem.Decode(keyPem)
	if keyDERBlock == nil || keyDERBlock.Type != "PRIVATE KEY" {
		log.Fatalln("ERROR: failed to read the CA key: unexpected content")
	}
	pKey, err := x509.ParsePKCS8PrivateKey(keyDERBlock.Bytes)
	if err != nil {
		err = errors.Errorf(err.Error())
		return
	}
	privateKey, ok := pKey.(*ecdsa.PrivateKey)
	if !ok {
		err = errors.Errorf("error type:%T", pKey)
		return
	}
	//

	// //2.使用pem进行数据解码
	// block, _ := pem.Decode(buf)
	// //3.使用x509对数据还原
	// privateKey, err := x509.ParseECPrivateKey(block.Bytes)
	// if err != nil {
	// 	panic(err)
	// }
	//4.对原始数据进行哈希运算
	hashText := sha256.Sum256(plainText)
	//5.进行数字签名
	var r, s *big.Int //注意这里
	r, s, err = ecdsa.Sign(rand.Reader, privateKey, hashText[:])
	if err != nil {
		err = errors.Errorf(err.Error())
		return
	}
	//6.返回值为指针，因此需要将该地址指向内存中的数据进行序列话
	// rText, err = r.MarshalText()
	rByte, err := r.GobEncode()
	if err != nil {
		err = errors.Errorf(err.Error())
		return
	}
	// sText, err = s.MarshalText()
	sByte, err := s.GobEncode()
	if err != nil {
		err = errors.Errorf(err.Error())
		return
	}

	// return rText, sText
	return base64.StdEncoding.EncodeToString(rByte) + ":" + base64.StdEncoding.EncodeToString(sByte)
}

// 验签
func X509VerifyECDSA(plainText []byte, signature string, certPem []byte) (ok bool, err error) {
	//1.打开公钥文件，读出数据
	certDERBlock, _ := pem.Decode(certPem)
	if certDERBlock == nil || certDERBlock.Type != "CERTIFICATE" {
		err = errors.Errorf("ERROR: failed to read the CA certificate: unexpected content")
		return
	}

	publicCert, err := x509.ParseCertificate(certDERBlock.Bytes)
	if err != nil {
		err = errors.Errorf(err.Error())
		return
	}
	publicKey := publicCert.PublicKey.(*ecdsa.PublicKey)

	// //2.使用pem进行解码
	// block, _ := pem.Decode(buf)
	// //3.使用x509进行公钥数据还原
	// pubInterface, err := x509.ParsePKIXPublicKey(block.Bytes)
	// if err != nil {
	// 	panic(err)
	// }
	// //4.由于上一步返回的是一个接口类型，因此需要进行类型断言，将接口类型转换为公钥
	// publicKey := pubInterface.(*ecdsa.PublicKey)
	//5.对原始数据进行哈希运算
	hashText := sha256.Sum256(plainText)
	//6.验签
	strs := strings.Split(signature, ":")
	if len(strs) != 2 {
		err = errors.Errorf("signature error")
		return
	}
	rText, sText := strs[0], strs[1]
	var r, s big.Int
	rByte, err := base64.StdEncoding.DecodeString(rText)
	if err != nil {
		err = errors.Errorf(err.Error())
		return
	}
	sByte, err := base64.StdEncoding.DecodeString(sText)
	if err != nil {
		err = errors.Errorf(err.Error())
		return
	}
	// r.UnmarshalText(rByte)
	// s.UnmarshalText(sByte)
	r.GobDecode(rByte)
	s.GobDecode(sByte)
	res := ecdsa.Verify(publicKey, hashText[:], &r, &s)
	return res, nil
}
