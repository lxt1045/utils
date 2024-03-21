package rpc

import (
	"context"
	"io"
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

	svcMethods    []codec.Caller
	svcInterfaces map[string]uint32
	callIDs       []string
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
		svcMethods:    make([]codec.Caller, 0, 32),
		svcInterfaces: make(map[string]uint32),
		callIDs:       make([]string, 0, 16), // service 的 callID 列表
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
			s.callIDs = append(s.callIDs, m.Name)
		}
	}

	if rwc != nil {
		s.Codec, err = codec.NewCodec(ctx, rwc, s.callIDs)
		if err != nil {
			return
		}

		go s.Codec.ReadLoop(ctx, s.svcMethods)
	}

	return
}

func (c Service) Close(ctx context.Context) (err error) {
	defer c.Codec.Close()
	err = c.Codec.SendCloseMsg(ctx)
	return
}

func (s *Service) AddService(ctx context.Context, fRegister interface{}, service interface{}) (err error) {
	methods, err := getSvcMethods(fRegister, service)
	if err != nil {
		return
	}

	for _, m := range methods {
		if _, ok := s.svcInterfaces[m.Name]; ok {
			err = errors.Errorf("%s already exists", m.Name)
			return
		}
		s.svcInterfaces[m.Name] = uint32(len(s.svcMethods))
		s.svcMethods = append(s.svcMethods, m)
		s.callIDs = append(s.callIDs, m.Name)
	}

	return
}

func (s Service) Clone(ctx context.Context, rwc io.ReadWriteCloser) (sNew Service, err error) {
	s.Codec, err = codec.NewCodec(ctx, rwc, s.callIDs)
	if err != nil {
		return
	}
	go s.Codec.ReadLoop(ctx, s.svcMethods)
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

	m := s.svcMethods[methodIdx].(SvcMethod)
	resp, err = m.Func(m.SvcPointer, ctx, req)
	return
}

func (s Service) AllInterfaces() (is []codec.Caller) {
	return s.svcMethods
}
