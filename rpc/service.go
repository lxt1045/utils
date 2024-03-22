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

func (s Service) Callers() []codec.Caller {
	callers := make([]codec.Caller, len(s.svcMethods))
	for i, m := range s.svcMethods {
		callers[i] = m
	}
	return callers
}

// StartService fRegister: pb.RegisterHelloService, service: implementation
func StartService(ctx context.Context, rwc io.ReadWriteCloser, service interface{}, fRegisters ...interface{}) (s Service, err error) {
	rpc, err := StartPeer(ctx, rwc, service, fRegisters...)
	if err != nil {
		return
	}
	s = rpc.Service
	return
}

// StartService fRegister: pb.RegisterHelloService, service: implementation
func startService(ctx context.Context, rwc io.ReadWriteCloser, service interface{}, fRegisters ...interface{}) (s Service, err error) {
	s = Service{
		svcMethods:    make([]SvcMethod, 0, 32),
		svcInterfaces: make(map[string]uint32),
	}
	for _, fRegister := range fRegisters {
		if fRegister == nil {
			err = errors.Errorf("fRegister should not been nil")
			return
		}
		methods, err1 := getSvcMethods(fRegister, service)
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
		s.Codec, err = codec.NewCodec(ctx, rwc, s.Callers())
		if err != nil {
			return
		}

		go s.Codec.ReadLoop(ctx)
	}

	return
}

func (c Service) Close(ctx context.Context) (err error) {
	defer c.Codec.Close()
	err = c.Codec.SendCloseMsg(ctx)
	return
}

func (s Service) Clone(ctx context.Context, rwc io.ReadWriteCloser) (sNew Service, err error) {
	s.Codec, err = codec.NewCodec(ctx, rwc, s.Callers())
	if err != nil {
		return
	}
	go s.Codec.ReadLoop(ctx)
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
