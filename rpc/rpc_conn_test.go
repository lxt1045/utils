package rpc

import (
	"context"
	"net"
	"testing"
	"time"

	base "github.com/lxt1045/utils/rpc/base"
)

func TestConn(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	addr := ":18080"
	ch := make(chan struct{})
	go func() {
		testService(ctx, cancel, t, addr, ch)
	}()

	<-ch
	time.Sleep(time.Millisecond * 10)
	testClient(ctx, cancel, t, addr)
}

func testService(ctx context.Context, cancel context.CancelFunc, t *testing.T, addr string, ch chan struct{}) {
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

		s, err := StartService(ctx, cancel, conn, &server{Str: "test"}, base.RegisterHelloServer)
		if err != nil {
			t.Fatal(err)
		}

		all := s.AllInterfaces()
		for i, m := range all {
			svc := (*server)(m.SvcPointer)
			t.Logf("idx:%d, service.Str:%v, func_key:%s, req:%s",
				i, svc.Str, m.Name, m.reqType.String())
		}
	}
}

func testClient(ctx context.Context, cancel context.CancelFunc, t *testing.T, addr string) {
	addr = "127.0.0.1" + addr
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if err != nil {
		t.Fatal(err)
	}

	client, err := StartClient(ctx, cancel, conn, base.NewHelloClient)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close(ctx)
	if err != nil {
		t.Fatal(err)
	}

	req := base.HelloReq{
		Name: "call 10086",
	}
	resp := base.HelloRsp{}

	err = client.Invoke(ctx, "SayHello", &req, &resp)
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("resp.Msg:\"%s\"", resp.Msg)
}
