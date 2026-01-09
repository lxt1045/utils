package rpc

import (
	"context"
	"io"
	"reflect"

	"github.com/lxt1045/errors"
	"github.com/lxt1045/utils/rpc/codec"
)

type LogidKey = codec.LogidKey

type Peer struct {
	Client
	Service
	cancel      context.CancelFunc
	chDone      <-chan struct{}
	cliPassKeys []string // 客户端: ctx 透传 key
}

func StartPeer(ctx context.Context, rwc io.ReadWriteCloser, svc interface{}, fRegisters ...interface{}) (rpc Peer, err error) {
	rpc, err = NewPeer(ctx, svc, fRegisters...)

	if rwc != nil {
		err = rpc.Conn(ctx, rwc)
	}
	return
}

// ctx 里 PassthroughKey{} 的 []string 类型; ctx透传value 的key列表, 传value总和长度不能超过65532
// fRegisters: 自己需要实现的server接口、需要请求对方的client接口注册函数.
func NewPeer(ctx context.Context, svc interface{}, fRegisters ...interface{}) (rpc Peer, err error) {
	ctx, cancel := context.WithCancel(ctx)
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
		cancel: cancel,
		chDone: ctx.Done(),
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

	return
}

func (rpc *Peer) Conn(ctx context.Context, rwc io.ReadWriteCloser) (err error) {
	callers := rpc.Service.Methods()
	return rpc.bindCodec(ctx, rpc.cancel, rwc, callers)
}

func (rpc Peer) Clone(ctx context.Context, rwc io.ReadWriteCloser, svc interface{}) (out Peer, err error) {
	if reflect.TypeOf(svc) != rpc.Service._type {
		err = errors.Errorf("svc type is not equal")
		return
	}
	ctx, cancel := context.WithCancel(ctx)
	rpc.cancel = cancel
	callers := rpc.Service.CloneMethods(svc)
	err = rpc.bindCodec(ctx, cancel, rwc, callers)
	if err != nil {
		return
	}

	return rpc, nil
}

func (rpc *Peer) bindCodec(ctx context.Context, cancel context.CancelFunc, rwc io.ReadWriteCloser, callers []codec.Method) (err error) {
	hasClient := len(rpc.Client.cliMethods) > 0
	pCodec, err := codec.NewCodec(ctx, cancel, rwc, callers, rpc.cliPassKeys, hasClient)
	if err != nil {
		return
	}
	rpc.Client.Codec = pCodec
	rpc.Service.Codec = pCodec

	if hasClient {
		err = rpc.Client.getMethodsFromSvc(ctx)
		if err != nil {
			return
		}
	}
	return
}
func (rpc *Peer) ServiceUse(middleware ...MiddlewareRespFunc) {
	rpc.Service.Use(middleware...)
}
func (rpc *Peer) ClientUse(middleware ...MiddlewareReqFunc) {
	rpc.Client.Use(middleware...)
}

// value 必须是 []string 类型的才会透传到 server 端
func (rpc *Peer) ClientPassKey(cliPassKeys ...string) {
	rpc.cliPassKeys = cliPassKeys
}

func (rpc Peer) Close(ctx context.Context) (err error) {
	if rpc.cancel != nil {
		rpc.cancel()
	}
	err = rpc.Client.Close(ctx)
	err1 := rpc.Service.Close(ctx)
	if err == nil {
		err = err1
	}
	return
}

func (rpc Peer) Done() <-chan struct{} {
	return rpc.chDone
}
