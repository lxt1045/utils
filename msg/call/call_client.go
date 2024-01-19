package call

import (
	"context"
	"net"
	"reflect"
	"strings"
	"unsafe"

	"github.com/lxt1045/errors"
	"github.com/lxt1045/utils/msg/call/base"
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

	req := &base.CmdReq{
		Cmd: base.CmdReq_CallIDs,
	}
	res, err := c.SendCmd(ctx, 0, req)
	if err != nil {
		c.Codec.Close()
		return
	}

	methodsNew := make(map[string]MethodFull)
	for i, name := range res.Fields {
		m, ok := c.Methods[name]
		if !ok {
			continue
		}
		m.CallID = uint16(i)

		i := strings.LastIndex(name, ".")
		if i >= 0 {
			name = name[i+1:]
		}
		methodsNew[name] = m
	}
	c.Methods = methodsNew
	return
}

func (c Client) Close(ctx context.Context) (err error) {
	defer c.Codec.Close()
	err = c.Codec.SendCloseMsg(ctx)
	return
}

func (c Client) Invoke(ctx context.Context, method string, req codec.Msg) (resp codec.Msg, err error) {
	m, ok := c.Methods[method]
	if !ok {
		err = errors.Errorf("method not found: %s", method)
		return
	}

	if t := reflect.TypeOf(req); req != nil && t.Elem() != m.reqType {
		err = errors.Errorf("req type error: should be %s, not %s", m.reqType.String(), t.String())
		return
	}
	resp = m.NewResp()
	done, err := c.Codec.ClientCall(ctx, 0, m.CallID, req, resp)
	if err != nil {
		return
	}
	if done != nil {
		err = <-done
	}
	return
}

// Stream 流式调用
func (c *Client) Stream(ctx context.Context, method string) (stream *codec.Stream, err error) {
	m, ok := c.Methods[method]
	if !ok {
		err = errors.Errorf("method not found: %s", method)
		return
	}
	stream, err = c.Codec.Stream(ctx, 0, m.CallID, m)
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
		i := strings.LastIndex(m.Name, ".")
		if i >= 0 {
			m.Name = m.Name[i+1:]
		}
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

	resp = reflect.NewAt(m.respType, pr).Interface()
	return
}

func (c mockClient) Invoke2(ctx context.Context, method string, req interface{}) (resp interface{}, err error) {
	m := c.Methods[method]

	p := reflect.ValueOf(req).UnsafePointer()
	pr, err := m.Func(m.SvcPointer, ctx, p)
	if err != nil {
		return
	}

	resp = reflect.NewAt(m.respType, pr).Interface()
	return
}
