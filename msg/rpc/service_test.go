package rpc

import (
	"context"
	"net"
	"reflect"
	"testing"
	"unsafe"

	"github.com/lxt1045/utils/cert/test/grpc/pb"
	"github.com/lxt1045/utils/log"
	"github.com/lxt1045/utils/msg/rpc/base"
)

func TestAddService(t *testing.T) {
	ctx := context.Background()
	s, err := StartService(ctx, nil, &server{Str: "test"}, base.RegisterHelloServer, base.RegisterTestServer)
	if err != nil {
		t.Fatal(err)
	}

	all := s.AllInterfaces()
	for i, s := range all {
		m := s.(SvcMethod)
		svc := (*server)(m.SvcPointer)
		t.Logf("idx:%d, service.Str:%v, func_key:%s, req:%s",
			i, svc.Str, m.Name, m.reqType.String())
	}

	err = s.AddService(ctx, pb.RegisterHelloServer, nil)
	if err != nil {
		t.Fatal(err)
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

func TestMake(t *testing.T) {
	ctx := context.Background()
	s, err := StartService(ctx, nil, &server{Str: "test"}, base.RegisterHelloServer)
	if err != nil {
		t.Fatal(err)
	}

	all := s.AllInterfaces()
	for i, s := range all {
		m := s.(SvcMethod)
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
	s, err := StartService(ctx, nil, &server{Str: "test"}, base.RegisterHelloServer)
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

func TestIps(t *testing.T) {
	ctx := context.Background()
	addrs, err := GetBroadcastAddress()
	if err != nil {
		t.Fatal(err)
	}

	log.Ctx(ctx).Info().Caller().Interface("addrs", addrs).Send()
}

type Addr struct {
	MAC string
	IPs []string
}

// 返回广播地址列表
func GetBroadcastAddress() (addrs []Addr, err error) {
	interfaces, err := net.Interfaces() // 获取所有网络接口
	if err != nil {
		return
	}

	for _, face := range interfaces {
		// 选择 已启用的、能广播的、非回环 的接口
		if (face.Flags & (net.FlagUp | net.FlagBroadcast | net.FlagLoopback)) == (net.FlagBroadcast | net.FlagUp) {
			ips, err := face.Addrs() // 获取该接口下IP地址
			if err != nil {
				return nil, err
			}
			addr := Addr{
				MAC: face.HardwareAddr.String(),
			}
			for _, ip := range ips {
				if ipnet, ok := ip.(*net.IPNet); ok { // 转换成 IPNet { IP Mask } 形式
					if ipnet.IP.To4() != nil { // 只取IPv4的
						var fields net.IP // 用于存放广播地址字段（共4个字段）
						for i := 0; i < 4; i++ {
							fields = append(fields, (ipnet.IP.To4())[i]|(^ipnet.Mask[i])) // 计算广播地址各个字段
						}
						addr.IPs = append(addr.IPs, fields.String()) // 转换为字符串形式
					}
				}
			}
			addrs = append(addrs, addr) // 转换为字符串形式
		}
	}

	return
}
