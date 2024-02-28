package cert

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"log"
	"net"
	"os"
	"path/filepath"
	"time"

	"github.com/lxt1045/errors"
)

var (
	// 证书过期时间
	expiration = time.Now().AddDate(10, 0, 0)

	defaultSubject = pkix.Name{
		Country:            []string{"CN"},
		Province:           []string{"Chengdu"},
		Locality:           []string{"Chengdu"},
		Organization:       []string{"LxtLtd"},
		OrganizationalUnit: []string{"LxtProxy"},
		CommonName:         "Lxt Root CA",
	}
)

type Cert struct {
	rootCert *x509.Certificate
	rootKey  crypto.PrivateKey
}

func NewSelfSigned() (cert *Cert, err error) {
	cert = &Cert{
		//
	}
	return
}

func New(keyPem, certPem []byte) (cert *Cert, err error) {
	certDERBlock, _ := pem.Decode(certPem)
	if certDERBlock == nil || certDERBlock.Type != "CERTIFICATE" {
		err = errors.Errorf("ERROR: failed to read the CA certificate: unexpected content")
		return
	}

	rootCert, err := x509.ParseCertificate(certDERBlock.Bytes)
	if err != nil {
		err = errors.Errorf(err.Error())
		return
	}

	keyDERBlock, _ := pem.Decode(keyPem)
	if keyDERBlock == nil || keyDERBlock.Type != "PRIVATE KEY" {
		err = errors.Errorf("ERROR: failed to read the CA key: unexpected content")
		return
	}
	rootKey, err := x509.ParsePKCS8PrivateKey(keyDERBlock.Bytes)
	if err != nil {
		err = errors.Errorf(err.Error())
		return
	}

	cert = &Cert{
		rootKey:  rootKey,
		rootCert: rootCert,
	}
	return
}

// MakeEcdsa certMode: root, inner, leaf
func (c *Cert) MakeEcdsa(certMode CertMode, ips, hosts []string) (privPEM, certPEM []byte, err error) {
	defer func() {
		if ec, ok := err.(*errors.Code); !ok && err != nil && ec.Code() == 0 {
			err = errors.Errorf(err.Error())
		}
	}()
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return
	}
	pub := priv.Public()

	caCert, caKey := c.rootCert, c.rootKey

	randSN, err := randomSerialNumber()
	if err != nil {
		return
	}
	unsignedCert := &x509.Certificate{
		Version:      3,
		SerialNumber: randSN,
		Subject: pkix.Name{
			Country:            []string{"CN"},
			Province:           []string{"Chengdu"},
			Locality:           []string{"Chengdu"},
			Organization:       []string{"LxtLtd"},
			OrganizationalUnit: []string{"LxtProxy"},
			// CommonName:         "Lxt Root CA",
		},
		NotBefore:             time.Now(),
		NotAfter:              expiration,
		BasicConstraintsValid: true,
	}

	switch certMode {
	case CertModeRoot:
		unsignedCert.Subject.CommonName = "Lxt Root CA"
		unsignedCert.IsCA = true
		unsignedCert.MaxPathLen = 1
		unsignedCert.MaxPathLenZero = false
		unsignedCert.KeyUsage = x509.KeyUsageCertSign | x509.KeyUsageCRLSign
	case CertModeInner:
		unsignedCert.Subject.CommonName = "Lxt Organization CA" // 中间证书
		unsignedCert.IsCA = true
		unsignedCert.MaxPathLen = 0
		unsignedCert.MaxPathLenZero = true
		unsignedCert.KeyUsage = x509.KeyUsageCertSign | x509.KeyUsageCRLSign
	case CertModeLeaf:
		unsignedCert.Subject.CommonName = "lxt1045.com"
		unsignedCert.IsCA = false
		unsignedCert.KeyUsage = x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature
		unsignedCert.ExtKeyUsage = []x509.ExtKeyUsage{
			x509.ExtKeyUsageServerAuth,
			x509.ExtKeyUsageClientAuth,
		}
		unsignedCert.DNSNames = hosts
		for _, h := range ips {
			if ip := net.ParseIP(h); ip != nil {
				unsignedCert.IPAddresses = append(unsignedCert.IPAddresses, ip)
			}
		}
	default:
		err = errors.Errorf("certMode[%d] error, only support: CertModeRoot, CertModeInner, CertModeLeaf", certMode)
		return
	}

	bSelfSigned := c.rootCert == nil || c.rootKey == nil
	if bSelfSigned {
		caCert, caKey = unsignedCert, priv
	}

	// 签名公钥? 	ecPublicKey, err := x509.MarshalPKIXPublicKey(&publicKey)
	cert, err := x509.CreateCertificate(rand.Reader, unsignedCert, caCert, pub, caKey)
	if err != nil {
		log.Println(err)
		return
	}
	certPEM = pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: cert})

	// privDER, err := x509.MarshalECPrivateKey(priv)
	privDER, err := x509.MarshalPKCS8PrivateKey(priv)
	if err != nil {
		log.Println(err)
		return
	}
	privPEM = pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: privDER}) // "EC PRIVATE KEY"

	return
}

func WriteFile(dir, key, cert string, keyPEMBlock, certPEMBlock []byte) (err error) {
	certFile := filepath.Join(dir, cert)
	keyFile := filepath.Join(dir, key)

	err = os.WriteFile(certFile, certPEMBlock, 0644)
	if err != nil {
		err = errors.Errorf(err.Error())
		return
	}
	err = os.WriteFile(keyFile, keyPEMBlock, 0600)
	if err != nil {
		err = errors.Errorf(err.Error())
		return
	}
	return
}
