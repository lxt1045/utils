package call

import (
	"context"
	"reflect"
	"strconv"
	"testing"
	"unsafe"

	"github.com/lxt1045/utils/cert/test/grpc/pb"
	"github.com/lxt1045/utils/log"
)

func TestMake(t *testing.T) {
	ctx := context.Background()
	s, err := NewService(ctx, pb.RegisterHelloServer, &server{Str: "test"}, nil)
	if err != nil {
		t.Fatal(err)
	}

	all := s.AllInterfaces()
	for i, m := range all {
		svc := (*server)(m.SvcPointer)
		t.Logf("idx:%d, service.Str:%v, func_key:%s, req:%s",
			i, svc.Str, m.Name, m.ReqType.String())
	}

	idx, exist := s.MethodIdx("pb.HelloServer.SayHello")
	if !exist {
		t.Fatal("exist")
	}

	req := pb.HelloReq{
		Name: "call 1",
	}
	r, err := s.Call(ctx, idx, unsafe.Pointer(&req))
	if err != nil {
		t.Fatal(err)
	}
	resp := (*pb.HelloRsp)(r)
	t.Logf("resp.Msg:\"%s\"", resp.Msg)
}

func BenchmarkMethod(b *testing.B) {
	ctx := context.Background()
	s, err := NewService(ctx, pb.RegisterHelloServer, &server{Str: "test"}, nil)
	if err != nil {
		b.Fatal(err)
	}

	req := pb.BenchmarkReq{}
	idx, exist := s.MethodIdx("pb.HelloServer.Benchmark")
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

type server struct {
	Str   string
	count int
}

func (s *server) SayHello(ctx context.Context, in *pb.HelloReq) (out *pb.HelloRsp, err error) {
	s.count++
	str := s.Str + ": " + "hello " + in.Name + " " + strconv.Itoa(s.count)
	log.Ctx(ctx).Info().Caller().Msg(str)
	return &pb.HelloRsp{Msg: str}, nil
}

func (s *server) Benchmark(ctx context.Context, in *pb.BenchmarkReq) (out *pb.BenchmarkRsp, err error) {
	return &pb.BenchmarkRsp{}, nil
}
