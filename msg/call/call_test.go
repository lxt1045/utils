package call

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/lxt1045/utils/cert/test/grpc/pb"
)

func TestConn(t *testing.T) {
	ctx := context.Background()
	addr := ":18080"
	ch := make(chan struct{})
	go func() {
		testService(ctx, t, addr, ch)
	}()

	<-ch
	time.Sleep(time.Millisecond * 10)
	testClient(ctx, t, addr)
}

func testService(ctx context.Context, t *testing.T, addr string, ch chan struct{}) {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		t.Fatal(err)
	}
	for {
		ch <- struct{}{}

		// 接收输入流
		conn, err := ln.Accept()
		if err != nil {
			t.Fatal(err)
		}

		s, err := NewService(ctx, pb.RegisterHelloServer, &server{Str: "test"}, conn)
		if err != nil {
			t.Fatal(err)
		}

		all := s.AllInterfaces()
		for i, m := range all {
			svc := (*server)(m.SvcPointer)
			t.Logf("idx:%d, service.Str:%v, func_key:%s, req:%s",
				i, svc.Str, m.Name, m.ReqType.String())
		}
	}
}

func testClient(ctx context.Context, t *testing.T, addr string) {
	addr = "127.0.0.1" + addr
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if err != nil {
		t.Fatal(err)
	}

	client, err := NewClient(ctx, pb.RegisterHelloServer, conn)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close(ctx)
	if err != nil {
		t.Fatal(err)
	}

	req := pb.HelloReq{
		Name: "call 10086",
	}

	ir, err := client.Invoke(ctx, "pb.HelloServer.SayHello", &req)
	if err != nil {
		t.Fatal(err)
	}

	resp, ok := ir.(*pb.HelloRsp)
	if !ok {
		t.Fatal("!ok")
	}
	t.Logf("resp.Msg:\"%s\"", resp.Msg)

}
