package call

import (
	"context"
	"net"
	"unsafe"

	"github.com/lxt1045/utils/msg/codec"
)

type Service struct {
	*codec.Codec

	svcMethods    []Method
	svcInterfaces map[string]uint32
}

// NewService fRegister: pb.RegisterHelloService, service: implementation
func NewService(ctx context.Context, fRegister interface{}, service interface{}, conn net.Conn) (s Service, err error) {
	s = Service{
		svcMethods:    make([]Method, 0, 32),
		svcInterfaces: make(map[string]uint32),
	}

	methods, err := getMethods(ctx, fRegister, service)
	if err != nil {
		return
	}

	callIDs := make([]string, 0, len(methods))
	for _, m := range methods {
		s.svcInterfaces[m.Name] = uint32(len(s.svcMethods))
		s.svcMethods = append(s.svcMethods, m.Method)
		callIDs = append(callIDs, m.Name)
	}

	s.Codec, err = codec.NewCodec(ctx, conn)
	if err != nil {
		return
	}
	s.Codec.SetCallIDs(callIDs)

	go s.Codec.ReadLoop(ctx, func(callID uint16) codec.Caller {
		m := s.svcMethods[callID]
		return m
	})
	return
}

func (c Service) Close(ctx context.Context) (err error) {
	defer c.Conn.Close()
	err = c.Codec.SendCloseMsg(ctx)
	return
}
func (s *Service) AddService(ctx context.Context, fRegister interface{}, service interface{}) (err error) {
	methods, err := getMethods(ctx, fRegister, service)
	if err != nil {
		return
	}

	callIDs := make([]string, 0, len(methods))
	for _, m := range methods {
		s.svcInterfaces[m.Name] = uint32(len(s.svcMethods))
		s.svcMethods = append(s.svcMethods, m.Method)
		callIDs = append(callIDs, m.Name)
	}

	s.Codec.SetCallIDs(callIDs)
	return
}

func (s Service) Clone(ctx context.Context, conn net.Conn) (sNew Service, err error) {
	s.Codec, err = codec.NewCodec(ctx, conn)
	if err != nil {
		return
	}
	go s.Codec.ReadLoop(ctx, func(callID uint16) codec.Caller {
		m := s.svcMethods[callID]
		return m
	})
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

func (s Service) AllInterfaces() (is []MethodFull) {
	names := make(map[int]string)
	for k, v := range s.svcInterfaces {
		names[int(v)] = k
	}
	for i, m := range s.svcMethods {
		is = append(is, MethodFull{
			Method: m,
			Name:   names[i],
		})
	}
	return
}
