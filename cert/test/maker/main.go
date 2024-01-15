package main

import (
	"log"

	"github.com/lxt1045/utils/cert"
)

func init() {
	log.SetFlags(log.Flags() | log.Lmicroseconds | log.Lshortfile) //log.Llongfile
}

func main() {
	dir := "../ca"
	cert.MakeRoot(dir, "lxt")          // 创建根正式
	cert.MainInner(dir, "lxt", "root") // 创建证书颁发机构证书
	cert.MainLeaf(dir, "root", "server", []string{}, []string{"lxt1045.com", "lxt1045.cn"})
	cert.MainLeaf(dir, "root", "client", []string{}, []string{})

	log.Println("success...")
}
