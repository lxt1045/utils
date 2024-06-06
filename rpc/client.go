package rpc

import (
	"context"
	"io"
	"reflect"
	"strings"
	"unsafe"

	"github.com/lxt1045/errors"
	"github.com/lxt1045/utils/rpc/base"
	"github.com/lxt1045/utils/rpc/codec"
)

type Client struct {
	*codec.Codec
	cliMethods map[string]CliMethod // CallID 需要连接握手后从service端获取
	svcMethods map[string]CliMethod // CallID 需要连接握手后从service端获取
}

// StartClient fRegister: pb.RegisterHelloServer(s *grpc.Server, srv HelloServer)
func StartClient(ctx context.Context, rwc io.ReadWriteCloser, fRegisters ...interface{}) (c Client, err error) {
	rpc, err := StartPeer(ctx, rwc, nil, fRegisters...)
	if err != nil {
		return
	}
	c = rpc.Client
	return
}

// StartClient fRegister: pb.RegisterHelloServer(s *grpc.Server, srv HelloServer)
func startClient(ctx context.Context, rwc io.ReadWriteCloser, fRegisters ...interface{}) (c Client, err error) {
	c = Client{
		cliMethods: make(map[string]CliMethod),
		svcMethods: make(map[string]CliMethod),
	}
	for _, fRegister := range fRegisters {
		if fRegister == nil {
			err = errors.Errorf("fRegister should not been nil")
			return
		}
		methods, err1 := getCliMethods(fRegister)
		if err1 != nil {
			err = err1
			return
		}
		for i, m := range methods {
			m.CallID = uint16(i)
			c.cliMethods[m.Name] = m
		}
	}

	c.Codec, err = codec.NewCodec(ctx, rwc, nil)
	if err != nil {
		return
	}
	go c.Codec.ReadLoop(ctx)
	go c.Codec.Heartbeat(ctx)

	err = c.getMethodsFromSvc(ctx)
	if err != nil {
		return
	}
	return
}

func (c *Client) getMethodsFromSvc(ctx context.Context) (err error) {
	req := &base.CmdReq{
		Cmd: base.CmdReq_CallIDs,
	}
	res, err := c.SendCmd(ctx, 0, req)
	if err != nil {
		return
	}
	if len(res.Fields) == 0 {
		err = errors.Errorf("service func is empty")
		return
	}

	methodsNew := make(map[string]CliMethod)
	for i, name := range res.Fields {
		m, ok := c.cliMethods[name]
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
	c.svcMethods = methodsNew
	return
}

func (c Client) Close(ctx context.Context) (err error) {
	defer c.Codec.Close()
	err = c.Codec.SendCloseMsg(ctx)
	return
}

func (c Client) Invoke(ctx context.Context, method string, req codec.Msg, resp codec.Msg) (err error) {
	m, ok := c.svcMethods[method]
	if !ok {
		if false {
			cc := c
			err = cc.getMethodsFromSvc(ctx)
			if err == nil {
				for k, v := range cc.svcMethods {
					c.svcMethods[k] = v // map 是指针，所以 c 不是指针也可以新增
				}
				m, ok = c.svcMethods[method]
			}
		}
		if !ok {
			err = errors.Errorf("method not found: %s", method)
			return
		}
	}

	if t := reflect.TypeOf(req); req != nil && t.Elem() != m.reqType {
		err = errors.Errorf("req type error: should be %s, not %s", m.reqType.String(), t.String())
		return
	}
	if t := reflect.TypeOf(resp); resp != nil && t.Elem() != m.respType {
		err = errors.Errorf("req type error: should be %s, not %s", m.reqType.String(), t.String())
		return
	}
	// resp = m.NewResp()
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
	return c.doStream(ctx, method, true)
}

// StreamAsync 不等对方返回
func (c *Client) StreamAsync(ctx context.Context, method string) (stream *codec.Stream, err error) {
	return c.doStream(ctx, method, false)
}
func (c *Client) doStream(ctx context.Context, method string, sync bool) (stream *codec.Stream, err error) {
	m, ok := c.svcMethods[method]
	if !ok {
		err = errors.Errorf("method not found: %s", method)
		return
	}
	stream, err = c.Codec.Stream(ctx, 0, m.CallID, m, sync)
	return
}

type mockClient struct {
	Methods map[string]SvcMethod
}

// NewClient fun: pb.RegisterHelloServer(s *grpc.Server, srv HelloServer)
func NewMockClient(ctx context.Context, fun interface{}, s interface{}) (c mockClient, err error) {
	methods, err := getSvcMethods(fun, s)
	if err != nil {
		return
	}
	c.Methods = make(map[string]SvcMethod)
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
