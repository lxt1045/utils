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
	"github.com/lxt1045/utils/msg/codec"
	"github.com/lxt1045/utils/msg/conn"
	base "github.com/lxt1045/utils/msg/rpc/base"
)

var x reflect.Type

func Type() reflect.Type {
	return x
}

func TestPipe(t *testing.T) {
	if (Method{}).RespType() == nil {
		t.Log("nil")
	}

	ctx := context.Background()
	s, c := NewFakeConnPipe()
	svc, err := conn.NewZip(ctx, s)
	if err != nil {
		t.Fatal(err)
	}
	cli, err := conn.NewZip(ctx, c)
	if err != nil {
		t.Fatal(err)
	}

	_, err = NewService(ctx, svc, &server{Str: "test"}, base.RegisterHelloServer)
	if err != nil {
		t.Fatal(err)
	}

	time.Sleep(time.Millisecond * 10)
	client, err := NewClient(ctx, cli, base.RegisterHelloServer)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close(ctx)
	if err != nil {
		t.Fatal(err)
	}
	{
		req := base.HelloReq{Name: "call 1"}
		ir, err := client.Invoke(ctx, "SayHello", &req)
		if err != nil {
			t.Fatal(err)
		}
		resp, ok := ir.(*base.HelloRsp)
		if !ok {
			t.Fatal("!ok")
		}
		t.Logf("resp.Msg:\"%s\"", resp.Msg)
	}
	{
		req := base.HelloReq{Name: "call 2"}
		ir, err := client.Invoke(ctx, "SayHello", &req)
		if err != nil {
			t.Fatal(err)
		}
		resp, ok := ir.(*base.HelloRsp)
		if !ok {
			t.Fatal("!ok")
		}
		t.Logf("resp.Msg:\"%s\"", resp.Msg)
	}
}

func TestPipeStream(t *testing.T) {
	ctx := context.Background()
	s, c := NewFakeConnPipe()
	svc, err := conn.NewZip(ctx, s)
	if err != nil {
		t.Fatal(err)
	}
	cli, err := conn.NewZip(ctx, c)
	if err != nil {
		t.Fatal(err)
	}

	_, err = NewService(ctx, svc, &server{Str: "test"}, base.RegisterHelloServer)
	if err != nil {
		t.Fatal(err)
	}

	time.Sleep(time.Millisecond * 10)
	client, err := NewClient(ctx, cli, base.RegisterHelloServer)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close(ctx)
	if err != nil {
		t.Fatal(err)
	}

	// base.NewHelloClient(nil)

	stream, err := client.Stream(ctx, "SayHello")
	if err != nil {
		t.Fatal(err)
	}

	go func() {
		for i := 0; i < 10; i++ {
			r, err := stream.Recv(ctx)
			if err != nil {
				t.Fatal(err)
			}
			resp, ok := r.(*base.HelloRsp)
			if !ok {
				t.Fatal("!ok")
			}
			log.Ctx(ctx).Info().Caller().Msg("client recv: " + resp.Msg)
		}
	}()
	for i := 0; i < 10; i++ {
		req := base.HelloReq{
			Name: "client Send " + strconv.Itoa(i),
		}
		log.Ctx(ctx).Info().Caller().Msg("client send: " + req.Name)
		err = stream.Send(ctx, &req)
		if err != nil {
			t.Fatal(err)
		}
		time.Sleep(time.Second)
	}
}

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

		s, err := NewService(ctx, conn, &server{Str: "test"}, base.RegisterHelloServer)
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

	client, err := NewClient(ctx, conn, base.RegisterHelloServer)
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

	ir, err := client.Invoke(ctx, "SayHello", &req)
	if err != nil {
		t.Fatal(err)
	}

	resp, ok := ir.(*base.HelloRsp)
	if !ok {
		t.Fatal("!ok")
	}
	t.Logf("resp.Msg:\"%s\"", resp.Msg)

}

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
