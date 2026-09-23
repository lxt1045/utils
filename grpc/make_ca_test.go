package grpc_test

import (
	"log"
	"testing"

	"github.com/lxt1045/utils/cert"
)

func TestCA(t *testing.T) {

	dir := "./filesystem/ca"
	// cert.MakeRoot(dir, "root") // 创建根正式
	cert.MainLeaf(dir, "root", "server", []string{}, []string{"test.rpc"})
	cert.MainLeaf(dir, "root", "client", []string{}, []string{})

	log.Println("success...")
}
