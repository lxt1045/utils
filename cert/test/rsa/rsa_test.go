package rsa

import (
	"bytes"
	"encoding/base64"
	"testing"
)

func init() {
}

func TestEncrypter(t *testing.T) {
	privateKey, publicKey, err := Generate(1024)
	if err != nil {
		t.Error("密钥文件生成失败：", err)
	}

	origData := []byte("hello world")

	data, _ := RsaEncrypt(origData, publicKey)
	str := base64.StdEncoding.EncodeToString(data)
	//v4CRbySoD/T5IOM/3iOHqTNgixSdR4YM7FV0TBUUnKKOcwwMZkXYg9eqyjGncai9Gwdp0K69dlI76M54Yu7oPdRfglOAKqmP7eSUvVfPEw7srQfF4BROMeeUK5OK6asRn/sOCGDuFZ8yYw5DK7G7KnrqKmg+7/dgj/L2NjEhKmQ=
	//riKSi39QVSVIJf+MSildNgewcKM2fHcSkVG6paRVYo+rO6FYTv+2salAXi9XJxq6I9c7w3Hffg5TiDEHL7JnuyqjkCfDI+iQL08bANV9gqGDbHAfqx0fUUd7zdFAOao3MeV5uUvC/SuEoxhInVoLyCN+jdxR8GmnhoHWiweBFIM=
	//eLETogGj98nR9EOEhYQm1XvGbU5N4C2wICCR7vpUUuN3Uu3TGDvDUWY3Tb6oX0EalSQ3jZPL0F+j1rrTpnvx674GBgZroX9HTOM5wByVEEqrHfccpq8xu+CrbZpyaASUc1oH82lMxSN6/Sw/Y09PqiSyo6zlGgbDCMJL4peHEliS/YRceCnwDPDfmF6De2vT/TiOLtIWV0TFrVxdg7yMSndo7oVH7l6Zq2ujq2Cpt8tvXzCGFRoUK8usDHTsaIKQmPwADxfOqr6LcXRM2f/MmfEdAU4rFSKqELdEGXLe9wX9JbY/w0lwNeCCsCspjT9hsUeMF998d1p3g9Qgq/fYfQ==
	t.Log(str)
	d, err := base64.StdEncoding.DecodeString(str)
	if err != nil {
		t.Error(err)
	}
	getData, _ := RsaDecrypt(d, privateKey)

	if bytes.Compare(getData, origData) != 0 {
		t.Errorf("/norigData:%v, /ngetData: %v", string(origData), string(getData))
	}
	t.Logf("/norigData:%v, /ngetData: %v", string(origData), string(getData))
}

func TestGenerateFile(t *testing.T) {
	err := GenRsaKey(2048)
	if err != nil {
		t.Error("密钥文件生成失败：", err)
	}

}
