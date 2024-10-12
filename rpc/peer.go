package rpc

import (
	"context"
	"io"
	"reflect"

	"github.com/lxt1045/errors"
	"github.com/lxt1045/utils/rpc/codec"
)

type Peer struct {
	Client
	Service
}

// StartPeer fRegister: pb.RegisterHelloServer(rpc *grpc.Server, srv HelloServer)
func StartPeer(ctx context.Context, cacnel context.CancelFunc, rwc io.ReadWriteCloser, svc interface{}, fRegisters ...interface{}) (rpc Peer, err error) {
	rpc = Peer{
		Client: Client{
			cliMethods: make(map[string]CliMethod),
			svcMethods: make(map[string]CliMethod),
		},
		Service: Service{
			svcMethods:    make([]SvcMethod, 0, 32),
			svcInterfaces: make(map[string]uint32),
			_type:         reflect.TypeOf(svc),
		},
	}

	for _, f := range fRegisters {
		if f == nil {
			continue
		}
		regMethodType := reflect.TypeOf(f)
		if regMethodType.Kind() != reflect.Func {
			err = errors.Errorf("arg fun must be func:%s", regMethodType.String())
			return
		}

		// client
		if regMethodType.NumIn() == 1 && regMethodType.NumOut() == 1 {
			methods, err1 := getCliMethods(f)
			if err1 != nil {
				err = err1
				return
			}

			for i, m := range methods {
				m.CallID = uint16(i)
				rpc.Client.cliMethods[m.Name] = m
			}
			continue
		}

		// svc
		if regMethodType.NumIn() == 2 && regMethodType.NumOut() == 0 {
			methods, err1 := getSvcMethods(f, svc)
			if err1 != nil {
				err = err1
				return
			}

			for _, m := range methods {
				rpc.svcInterfaces[m.Name] = uint32(len(rpc.Service.svcMethods))
				rpc.Service.svcMethods = append(rpc.Service.svcMethods, m)
			}
			continue
		}
	}

	if len(rpc.Service.svcMethods) == 0 && len(rpc.Client.cliMethods) == 0 {
		err = errors.Errorf("method is empty")
		return
	}

	if rwc != nil {
		pCodec, err1 := codec.NewCodec(ctx, rwc, rpc.Service.Methods())
		if err1 != nil {
			err = err1
			return
		}
		rpc.Client.Codec = pCodec
		rpc.Service.Codec = pCodec

		go pCodec.ReadLoop(ctx, cacnel)

		if len(rpc.Client.cliMethods) > 0 {
			go pCodec.Heartbeat(ctx, cacnel)
			err = rpc.Client.getMethodsFromSvc(ctx)
			if err != nil {
				return
			}
		}
	}
	return
}

func (rpc Peer) Clone(ctx context.Context, cacnel context.CancelFunc, rwc io.ReadWriteCloser, svc interface{}) (out Peer, err error) {
	if reflect.TypeOf(svc) != rpc.Service._type {
		err = errors.Errorf("svc type is not equal")
		return
	}
	pCodec, err := codec.NewCodec(ctx, rwc, rpc.Service.CloneMethods(svc))
	if err != nil {
		return
	}
	rpc.Client.Codec = pCodec
	rpc.Service.Codec = pCodec

	go pCodec.ReadLoop(ctx, cacnel)

	if len(rpc.Client.cliMethods) > 0 {
		go pCodec.Heartbeat(ctx, cacnel)
		err = rpc.Client.getMethodsFromSvc(ctx)
		if err != nil {
			return
		}
	}

	return rpc, nil
}

func (rpc Peer) Close(ctx context.Context) (err error) {
	err = rpc.Client.Close(ctx)
	err1 := rpc.Service.Close(ctx)
	if err == nil {
		err = err1
	}
	return
}
