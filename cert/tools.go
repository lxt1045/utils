package cert

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha1"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/asn1"
	"encoding/pem"
	"log"
	"math/big"
	"os"
	"os/user"
	"path/filepath"
	"time"
)

var userAndHostname string

func init() {
	log.SetFlags(log.Flags() | log.Lmicroseconds | log.Lshortfile) //log.Llongfile

	u, err := user.Current()
	if err == nil {
		userAndHostname = u.Username + "@"
	}
	if h, err := os.Hostname(); err == nil {
		userAndHostname += h
	}
	if err == nil && u.Name != "" && u.Name != u.Username {
		userAndHostname += " (" + u.Name + ")"
	}
}

type Spki struct {
	Algorithm        pkix.AlgorithmIdentifier
	SubjectPublicKey asn1.BitString
}

func randomSerialNumber() (*big.Int, error) {
	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	return rand.Int(rand.Reader, serialNumberLimit)
}

// 生成椭圆密钥对
func keyToPem(priv crypto.Signer) (privPem, pubPem []byte, err error) {
	pub := priv.Public()
	spkiASN1, err := x509.MarshalPKIXPublicKey(pub)
	if err != nil {
		log.Println(err)
		return
	}
	var spki Spki
	_, err = asn1.Unmarshal(spkiASN1, &spki)
	if err != nil {
		log.Println(err)
		return
	}
	skid := sha1.Sum(spki.SubjectPublicKey.Bytes)

	randSN, err := randomSerialNumber()
	if err != nil {
		log.Println(err)
		return
	}
	tpl := &x509.Certificate{
		SerialNumber: randSN,
		Subject: pkix.Name{
			Organization:       []string{"lxt dev CA"},
			OrganizationalUnit: []string{userAndHostname},

			CommonName: "lxt-admin " + userAndHostname,
		},
		SubjectKeyId: skid[:],

		NotAfter:  time.Now().AddDate(10, 0, 0),
		NotBefore: time.Now(),

		KeyUsage: x509.KeyUsageCertSign,

		BasicConstraintsValid: true,
		IsCA:                  true,
		MaxPathLenZero:        true,
	}

	cert, err := x509.CreateCertificate(rand.Reader, tpl, tpl, pub, priv)
	if err != nil {
		log.Println(err)
		return
	}

	privDER, err := x509.MarshalPKCS8PrivateKey(priv)
	if err != nil {
		log.Println(err)
		return
	}
	privPem = pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: privDER})
	pubPem = pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: cert})

	return
}

func toFile(priv crypto.Signer, root, privFile, pubFile string) (err error) {
	privPem, pubPem, err := keyToPem(priv)
	if err != nil {
		log.Println(err)
		return
	}
	err = os.WriteFile(filepath.Join(root, privFile), privPem, 0644) //  0400)
	if err != nil {
		log.Println(err)
		return
	}

	err = os.WriteFile(filepath.Join(root, pubFile), pubPem, 0644)
	if err != nil {
		log.Println(err)
		return
	}
	return
}

// 生成椭圆密钥对
func ecdsaGenerateKey() (privPem, pubPem []byte, err error) {
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		log.Println(err)
		return
	}

	return keyToPem(priv)
}

func ecdsaToFile(root, privFile, pubFile string) (err error) {
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		log.Println(err)
		return
	}
	return toFile(priv, root, privFile, pubFile)
}

// 生成根 RSA 密钥对
func rootGenerateKey() (privPem, pubPem []byte, err error) {
	priv, err := rsa.GenerateKey(rand.Reader, 3072)
	if err != nil {
		log.Println(err)
		return
	}

	return keyToPem(priv)
}

func rootToFile(root, privFile, pubFile string) (err error) {
	priv, err := rsa.GenerateKey(rand.Reader, 3072)
	if err != nil {
		log.Println(err)
		return
	}
	return toFile(priv, root, privFile, pubFile)
}

// 生成 RSA 密钥对
func rsaGenerateKey() (privPem, pubPem []byte, err error) {
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		log.Println(err)
		return
	}

	return keyToPem(priv)
}

func rsaToFile(root, privFile, pubFile string) (err error) {
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		log.Println(err)
		return
	}
	return toFile(priv, root, privFile, pubFile)
}
