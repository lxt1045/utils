package cert

import (
	"log"
	"os"
	"path/filepath"
	"testing"
)

func TestSignVerify(t *testing.T) {
	dir := "./test/ca"
	keyPem, err := os.ReadFile(filepath.Join(dir, "server-key.pem"))
	if err != nil {
		log.Fatalln(err)
	}
	certPem, err := os.ReadFile(filepath.Join(dir, "server-cert.pem"))
	if err != nil {
		log.Fatalln(err)
	}

	plainText := "hello"
	signature := X509SignECDSA([]byte(plainText), keyPem)

	t.Logf("signature:%s\n", signature)

	ok, err := X509VerifyECDSA([]byte(plainText), signature, certPem)
	if err != nil {
		log.Fatalln(err)
	}

	t.Logf("ok:%v\n", ok)
}
