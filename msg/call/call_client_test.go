package call

import (
	"context"
	"testing"

	"github.com/lxt1045/utils/msg/call/base"
)

func TestClient(t *testing.T) {
	ctx := context.Background()
	client, err := NewMockClient(ctx, base.RegisterHelloServer, &server{
		Str: "test",
	})
	if err != nil {
		panic(err)
	}

	req := base.HelloReq{
		Name: "call 10086",
	}

	ir, err := client.Invoke(ctx, "base.HelloServer.SayHello", &req)
	if err != nil {
		t.Fatal(err)
	}

	resp, ok := ir.(*base.HelloRsp)
	if !ok {
		t.Fatal("!ok")
	}
	t.Logf("resp.Msg:\"%s\"", resp.Msg)
}

func BenchmarkClient(b *testing.B) {
	ctx := context.Background()
	client, err := NewMockClient(ctx, base.RegisterHelloServer, &server{
		Str: "test",
	})
	if err != nil {
		panic(err)
	}

	req := base.BenchmarkReq{}

	b.Run("Client", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_, err := client.Invoke(ctx, "base.HelloServer.Benchmark", &req)
			if err != nil {
				b.Fatal(err)
			}
		}
	})
	b.Run("Client2", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_, err := client.Invoke2(ctx, "base.HelloServer.Benchmark", &req)
			if err != nil {
				b.Fatal(err)
			}
		}
	})

	m := client.Methods["base.HelloServer.Benchmark"]
	b.Run("map", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			m = client.Methods["base.HelloServer.Benchmark"]
		}
	})
	_ = m
}
