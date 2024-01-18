package call

import (
	"context"
	"reflect"
	"testing"
	"unsafe"

	"github.com/lxt1045/utils/msg/call/base"
)

func TestMake(t *testing.T) {
	ctx := context.Background()
	s, err := NewService(ctx, base.RegisterHelloServer, &server{Str: "test"}, nil)
	if err != nil {
		t.Fatal(err)
	}

	all := s.AllInterfaces()
	for i, m := range all {
		svc := (*server)(m.SvcPointer)
		t.Logf("idx:%d, service.Str:%v, func_key:%s, req:%s",
			i, svc.Str, m.Name, m.reqType.String())
	}

	idx, exist := s.MethodIdx("base.HelloServer.SayHello")
	if !exist {
		t.Fatal("exist")
	}

	req := base.HelloReq{
		Name: "call 1",
	}
	r, err := s.Call(ctx, idx, unsafe.Pointer(&req))
	if err != nil {
		t.Fatal(err)
	}
	resp := (*base.HelloRsp)(r)
	t.Logf("resp.Msg:\"%s\"", resp.Msg)
}

func BenchmarkMethod(b *testing.B) {
	ctx := context.Background()
	s, err := NewService(ctx, base.RegisterHelloServer, &server{Str: "test"}, nil)
	if err != nil {
		b.Fatal(err)
	}

	req := base.BenchmarkReq{}
	idx, exist := s.MethodIdx("base.HelloServer.Benchmark")
	if !exist {
		b.Fatal("exist")
	}

	b.Run("Call", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_, err := s.Call(ctx, idx, unsafe.Pointer(&req))
			if err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("CallByReflect", func(b *testing.B) {
		svc := &server{}
		svcImpMethod, ok := reflect.TypeOf(svc).MethodByName("Benchmark")
		if !ok {
			b.Fatal("!ok")
		}
		fCall := func(ctx context.Context, methodIdx uint32, req interface{}) (resp interface{}, err error) {
			rvs := svcImpMethod.Func.Call([]reflect.Value{reflect.ValueOf(svc), reflect.ValueOf(ctx), reflect.ValueOf(req)})
			resp = rvs[0].Interface()
			err, _ = rvs[1].Interface().(error)
			return
		}
		for i := 0; i < b.N; i++ {
			_, err := fCall(ctx, idx, &req)
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}
