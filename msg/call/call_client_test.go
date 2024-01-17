package call

import (
	"context"
	"testing"

	"github.com/lxt1045/utils/cert/test/grpc/pb"
)

func TestClient(t *testing.T) {
	ctx := context.Background()
	client, err := NewMockClient(ctx, pb.RegisterHelloServer, &server{
		Str: "test",
	})
	if err != nil {
		panic(err)
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

func BenchmarkClient(b *testing.B) {
	ctx := context.Background()
	client, err := NewMockClient(ctx, pb.RegisterHelloServer, &server{
		Str: "test",
	})
	if err != nil {
		panic(err)
	}

	req := pb.BenchmarkReq{}

	b.Run("Client", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_, err := client.Invoke(ctx, "pb.HelloServer.Benchmark", &req)
			if err != nil {
				b.Fatal(err)
			}
		}
	})
	b.Run("Client2", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_, err := client.Invoke2(ctx, "pb.HelloServer.Benchmark", &req)
			if err != nil {
				b.Fatal(err)
			}
		}
	})

	m := client.Methods["pb.HelloServer.Benchmark"]
	b.Run("map", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			m = client.Methods["pb.HelloServer.Benchmark"]
		}
	})
	_ = m
}
