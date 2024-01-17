package call

import (
	"context"
	"net"
	"reflect"
	"unsafe"

	"github.com/lxt1045/errors"
	"github.com/lxt1045/utils/msg/codec"
)

type Client struct {
	*codec.Codec
	Methods map[string]MethodFull // CallID 需要连接握手后从service端获取
}

// NewClient fRegister: pb.RegisterHelloServer(s *grpc.Server, srv HelloServer)
func NewClient(ctx context.Context, fRegister interface{}, conn net.Conn) (c Client, err error) {
	c = Client{
		Methods: make(map[string]MethodFull),
	}

	methods, err := getMethods(ctx, fRegister, nil)
	if err != nil {
		return
	}
	for i, m := range methods {
		m.CallID = uint16(i)
		c.Methods[m.Name] = m
	}

	c.Codec, err = codec.NewCodec(ctx, conn)
	if err != nil {
		return
	}
	go c.Codec.ReadLoop(ctx, nil)
	return
}

func (c Client) Close(ctx context.Context) (err error) {
	defer c.Conn.Close()
	_, err = c.Codec.SendCloseMsg(ctx)
	return
}

func (c Client) Invoke(ctx context.Context, method string, req codec.Msg) (resp codec.Msg, err error) {
	m := c.Methods[method]

	if t := reflect.TypeOf(req); req != nil && t.Elem() != m.ReqType {
		err = errors.Errorf("req type error: should be %s, not %s", m.ReqType.String(), t.String())
		return
	}

	if m.RespType != nil {
		resp = reflect.New(m.RespType).Interface().(codec.Msg)
	}

	done, err := c.Codec.ClientCall(ctx, 0, m.CallID, req, resp)
	if err != nil {
		return
	}
	if done != nil {
		err = <-done
	}
	return
}

type mockClient struct {
	Methods map[string]MethodFull
}

// NewClient fun: pb.RegisterHelloServer(s *grpc.Server, srv HelloServer)
func NewMockClient(ctx context.Context, fun interface{}, s interface{}) (c mockClient, err error) {
	methods, err := getMethods(ctx, fun, s)
	if err != nil {
		return
	}
	c.Methods = make(map[string]MethodFull)
	for _, m := range methods {
		c.Methods[m.Name] = m
	}
	return
}

func (c mockClient) Invoke(ctx context.Context, method string, req interface{}) (resp interface{}, err error) {
	m := c.Methods[method]

	// p := reflect.ValueOf(req).UnsafePointer()
	p := (*[2]unsafe.Pointer)(unsafe.Pointer(&req))[1]
	pr, err := m.Func(m.SvcPointer, ctx, p)
	if err != nil {
		return
	}

	resp = reflect.NewAt(m.RespType, pr).Interface()
	return
}

func (c mockClient) Invoke2(ctx context.Context, method string, req interface{}) (resp interface{}, err error) {
	m := c.Methods[method]

	p := reflect.ValueOf(req).UnsafePointer()
	pr, err := m.Func(m.SvcPointer, ctx, p)
	if err != nil {
		return
	}

	resp = reflect.NewAt(m.RespType, pr).Interface()
	return
}
