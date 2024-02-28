package cert

import (
	"log"
	"os"
	"path/filepath"
)

type CertMode uint8

const (
	CertModeRoot  CertMode = 1
	CertModeInner CertMode = 2
	CertModeLeaf  CertMode = 3
)

// 创建根证书（自签名）
func MakeRoot(dir, name string) {
	selfCert, err := NewSelfSigned()
	if err != nil {
		log.Fatalln(err)
	}
	privPEM, certPEM, err := selfCert.MakeEcdsa(CertModeRoot, nil, nil)
	if err != nil {
		log.Fatalln(err)
	}
	err = WriteFile(dir, name+"-key.pem", name+"-cert.pem", privPEM, certPEM)
	if err != nil {
		log.Fatalln(err)
	}
}

// 创建证书颁发机构证书（根证书签名）
func MainInner(dir, parent, name string) {
	keyPem, err := os.ReadFile(filepath.Join(dir, parent+"-key.pem"))
	if err != nil {
		log.Fatalln(err)
	}
	certPem, err := os.ReadFile(filepath.Join(dir, parent+"-cert.pem"))
	if err != nil {
		log.Fatalln(err)
	}
	selfCert, err := New(keyPem, certPem)
	if err != nil {
		log.Fatalln(err)
	}

	privPEM, certPEM, err := selfCert.MakeEcdsa(CertModeInner, nil, nil)
	if err != nil {
		log.Fatalln(err)
	}
	err = WriteFile(dir, name+"-key.pem", name+"-cert.pem", privPEM, certPEM)
	if err != nil {
		log.Fatalln(err)
	}
}

// 创建TLS使用的证书（证书颁发机构证书签名）
func MainLeaf(dir, parent, name string, ips, hosts []string) {
	keyPem, err := os.ReadFile(filepath.Join(dir, parent+"-key.pem"))
	if err != nil {
		log.Fatalln(err)
	}
	certPem, err := os.ReadFile(filepath.Join(dir, parent+"-cert.pem"))
	if err != nil {
		log.Fatalln(err)
	}
	selfCert, err := New(keyPem, certPem)
	if err != nil {
		log.Fatalln(err)
	}

	privPEM, certPEM, err := selfCert.MakeEcdsa(CertModeLeaf, ips, hosts)
	if err != nil {
		log.Fatalln(err)
	}
	err = WriteFile(dir, name+"-key.pem", name+"-cert.pem", privPEM, certPEM)
	if err != nil {
		log.Fatalln(err)
	}
}
