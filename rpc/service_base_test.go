package rpc

import (
	"context"
	"net"
	"reflect"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/lxt1045/utils/log"
	"github.com/lxt1045/utils/rpc/base"
	"github.com/lxt1045/utils/rpc/codec"
)

type server struct {
	Str   string
	count int
}

func (s *server) SayHello(ctx context.Context, in *base.HelloReq) (out *base.HelloRsp, err error) {

	// stream 模式的返回值由己方随意控制
	if stream := codec.GetStream(ctx); stream != nil {
		go func() {
			for i := 0; i < 10; i++ {
				iface, err1 := stream.Recv(ctx)
				if err1 != nil {
					log.Ctx(ctx).Info().Caller().Err(err1).Msg("err")
					return
				}
				in = iface.(*base.HelloReq)
				log.Ctx(ctx).Info().Caller().Msg("service recv: " + in.Name)
			}
		}()
		for i := 0; i < 10; i++ {
			str := "Service Send: " + strconv.Itoa(i)
			log.Ctx(ctx).Info().Caller().Msg("service send: " + str)
			err1 := stream.Send(ctx, &base.HelloRsp{Msg: str})
			if err1 != nil {
				log.Ctx(ctx).Info().Caller().Err(err1).Msg("err")
				return
			}
			time.Sleep(time.Second)
		}
		return
	}

	log.Ctx(ctx).Info().Caller().Msg("service recv: " + in.Name)

	str := "Service Send: " + strconv.Itoa(s.count)
	log.Ctx(ctx).Info().Caller().Msg("service send: " + str)
	s.count++
	return &base.HelloRsp{Msg: str}, nil
}

func (s *server) Benchmark(ctx context.Context, in *base.BenchmarkReq) (out *base.BenchmarkRsp, err error) {
	return &base.BenchmarkRsp{}, nil
}

func (s *server) Cmd(ctx context.Context, in *base.CmdReq) (out *base.CmdRsp, err error) {
	return &base.CmdRsp{}, nil
}

func (s *server) TestHello(ctx context.Context, in *base.HelloReq) (out *base.HelloRsp, err error) {
	// stream 模式的返回值由己方随意控制
	if stream := codec.GetStream(ctx); stream != nil {
		go func() {
			for i := 0; i < 10; i++ {
				iface, err1 := stream.Recv(ctx)
				if err1 != nil {
					log.Ctx(ctx).Info().Caller().Err(err1).Msg("err")
					return
				}
				in = iface.(*base.HelloReq)
				log.Ctx(ctx).Info().Caller().Msg("service recv: " + in.Name)
			}
		}()
		for i := 0; i < 10; i++ {
			str := "Service Send: " + strconv.Itoa(i)
			log.Ctx(ctx).Info().Caller().Msg("service send: " + str)
			err1 := stream.Send(ctx, &base.HelloRsp{Msg: str})
			if err1 != nil {
				log.Ctx(ctx).Info().Caller().Err(err1).Msg("err")
				return
			}
			time.Sleep(time.Second)
		}
		return
	}

	log.Ctx(ctx).Info().Caller().Msg("service recv: " + in.Name)

	str := "Service Send: " + strconv.Itoa(s.count)
	log.Ctx(ctx).Info().Caller().Msg("service send: " + str)
	s.count++
	return &base.HelloRsp{Msg: str}, nil
}

type fakeConn struct {
	w      chan struct{}
	r      chan struct{}
	wCache *[]byte
	rCache *[]byte
	wl     *sync.Mutex
	rl     *sync.Mutex

	name string

	net.Conn
}

func NewFakeConnPipe() (svc, cli *fakeConn) {
	svc = &fakeConn{
		w:      make(chan struct{}),
		r:      make(chan struct{}),
		wCache: &[]byte{},
		rCache: &[]byte{},
		wl:     &sync.Mutex{},
		rl:     &sync.Mutex{},
		name:   "svc",
	}
	cli = &fakeConn{
		w:      svc.r,
		r:      svc.w,
		wCache: svc.rCache,
		rCache: svc.wCache,
		wl:     svc.rl,
		rl:     svc.wl,
		name:   "cli",
	}
	return
}

func (f *fakeConn) Read(data []byte) (n int, err error) {
	read := func(data []byte) (n int, err error) {
		f.rl.Lock()
		defer f.rl.Unlock()
		if l := len(*f.rCache); l > 0 {
			if l >= len(data) {
				n = len(data)
				copy(data, *f.rCache)
				*f.rCache = (*f.rCache)[n:]
				select {
				case f.r <- struct{}{}:
				default:
				}
				return
			}

			n = l
			copy(data, *f.rCache)
			*f.rCache = (*f.rCache)[:0]
			return
		}
		return
	}

	for {
		n, err = read(data)
		if err != nil || n > 0 {
			return
		}
		<-f.r
	}
}

func (f *fakeConn) Write(data []byte) (n int, err error) {
	f.wl.Lock()
	defer f.wl.Unlock()
	*f.wCache = append(*f.wCache, data...)
	select {
	case f.w <- struct{}{}:
	default:
	}
	return len(data), nil
}

func (f *fakeConn) Close() (err error) {
	return
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

func TestCmp(t *testing.T) {
	s1, s2 := &server{}, &server{}
	i1 := 1
	ts := []reflect.Type{
		reflect.TypeOf(s1),
		reflect.TypeOf(s2),
		reflect.TypeOf(&i1),
	}

	t.Logf("1:%v", ts[0] == ts[1])
	t.Logf("1:%v", ts[0] == ts[2])
}
