package main

import (
	"context"
	"fmt"

	"github.com/lxt1045/utils/cert/test/grpc/filesystem"
	"github.com/lxt1045/utils/cert/test/grpc/pb"
	"github.com/lxt1045/utils/config"
	werpc "github.com/lxt1045/utils/grpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	_ "go.uber.org/automaxprocs"
	"go.uber.org/automaxprocs/maxprocs"
)

/*
// gRPC四种通信方式
// 　　1. 简单Rpc（Simple RPC）：就是一般的rpc调用，一个请求对象对应一个返回对象。
// 　　2. 服务端流式rpc（Server-side streaming RPC）：一个请求对象，服务端可以传回多个结果对象。
// 　　3. 客户端流式rpc（Client-side streaming RPC）：客户端传入多个请求对象，服务端返回一个响应结果。
// 　　4. 双向流式rpc（Bidirectional streaming RPC）：结合客户端流式rpc和服务端流式rpc，可以传入多个对象，返回多个响应对象。
*/
func main() {
	//客户端连接服务端
	//从输入的证书文件中为客户端构造TLS凭证
	// creds, err := credentials.NewClientTLSFromFile("../pkg/tls/server.pem", "go-grpc-example")
	// if err != nil {
	// 	log.Fatalf("Failed to create TLS credentials %v", err)
	// }
	_ = maxprocs.Set
	certFile, keyFile, caFile := "static/ca/client-cert.pem", "static/ca/client-key.pem", "static/ca/root-cert.pem"
	tlsCert, err := config.LoadTLSConfig(filesystem.Static, certFile, keyFile, caFile)
	if err != nil {
		fmt.Println("Service error", err)
		return
	}
	creds := credentials.NewTLS(tlsCert)

	err = werpc.RegisterDNS(map[string][]string{
		"lxt1045.com": {"127.0.0.1:10088"},
	})
	if err != nil {
		fmt.Println("Service error", err)
		return
	}

	// 连接服务器
	conn, err := grpc.Dial("grpc:///lxt1045.com",
		grpc.WithTransportCredentials(creds),
		grpc.WithUnaryInterceptor(werpc.LogUnaryClientInterceptor()),
		grpc.WithStreamInterceptor(werpc.LogStreamClientInterceptor()),
	)
	// conn.ServerName = ""
	if err != nil {
		fmt.Println("network error", err)
	}

	//网络延迟关闭
	defer conn.Close()
	//获得grpc句柄
	c := pb.NewHelloClient(conn)

	ctx := context.TODO()

	//通过句柄进行调用服务端函数SayHello

	sayHello(ctx, c)
}

func sayHello(ctx context.Context, c pb.HelloClient) {
	ctx = werpc.GRPCContext(ctx)
	req := &pb.HelloReq{Name: "lixiantu"}
	re1, err := c.SayHello(ctx, req)
	if err != nil || re1 == nil {
		fmt.Println("calling SayHello() error", err)
		return
	}

	fmt.Println(re1.Msg)
}
