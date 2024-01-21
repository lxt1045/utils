package rpc

import (
	"context"
	"io"
	"strings"

	"github.com/lxt1045/errors"
	"github.com/lxt1045/utils/msg/codec"
	"github.com/lxt1045/utils/msg/rpc/base"
)

type RPC struct {
	Client
	Service
}

// NewCS fRegister: pb.RegisterHelloServer(rpc *grpc.Server, srv HelloServer)
func NewRPC(ctx context.Context, rwc io.ReadWriteCloser, service interface{}, cliRegisters, svcRegisters []interface{}) (rpc RPC, err error) {
	rpc = RPC{
		Client: Client{
			Methods: make(map[string]MethodFull),
		},
		Service: Service{
			svcMethods:    make([]Method, 0, 32),
			svcInterfaces: make(map[string]uint32),
		},
	}

	for _, fRegister := range cliRegisters {
		if fRegister == nil {
			err = errors.Errorf("fRegister should not been nil")
			return
		}
		methods, err1 := getMethods(ctx, fRegister, service)
		if err1 != nil {
			err = err1
			return
		}

		for i, m := range methods {
			m.CallID = uint16(i)
			rpc.Client.Methods[m.Name] = m
		}
		for _, m := range methods {
			rpc.svcInterfaces[m.Name] = uint32(len(rpc.svcMethods))
			rpc.svcMethods = append(rpc.svcMethods, m.Method)
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

	rpc.Client.Codec, err = codec.NewCodec(ctx, rwc)
	if err != nil {
		return
	}
	rpc.Service.Codec = rpc.Client.Codec
	rpc.Service.Codec.SetCallIDs(callIDs)

	go rpc.Service.Codec.ReadLoop(ctx, func(callID uint16) codec.Caller {
		m := rpc.svcMethods[callID]
		return m
	})

	req := &base.CmdReq{
		Cmd: base.CmdReq_CallIDs,
	}

	res, err := rpc.Client.SendCmd(ctx, 0, req)
	if err != nil {
		rpc.Client.Codec.Close()
		rpc.Service.Codec.Close()
		return
	}

	methodsNew := make(map[string]MethodFull)
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

func (rpc RPC) Close(ctx context.Context) (err error) {
	err = rpc.Client.Close(ctx)
	err1 := rpc.Service.Close(ctx)
	if err == nil {
		err = err1
	}
	return
}
