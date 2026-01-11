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

func TestMake_swinai(t *testing.T) {
	dir := "./test/ca_swinai"
	MakeRoot(dir, "swinai")                 // 创建根正式
	MainInner(dir, "swinai", "switch-root") // 创建证书颁发机构证书
	MainInner(dir, "swinai", "bridge-root") // 创建证书颁发机构证书
	MainLeaf(dir, "switch-root", "switch", []string{}, []string{"swinai.switch.com"})
	MainLeaf(dir, "bridge-root", "bridge", []string{}, []string{"swinai.bridge.com"})
	MainLeaf(dir, "bridge-root", "player", []string{}, []string{"swinai.player.com"})

	log.Println("success...")
}
