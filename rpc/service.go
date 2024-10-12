package rpc

import (
	"context"
	"io"
	"reflect"
	"unsafe"

	"github.com/lxt1045/errors"
	"github.com/lxt1045/utils/rpc/codec"
)

type status uint8

const (
	statusInit    status = 0
	statusRunning status = 0
)

type Service struct {
	*codec.Codec

	svcMethods    []SvcMethod
	svcInterfaces map[string]uint32
	_type         reflect.Type
}

func (s Service) Methods() []codec.Method {
	callers := make([]codec.Method, len(s.svcMethods))
	for i, m := range s.svcMethods {
		callers[i] = m
	}
	return callers
}
func (s Service) CloneMethods(svc interface{}) []codec.Method {
	svcPointer := reflect.ValueOf(svc).UnsafePointer()
	callers := make([]codec.Method, len(s.svcMethods))
	for i, m := range s.svcMethods {
		m.SvcPointer = svcPointer
		callers[i] = m
	}
	return callers
}

// StartService fRegister: pb.RegisterHelloService, svc: implementation
func StartService(ctx context.Context, cancel context.CancelFunc, rwc io.ReadWriteCloser, svc interface{}, fRegisters ...interface{}) (s Service, err error) {
	rpc, err := StartPeer(ctx, cancel, rwc, svc, fRegisters...)
	if err != nil {
		return
	}
	s = rpc.Service
	return
}

// StartService fRegister: pb.RegisterHelloService, svc: implementation
func startService(ctx context.Context, cancel context.CancelFunc, rwc io.ReadWriteCloser, svc interface{}, fRegisters ...interface{}) (s Service, err error) {
	s = Service{
		svcMethods:    make([]SvcMethod, 0, 32),
		svcInterfaces: make(map[string]uint32),
	}
	for _, fRegister := range fRegisters {
		if fRegister == nil {
			err = errors.Errorf("fRegister should not been nil")
			return
		}
		methods, err1 := getSvcMethods(fRegister, svc)
		if err1 != nil {
			err = err1
			return
		}

		for _, m := range methods {
			s.svcInterfaces[m.Name] = uint32(len(s.svcMethods))
			s.svcMethods = append(s.svcMethods, m)
		}
	}

	if rwc != nil {
		s.Codec, err = codec.NewCodec(ctx, cancel, rwc, s.Methods(), false)
		if err != nil {
			return
		}
	}

	return
}

func (c Service) Close(ctx context.Context) (err error) {
	defer c.Codec.Close()
	err = c.Codec.SendCloseMsg(ctx)
	return
}

func (s Service) Clone(ctx context.Context, cancel context.CancelFunc, rwc io.ReadWriteCloser, svc interface{}) (sNew Service, err error) {
	s.Codec, err = codec.NewCodec(ctx, cancel, rwc, s.CloneMethods(svc), false)
	if err != nil {
		return
	}
	return s, nil
}

func (s Service) MethodIdx(method string) (idx uint32, exist bool) {
	idx, exist = s.svcInterfaces[method]
	return
}

func (s Service) Call(ctx context.Context, methodIdx uint32, req unsafe.Pointer) (resp unsafe.Pointer, err error) {
	// if int(methodIdx) >= len(methods) {
	// 	err = errors.Errorf("methodIdx is out of range")
	// 	return
	// }

	m := s.svcMethods[methodIdx]
	resp, err = m.Func(m.SvcPointer, ctx, req)
	return
}

func (s Service) AllInterfaces() (is []SvcMethod) {
	return s.svcMethods
}
