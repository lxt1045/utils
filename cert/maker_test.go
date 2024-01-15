package cert

import (
	"log"
	"testing"
)

func TestMake(t *testing.T) {
	dir := "./test/ca"
	MakeRoot(dir, "lxt")          // 创建根正式
	MainInner(dir, "lxt", "root") // 创建证书颁发机构证书
	MainLeaf(dir, "root", "server", []string{}, []string{"lxt1045.com", "lxt1045.cn"})
	MainLeaf(dir, "root", "client", []string{}, []string{})

	log.Println("success...")
}
