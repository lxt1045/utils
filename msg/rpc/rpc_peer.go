package rpc

import (
	"context"
	"io"
	"reflect"
	"strings"

	"github.com/lxt1045/errors"
	"github.com/lxt1045/utils/msg/codec"
	"github.com/lxt1045/utils/msg/rpc/base"
)

type Peer struct {
	Client
	Service
}

// NewCS fRegister: pb.RegisterHelloServer(rpc *grpc.Server, srv HelloServer)
func NewPeer(ctx context.Context, rwc io.ReadWriteCloser, service interface{}, fRegisters ...interface{}) (rpc Peer, err error) {
	rpc = Peer{
		Client: Client{
			Methods: make(map[string]MethodClient),
		},
		Service: Service{
			svcMethods:    make([]Method, 0, 32),
			svcInterfaces: make(map[string]uint32),
		},
	}

	var cliRegisters, svcRegisters []interface{}
	for _, f := range fRegisters {
		regMethodType := reflect.TypeOf(f)
		if regMethodType.Kind() != reflect.Func {
			err = errors.Errorf("arg fun must be func")
			return
		}
		if regMethodType.NumIn() == 1 && regMethodType.NumOut() == 1 {
			cliRegisters = append(cliRegisters, f)
			continue
		}
		if regMethodType.NumIn() == 2 && regMethodType.NumOut() == 0 {
			svcRegisters = append(svcRegisters, f)
			continue
		}
	}

	for _, fRegister := range cliRegisters {
		if fRegister == nil {
			err = errors.Errorf("fRegister should not been nil")
			return
		}
		methods, err1 := getClientMethods(ctx, fRegister)
		if err1 != nil {
			err = err1
			return
		}

		for i, m := range methods {
			m.CallID = uint16(i)
			rpc.Client.Methods[m.Name] = m
		}
	}

	callIDs := make([]string, 0, 16)
	for _, fRegister := range svcRegisters {
		if fRegister == nil {
			err = errors.Errorf("fRegister should not been nil")
			return
		}
		methods, err1 := getMethods(ctx, fRegister, service)
		if err1 != nil {
			err = err1
			return
		}

		for _, m := range methods {
			rpc.svcInterfaces[m.Name] = uint32(len(rpc.svcMethods))
			rpc.svcMethods = append(rpc.svcMethods, m.Method)
			callIDs = append(callIDs, m.Name)
		}
	}

	rpc.Service.Codec, err = codec.NewCodec(ctx, rwc)
	if err != nil {
		return
	}
	rpc.Client.Codec = rpc.Service.Codec // 这个在收到
	rpc.Service.Codec.SetCallIDs(callIDs)

	go rpc.Service.Codec.ReadLoop(ctx, func(callID uint16) codec.Caller {
		m := rpc.svcMethods[callID]
		return m
	})
	go rpc.Client.Codec.Heartbeat(ctx)

	req := &base.CmdReq{
		Cmd: base.CmdReq_CallIDs,
	}

	res, err := rpc.Client.SendCmd(ctx, 0, req)
	if err != nil {
		rpc.Client.Codec.Close()
		rpc.Service.Codec.Close()
		return
	}

	methodsNew := make(map[string]MethodClient)
	for i, name := range res.Fields {
		m, ok := rpc.Methods[name]
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
	rpc.Methods = methodsNew
	return
}

func (rpc Peer) Close(ctx context.Context) (err error) {
	err = rpc.Client.Close(ctx)
	err1 := rpc.Service.Close(ctx)
	if err == nil {
		err = err1
	}
	return
}
