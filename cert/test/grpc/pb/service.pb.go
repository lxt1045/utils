// Code generated by protoc-gen-gogo. DO NOT EDIT.
// source: cert/test/grpc/pb/service.proto

package pb

import (
	context "context"
	fmt "fmt"
	proto "github.com/gogo/protobuf/proto"
	grpc "google.golang.org/grpc"
	codes "google.golang.org/grpc/codes"
	status "google.golang.org/grpc/status"
	io "io"
	math "math"
	math_bits "math/bits"
)

// Reference imports to suppress errors if they are not otherwise used.
var _ = proto.Marshal
var _ = fmt.Errorf
var _ = math.Inf

// This is a compile-time assertion to ensure that this generated file
// is compatible with the proto package it is being compiled against.
// A compilation error at this line likely means your copy of the
// proto package needs to be updated.
const _ = proto.GoGoProtoPackageIsVersion3 // please upgrade the proto package

// 客户端发送给服务端
type HelloReq struct {
	Name                 string   `protobuf:"bytes,1,opt,name=name,proto3" json:"name,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *HelloReq) Reset()         { *m = HelloReq{} }
func (m *HelloReq) String() string { return proto.CompactTextString(m) }
func (*HelloReq) ProtoMessage()    {}
func (*HelloReq) Descriptor() ([]byte, []int) {
	return fileDescriptor_92865409db567e80, []int{0}
}
func (m *HelloReq) XXX_Unmarshal(b []byte) error {
	return m.Unmarshal(b)
}
func (m *HelloReq) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	if deterministic {
		return xxx_messageInfo_HelloReq.Marshal(b, m, deterministic)
	} else {
		b = b[:cap(b)]
		n, err := m.MarshalToSizedBuffer(b)
		if err != nil {
			return nil, err
		}
		return b[:n], nil
	}
}
func (m *HelloReq) XXX_Merge(src proto.Message) {
	xxx_messageInfo_HelloReq.Merge(m, src)
}
func (m *HelloReq) XXX_Size() int {
	return m.Size()
}
func (m *HelloReq) XXX_DiscardUnknown() {
	xxx_messageInfo_HelloReq.DiscardUnknown(m)
}

var xxx_messageInfo_HelloReq proto.InternalMessageInfo

func (m *HelloReq) GetName() string {
	if m != nil {
		return m.Name
	}
	return ""
}

// 服务端返回给客户端
type HelloRsp struct {
	Msg                  string   `protobuf:"bytes,1,opt,name=msg,proto3" json:"msg,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *HelloRsp) Reset()         { *m = HelloRsp{} }
func (m *HelloRsp) String() string { return proto.CompactTextString(m) }
func (*HelloRsp) ProtoMessage()    {}
func (*HelloRsp) Descriptor() ([]byte, []int) {
	return fileDescriptor_92865409db567e80, []int{1}
}
func (m *HelloRsp) XXX_Unmarshal(b []byte) error {
	return m.Unmarshal(b)
}
func (m *HelloRsp) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	if deterministic {
		return xxx_messageInfo_HelloRsp.Marshal(b, m, deterministic)
	} else {
		b = b[:cap(b)]
		n, err := m.MarshalToSizedBuffer(b)
		if err != nil {
			return nil, err
		}
		return b[:n], nil
	}
}
func (m *HelloRsp) XXX_Merge(src proto.Message) {
	xxx_messageInfo_HelloRsp.Merge(m, src)
}
func (m *HelloRsp) XXX_Size() int {
	return m.Size()
}
func (m *HelloRsp) XXX_DiscardUnknown() {
	xxx_messageInfo_HelloRsp.DiscardUnknown(m)
}

var xxx_messageInfo_HelloRsp proto.InternalMessageInfo

func (m *HelloRsp) GetMsg() string {
	if m != nil {
		return m.Msg
	}
	return ""
}

type BenchmarkReq struct {
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *BenchmarkReq) Reset()         { *m = BenchmarkReq{} }
func (m *BenchmarkReq) String() string { return proto.CompactTextString(m) }
func (*BenchmarkReq) ProtoMessage()    {}
func (*BenchmarkReq) Descriptor() ([]byte, []int) {
	return fileDescriptor_92865409db567e80, []int{2}
}
func (m *BenchmarkReq) XXX_Unmarshal(b []byte) error {
	return m.Unmarshal(b)
}
func (m *BenchmarkReq) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	if deterministic {
		return xxx_messageInfo_BenchmarkReq.Marshal(b, m, deterministic)
	} else {
		b = b[:cap(b)]
		n, err := m.MarshalToSizedBuffer(b)
		if err != nil {
			return nil, err
		}
		return b[:n], nil
	}
}
func (m *BenchmarkReq) XXX_Merge(src proto.Message) {
	xxx_messageInfo_BenchmarkReq.Merge(m, src)
}
func (m *BenchmarkReq) XXX_Size() int {
	return m.Size()
}
func (m *BenchmarkReq) XXX_DiscardUnknown() {
	xxx_messageInfo_BenchmarkReq.DiscardUnknown(m)
}

var xxx_messageInfo_BenchmarkReq proto.InternalMessageInfo

type BenchmarkRsp struct {
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *BenchmarkRsp) Reset()         { *m = BenchmarkRsp{} }
func (m *BenchmarkRsp) String() string { return proto.CompactTextString(m) }
func (*BenchmarkRsp) ProtoMessage()    {}
func (*BenchmarkRsp) Descriptor() ([]byte, []int) {
	return fileDescriptor_92865409db567e80, []int{3}
}
func (m *BenchmarkRsp) XXX_Unmarshal(b []byte) error {
	return m.Unmarshal(b)
}
func (m *BenchmarkRsp) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	if deterministic {
		return xxx_messageInfo_BenchmarkRsp.Marshal(b, m, deterministic)
	} else {
		b = b[:cap(b)]
		n, err := m.MarshalToSizedBuffer(b)
		if err != nil {
			return nil, err
		}
		return b[:n], nil
	}
}
func (m *BenchmarkRsp) XXX_Merge(src proto.Message) {
	xxx_messageInfo_BenchmarkRsp.Merge(m, src)
}
func (m *BenchmarkRsp) XXX_Size() int {
	return m.Size()
}
func (m *BenchmarkRsp) XXX_DiscardUnknown() {
	xxx_messageInfo_BenchmarkRsp.DiscardUnknown(m)
}

var xxx_messageInfo_BenchmarkRsp proto.InternalMessageInfo

func init() {
	proto.RegisterType((*HelloReq)(nil), "pb.HelloReq")
	proto.RegisterType((*HelloRsp)(nil), "pb.HelloRsp")
	proto.RegisterType((*BenchmarkReq)(nil), "pb.BenchmarkReq")
	proto.RegisterType((*BenchmarkRsp)(nil), "pb.BenchmarkRsp")
}

func init() { proto.RegisterFile("cert/test/grpc/pb/service.proto", fileDescriptor_92865409db567e80) }

var fileDescriptor_92865409db567e80 = []byte{
	// 239 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0xff, 0x8c, 0x90, 0xb1, 0x4e, 0xc3, 0x40,
	0x10, 0x44, 0x59, 0x20, 0x28, 0x59, 0xa2, 0x10, 0x2d, 0x0d, 0x8a, 0x90, 0x41, 0xae, 0x5c, 0xf9,
	0x1c, 0x28, 0xe9, 0x52, 0x51, 0x3b, 0x1d, 0x9d, 0xef, 0xb4, 0x0a, 0x88, 0x5c, 0xbc, 0xbe, 0x3b,
	0x21, 0xf1, 0x87, 0x94, 0x7c, 0x02, 0xf2, 0x67, 0x50, 0xa1, 0x5c, 0x30, 0x0a, 0xa9, 0xdc, 0xcd,
	0xec, 0xe8, 0xcd, 0x48, 0x8b, 0x37, 0x86, 0x5d, 0x50, 0x81, 0x7d, 0x50, 0x2b, 0x27, 0x46, 0x89,
	0x56, 0x9e, 0xdd, 0xdb, 0x8b, 0xe1, 0x5c, 0x5c, 0x1d, 0x6a, 0x3a, 0x16, 0x9d, 0x26, 0x38, 0x7c,
	0xe4, 0xf5, 0xba, 0x2e, 0xb9, 0x21, 0xc2, 0xd3, 0x4d, 0x65, 0xf9, 0x0a, 0x6e, 0x21, 0x1b, 0x95,
	0x51, 0xa7, 0xd7, 0x5d, 0xee, 0x85, 0xa6, 0x78, 0x62, 0xfd, 0xea, 0x37, 0xde, 0xca, 0x74, 0x82,
	0xe3, 0x05, 0x6f, 0xcc, 0xb3, 0xad, 0xdc, 0x6b, 0xc9, 0xcd, 0x7f, 0xef, 0xe5, 0xee, 0x1b, 0x70,
	0x10, 0x71, 0xca, 0x70, 0xb8, 0xac, 0xde, 0x77, 0x7a, 0x9c, 0x8b, 0xce, 0xbb, 0xd5, 0xd9, 0x9e,
	0xf3, 0x92, 0x1e, 0xd1, 0x1c, 0x47, 0x7f, 0x1d, 0x34, 0xdd, 0x86, 0xfb, 0x13, 0xb3, 0x83, 0x4b,
	0x44, 0x14, 0x9e, 0x2f, 0x83, 0xe3, 0xca, 0xf6, 0xe8, 0xcf, 0xa0, 0x00, 0x2a, 0x70, 0xb2, 0x03,
	0x4a, 0x6e, 0x7a, 0x31, 0x34, 0xc7, 0x8b, 0x8e, 0xf0, 0xd2, 0x03, 0x29, 0x60, 0x71, 0xf9, 0xd1,
	0x26, 0xf0, 0xd9, 0x26, 0xf0, 0xd5, 0x26, 0xf0, 0x34, 0xc8, 0xd5, 0x83, 0x68, 0x7d, 0x16, 0x5f,
	0x7f, 0xff, 0x13, 0x00, 0x00, 0xff, 0xff, 0x45, 0x1b, 0x58, 0x01, 0x9d, 0x01, 0x00, 0x00,
}

// Reference imports to suppress errors if they are not otherwise used.
var _ context.Context
var _ grpc.ClientConn

// This is a compile-time assertion to ensure that this generated file
// is compatible with the grpc package it is being compiled against.
const _ = grpc.SupportPackageIsVersion4

// HelloClient is the client API for Hello service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://godoc.org/google.golang.org/grpc#ClientConn.NewStream.
type HelloClient interface {
	SayHello(ctx context.Context, in *HelloReq, opts ...grpc.CallOption) (*HelloRsp, error)
	Benchmark(ctx context.Context, in *BenchmarkReq, opts ...grpc.CallOption) (*BenchmarkRsp, error)
	StreamHello(ctx context.Context, opts ...grpc.CallOption) (Hello_StreamHelloClient, error)
	StreamReqHello(ctx context.Context, opts ...grpc.CallOption) (Hello_StreamReqHelloClient, error)
	StreamRespHello(ctx context.Context, in *HelloReq, opts ...grpc.CallOption) (Hello_StreamRespHelloClient, error)
}

type helloClient struct {
	cc *grpc.ClientConn
}

func NewHelloClient(cc *grpc.ClientConn) HelloClient {
	return &helloClient{cc}
}

func (c *helloClient) SayHello(ctx context.Context, in *HelloReq, opts ...grpc.CallOption) (*HelloRsp, error) {
	out := new(HelloRsp)
	err := c.cc.Invoke(ctx, "/pb.Hello/SayHello", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *helloClient) Benchmark(ctx context.Context, in *BenchmarkReq, opts ...grpc.CallOption) (*BenchmarkRsp, error) {
	out := new(BenchmarkRsp)
	err := c.cc.Invoke(ctx, "/pb.Hello/Benchmark", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *helloClient) StreamHello(ctx context.Context, opts ...grpc.CallOption) (Hello_StreamHelloClient, error) {
	stream, err := c.cc.NewStream(ctx, &_Hello_serviceDesc.Streams[0], "/pb.Hello/StreamHello", opts...)
	if err != nil {
		return nil, err
	}
	x := &helloStreamHelloClient{stream}
	return x, nil
}

type Hello_StreamHelloClient interface {
	Send(*HelloReq) error
	Recv() (*HelloRsp, error)
	grpc.ClientStream
}

type helloStreamHelloClient struct {
	grpc.ClientStream
}

func (x *helloStreamHelloClient) Send(m *HelloReq) error {
	return x.ClientStream.SendMsg(m)
}

func (x *helloStreamHelloClient) Recv() (*HelloRsp, error) {
	m := new(HelloRsp)
	if err := x.ClientStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

func (c *helloClient) StreamReqHello(ctx context.Context, opts ...grpc.CallOption) (Hello_StreamReqHelloClient, error) {
	stream, err := c.cc.NewStream(ctx, &_Hello_serviceDesc.Streams[1], "/pb.Hello/StreamReqHello", opts...)
	if err != nil {
		return nil, err
	}
	x := &helloStreamReqHelloClient{stream}
	return x, nil
}

type Hello_StreamReqHelloClient interface {
	Send(*HelloReq) error
	CloseAndRecv() (*HelloRsp, error)
	grpc.ClientStream
}

type helloStreamReqHelloClient struct {
	grpc.ClientStream
}

func (x *helloStreamReqHelloClient) Send(m *HelloReq) error {
	return x.ClientStream.SendMsg(m)
}

func (x *helloStreamReqHelloClient) CloseAndRecv() (*HelloRsp, error) {
	if err := x.ClientStream.CloseSend(); err != nil {
		return nil, err
	}
	m := new(HelloRsp)
	if err := x.ClientStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

func (c *helloClient) StreamRespHello(ctx context.Context, in *HelloReq, opts ...grpc.CallOption) (Hello_StreamRespHelloClient, error) {
	stream, err := c.cc.NewStream(ctx, &_Hello_serviceDesc.Streams[2], "/pb.Hello/StreamRespHello", opts...)
	if err != nil {
		return nil, err
	}
	x := &helloStreamRespHelloClient{stream}
	if err := x.ClientStream.SendMsg(in); err != nil {
		return nil, err
	}
	if err := x.ClientStream.CloseSend(); err != nil {
		return nil, err
	}
	return x, nil
}

type Hello_StreamRespHelloClient interface {
	Recv() (*HelloRsp, error)
	grpc.ClientStream
}

type helloStreamRespHelloClient struct {
	grpc.ClientStream
}

func (x *helloStreamRespHelloClient) Recv() (*HelloRsp, error) {
	m := new(HelloRsp)
	if err := x.ClientStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

// HelloServer is the server API for Hello service.
type HelloServer interface {
	SayHello(context.Context, *HelloReq) (*HelloRsp, error)
	Benchmark(context.Context, *BenchmarkReq) (*BenchmarkRsp, error)
	StreamHello(Hello_StreamHelloServer) error
	StreamReqHello(Hello_StreamReqHelloServer) error
	StreamRespHello(*HelloReq, Hello_StreamRespHelloServer) error
}

// UnimplementedHelloServer can be embedded to have forward compatible implementations.
type UnimplementedHelloServer struct {
}

func (*UnimplementedHelloServer) SayHello(ctx context.Context, req *HelloReq) (*HelloRsp, error) {
	return nil, status.Errorf(codes.Unimplemented, "method SayHello not implemented")
}
func (*UnimplementedHelloServer) Benchmark(ctx context.Context, req *BenchmarkReq) (*BenchmarkRsp, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Benchmark not implemented")
}
func (*UnimplementedHelloServer) StreamHello(srv Hello_StreamHelloServer) error {
	return status.Errorf(codes.Unimplemented, "method StreamHello not implemented")
}
func (*UnimplementedHelloServer) StreamReqHello(srv Hello_StreamReqHelloServer) error {
	return status.Errorf(codes.Unimplemented, "method StreamReqHello not implemented")
}
func (*UnimplementedHelloServer) StreamRespHello(req *HelloReq, srv Hello_StreamRespHelloServer) error {
	return status.Errorf(codes.Unimplemented, "method StreamRespHello not implemented")
}

func RegisterHelloServer(s *grpc.Server, srv HelloServer) {
	s.RegisterService(&_Hello_serviceDesc, srv)
}

func _Hello_SayHello_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(HelloReq)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(HelloServer).SayHello(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/pb.Hello/SayHello",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(HelloServer).SayHello(ctx, req.(*HelloReq))
	}
	return interceptor(ctx, in, info, handler)
}

func _Hello_Benchmark_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(BenchmarkReq)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(HelloServer).Benchmark(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/pb.Hello/Benchmark",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(HelloServer).Benchmark(ctx, req.(*BenchmarkReq))
	}
	return interceptor(ctx, in, info, handler)
}

func _Hello_StreamHello_Handler(srv interface{}, stream grpc.ServerStream) error {
	return srv.(HelloServer).StreamHello(&helloStreamHelloServer{stream})
}

type Hello_StreamHelloServer interface {
	Send(*HelloRsp) error
	Recv() (*HelloReq, error)
	grpc.ServerStream
}

type helloStreamHelloServer struct {
	grpc.ServerStream
}

func (x *helloStreamHelloServer) Send(m *HelloRsp) error {
	return x.ServerStream.SendMsg(m)
}

func (x *helloStreamHelloServer) Recv() (*HelloReq, error) {
	m := new(HelloReq)
	if err := x.ServerStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

func _Hello_StreamReqHello_Handler(srv interface{}, stream grpc.ServerStream) error {
	return srv.(HelloServer).StreamReqHello(&helloStreamReqHelloServer{stream})
}

type Hello_StreamReqHelloServer interface {
	SendAndClose(*HelloRsp) error
	Recv() (*HelloReq, error)
	grpc.ServerStream
}

type helloStreamReqHelloServer struct {
	grpc.ServerStream
}

func (x *helloStreamReqHelloServer) SendAndClose(m *HelloRsp) error {
	return x.ServerStream.SendMsg(m)
}

func (x *helloStreamReqHelloServer) Recv() (*HelloReq, error) {
	m := new(HelloReq)
	if err := x.ServerStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

func _Hello_StreamRespHello_Handler(srv interface{}, stream grpc.ServerStream) error {
	m := new(HelloReq)
	if err := stream.RecvMsg(m); err != nil {
		return err
	}
	return srv.(HelloServer).StreamRespHello(m, &helloStreamRespHelloServer{stream})
}

type Hello_StreamRespHelloServer interface {
	Send(*HelloRsp) error
	grpc.ServerStream
}

type helloStreamRespHelloServer struct {
	grpc.ServerStream
}

func (x *helloStreamRespHelloServer) Send(m *HelloRsp) error {
	return x.ServerStream.SendMsg(m)
}

var _Hello_serviceDesc = grpc.ServiceDesc{
	ServiceName: "pb.Hello",
	HandlerType: (*HelloServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "SayHello",
			Handler:    _Hello_SayHello_Handler,
		},
		{
			MethodName: "Benchmark",
			Handler:    _Hello_Benchmark_Handler,
		},
	},
	Streams: []grpc.StreamDesc{
		{
			StreamName:    "StreamHello",
			Handler:       _Hello_StreamHello_Handler,
			ServerStreams: true,
			ClientStreams: true,
		},
		{
			StreamName:    "StreamReqHello",
			Handler:       _Hello_StreamReqHello_Handler,
			ClientStreams: true,
		},
		{
			StreamName:    "StreamRespHello",
			Handler:       _Hello_StreamRespHello_Handler,
			ServerStreams: true,
		},
	},
	Metadata: "cert/test/grpc/pb/service.proto",
}

func (m *HelloReq) Marshal() (dAtA []byte, err error) {
	size := m.Size()
	dAtA = make([]byte, size)
	n, err := m.MarshalToSizedBuffer(dAtA[:size])
	if err != nil {
		return nil, err
	}
	return dAtA[:n], nil
}

func (m *HelloReq) MarshalTo(dAtA []byte) (int, error) {
	size := m.Size()
	return m.MarshalToSizedBuffer(dAtA[:size])
}

func (m *HelloReq) MarshalToSizedBuffer(dAtA []byte) (int, error) {
	i := len(dAtA)
	_ = i
	var l int
	_ = l
	if m.XXX_unrecognized != nil {
		i -= len(m.XXX_unrecognized)
		copy(dAtA[i:], m.XXX_unrecognized)
	}
	if len(m.Name) > 0 {
		i -= len(m.Name)
		copy(dAtA[i:], m.Name)
		i = encodeVarintService(dAtA, i, uint64(len(m.Name)))
		i--
		dAtA[i] = 0xa
	}
	return len(dAtA) - i, nil
}

func (m *HelloRsp) Marshal() (dAtA []byte, err error) {
	size := m.Size()
	dAtA = make([]byte, size)
	n, err := m.MarshalToSizedBuffer(dAtA[:size])
	if err != nil {
		return nil, err
	}
	return dAtA[:n], nil
}

func (m *HelloRsp) MarshalTo(dAtA []byte) (int, error) {
	size := m.Size()
	return m.MarshalToSizedBuffer(dAtA[:size])
}

func (m *HelloRsp) MarshalToSizedBuffer(dAtA []byte) (int, error) {
	i := len(dAtA)
	_ = i
	var l int
	_ = l
	if m.XXX_unrecognized != nil {
		i -= len(m.XXX_unrecognized)
		copy(dAtA[i:], m.XXX_unrecognized)
	}
	if len(m.Msg) > 0 {
		i -= len(m.Msg)
		copy(dAtA[i:], m.Msg)
		i = encodeVarintService(dAtA, i, uint64(len(m.Msg)))
		i--
		dAtA[i] = 0xa
	}
	return len(dAtA) - i, nil
}

func (m *BenchmarkReq) Marshal() (dAtA []byte, err error) {
	size := m.Size()
	dAtA = make([]byte, size)
	n, err := m.MarshalToSizedBuffer(dAtA[:size])
	if err != nil {
		return nil, err
	}
	return dAtA[:n], nil
}

func (m *BenchmarkReq) MarshalTo(dAtA []byte) (int, error) {
	size := m.Size()
	return m.MarshalToSizedBuffer(dAtA[:size])
}

func (m *BenchmarkReq) MarshalToSizedBuffer(dAtA []byte) (int, error) {
	i := len(dAtA)
	_ = i
	var l int
	_ = l
	if m.XXX_unrecognized != nil {
		i -= len(m.XXX_unrecognized)
		copy(dAtA[i:], m.XXX_unrecognized)
	}
	return len(dAtA) - i, nil
}

func (m *BenchmarkRsp) Marshal() (dAtA []byte, err error) {
	size := m.Size()
	dAtA = make([]byte, size)
	n, err := m.MarshalToSizedBuffer(dAtA[:size])
	if err != nil {
		return nil, err
	}
	return dAtA[:n], nil
}

func (m *BenchmarkRsp) MarshalTo(dAtA []byte) (int, error) {
	size := m.Size()
	return m.MarshalToSizedBuffer(dAtA[:size])
}

func (m *BenchmarkRsp) MarshalToSizedBuffer(dAtA []byte) (int, error) {
	i := len(dAtA)
	_ = i
	var l int
	_ = l
	if m.XXX_unrecognized != nil {
		i -= len(m.XXX_unrecognized)
		copy(dAtA[i:], m.XXX_unrecognized)
	}
	return len(dAtA) - i, nil
}

func encodeVarintService(dAtA []byte, offset int, v uint64) int {
	offset -= sovService(v)
	base := offset
	for v >= 1<<7 {
		dAtA[offset] = uint8(v&0x7f | 0x80)
		v >>= 7
		offset++
	}
	dAtA[offset] = uint8(v)
	return base
}
func (m *HelloReq) Size() (n int) {
	if m == nil {
		return 0
	}
	var l int
	_ = l
	l = len(m.Name)
	if l > 0 {
		n += 1 + l + sovService(uint64(l))
	}
	if m.XXX_unrecognized != nil {
		n += len(m.XXX_unrecognized)
	}
	return n
}

func (m *HelloRsp) Size() (n int) {
	if m == nil {
		return 0
	}
	var l int
	_ = l
	l = len(m.Msg)
	if l > 0 {
		n += 1 + l + sovService(uint64(l))
	}
	if m.XXX_unrecognized != nil {
		n += len(m.XXX_unrecognized)
	}
	return n
}

func (m *BenchmarkReq) Size() (n int) {
	if m == nil {
		return 0
	}
	var l int
	_ = l
	if m.XXX_unrecognized != nil {
		n += len(m.XXX_unrecognized)
	}
	return n
}

func (m *BenchmarkRsp) Size() (n int) {
	if m == nil {
		return 0
	}
	var l int
	_ = l
	if m.XXX_unrecognized != nil {
		n += len(m.XXX_unrecognized)
	}
	return n
}

func sovService(x uint64) (n int) {
	return (math_bits.Len64(x|1) + 6) / 7
}
func sozService(x uint64) (n int) {
	return sovService(uint64((x << 1) ^ uint64((int64(x) >> 63))))
}
func (m *HelloReq) Unmarshal(dAtA []byte) error {
	l := len(dAtA)
	iNdEx := 0
	for iNdEx < l {
		preIndex := iNdEx
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if shift >= 64 {
				return ErrIntOverflowService
			}
			if iNdEx >= l {
				return io.ErrUnexpectedEOF
			}
			b := dAtA[iNdEx]
			iNdEx++
			wire |= uint64(b&0x7F) << shift
			if b < 0x80 {
				break
			}
		}
		fieldNum := int32(wire >> 3)
		wireType := int(wire & 0x7)
		if wireType == 4 {
			return fmt.Errorf("proto: HelloReq: wiretype end group for non-group")
		}
		if fieldNum <= 0 {
			return fmt.Errorf("proto: HelloReq: illegal tag %d (wire type %d)", fieldNum, wire)
		}
		switch fieldNum {
		case 1:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field Name", wireType)
			}
			var stringLen uint64
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowService
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				stringLen |= uint64(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			intStringLen := int(stringLen)
			if intStringLen < 0 {
				return ErrInvalidLengthService
			}
			postIndex := iNdEx + intStringLen
			if postIndex < 0 {
				return ErrInvalidLengthService
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			m.Name = string(dAtA[iNdEx:postIndex])
			iNdEx = postIndex
		default:
			iNdEx = preIndex
			skippy, err := skipService(dAtA[iNdEx:])
			if err != nil {
				return err
			}
			if (skippy < 0) || (iNdEx+skippy) < 0 {
				return ErrInvalidLengthService
			}
			if (iNdEx + skippy) > l {
				return io.ErrUnexpectedEOF
			}
			m.XXX_unrecognized = append(m.XXX_unrecognized, dAtA[iNdEx:iNdEx+skippy]...)
			iNdEx += skippy
		}
	}

	if iNdEx > l {
		return io.ErrUnexpectedEOF
	}
	return nil
}
func (m *HelloRsp) Unmarshal(dAtA []byte) error {
	l := len(dAtA)
	iNdEx := 0
	for iNdEx < l {
		preIndex := iNdEx
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if shift >= 64 {
				return ErrIntOverflowService
			}
			if iNdEx >= l {
				return io.ErrUnexpectedEOF
			}
			b := dAtA[iNdEx]
			iNdEx++
			wire |= uint64(b&0x7F) << shift
			if b < 0x80 {
				break
			}
		}
		fieldNum := int32(wire >> 3)
		wireType := int(wire & 0x7)
		if wireType == 4 {
			return fmt.Errorf("proto: HelloRsp: wiretype end group for non-group")
		}
		if fieldNum <= 0 {
			return fmt.Errorf("proto: HelloRsp: illegal tag %d (wire type %d)", fieldNum, wire)
		}
		switch fieldNum {
		case 1:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field Msg", wireType)
			}
			var stringLen uint64
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowService
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				stringLen |= uint64(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			intStringLen := int(stringLen)
			if intStringLen < 0 {
				return ErrInvalidLengthService
			}
			postIndex := iNdEx + intStringLen
			if postIndex < 0 {
				return ErrInvalidLengthService
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			m.Msg = string(dAtA[iNdEx:postIndex])
			iNdEx = postIndex
		default:
			iNdEx = preIndex
			skippy, err := skipService(dAtA[iNdEx:])
			if err != nil {
				return err
			}
			if (skippy < 0) || (iNdEx+skippy) < 0 {
				return ErrInvalidLengthService
			}
			if (iNdEx + skippy) > l {
				return io.ErrUnexpectedEOF
			}
			m.XXX_unrecognized = append(m.XXX_unrecognized, dAtA[iNdEx:iNdEx+skippy]...)
			iNdEx += skippy
		}
	}

	if iNdEx > l {
		return io.ErrUnexpectedEOF
	}
	return nil
}
func (m *BenchmarkReq) Unmarshal(dAtA []byte) error {
	l := len(dAtA)
	iNdEx := 0
	for iNdEx < l {
		preIndex := iNdEx
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if shift >= 64 {
				return ErrIntOverflowService
			}
			if iNdEx >= l {
				return io.ErrUnexpectedEOF
			}
			b := dAtA[iNdEx]
			iNdEx++
			wire |= uint64(b&0x7F) << shift
			if b < 0x80 {
				break
			}
		}
		fieldNum := int32(wire >> 3)
		wireType := int(wire & 0x7)
		if wireType == 4 {
			return fmt.Errorf("proto: BenchmarkReq: wiretype end group for non-group")
		}
		if fieldNum <= 0 {
			return fmt.Errorf("proto: BenchmarkReq: illegal tag %d (wire type %d)", fieldNum, wire)
		}
		switch fieldNum {
		default:
			iNdEx = preIndex
			skippy, err := skipService(dAtA[iNdEx:])
			if err != nil {
				return err
			}
			if (skippy < 0) || (iNdEx+skippy) < 0 {
				return ErrInvalidLengthService
			}
			if (iNdEx + skippy) > l {
				return io.ErrUnexpectedEOF
			}
			m.XXX_unrecognized = append(m.XXX_unrecognized, dAtA[iNdEx:iNdEx+skippy]...)
			iNdEx += skippy
		}
	}

	if iNdEx > l {
		return io.ErrUnexpectedEOF
	}
	return nil
}
func (m *BenchmarkRsp) Unmarshal(dAtA []byte) error {
	l := len(dAtA)
	iNdEx := 0
	for iNdEx < l {
		preIndex := iNdEx
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if shift >= 64 {
				return ErrIntOverflowService
			}
			if iNdEx >= l {
				return io.ErrUnexpectedEOF
			}
			b := dAtA[iNdEx]
			iNdEx++
			wire |= uint64(b&0x7F) << shift
			if b < 0x80 {
				break
			}
		}
		fieldNum := int32(wire >> 3)
		wireType := int(wire & 0x7)
		if wireType == 4 {
			return fmt.Errorf("proto: BenchmarkRsp: wiretype end group for non-group")
		}
		if fieldNum <= 0 {
			return fmt.Errorf("proto: BenchmarkRsp: illegal tag %d (wire type %d)", fieldNum, wire)
		}
		switch fieldNum {
		default:
			iNdEx = preIndex
			skippy, err := skipService(dAtA[iNdEx:])
			if err != nil {
				return err
			}
			if (skippy < 0) || (iNdEx+skippy) < 0 {
				return ErrInvalidLengthService
			}
			if (iNdEx + skippy) > l {
				return io.ErrUnexpectedEOF
			}
			m.XXX_unrecognized = append(m.XXX_unrecognized, dAtA[iNdEx:iNdEx+skippy]...)
			iNdEx += skippy
		}
	}

	if iNdEx > l {
		return io.ErrUnexpectedEOF
	}
	return nil
}
func skipService(dAtA []byte) (n int, err error) {
	l := len(dAtA)
	iNdEx := 0
	depth := 0
	for iNdEx < l {
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if shift >= 64 {
				return 0, ErrIntOverflowService
			}
			if iNdEx >= l {
				return 0, io.ErrUnexpectedEOF
			}
			b := dAtA[iNdEx]
			iNdEx++
			wire |= (uint64(b) & 0x7F) << shift
			if b < 0x80 {
				break
			}
		}
		wireType := int(wire & 0x7)
		switch wireType {
		case 0:
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return 0, ErrIntOverflowService
				}
				if iNdEx >= l {
					return 0, io.ErrUnexpectedEOF
				}
				iNdEx++
				if dAtA[iNdEx-1] < 0x80 {
					break
				}
			}
		case 1:
			iNdEx += 8
		case 2:
			var length int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return 0, ErrIntOverflowService
				}
				if iNdEx >= l {
					return 0, io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				length |= (int(b) & 0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			if length < 0 {
				return 0, ErrInvalidLengthService
			}
			iNdEx += length
		case 3:
			depth++
		case 4:
			if depth == 0 {
				return 0, ErrUnexpectedEndOfGroupService
			}
			depth--
		case 5:
			iNdEx += 4
		default:
			return 0, fmt.Errorf("proto: illegal wireType %d", wireType)
		}
		if iNdEx < 0 {
			return 0, ErrInvalidLengthService
		}
		if depth == 0 {
			return iNdEx, nil
		}
	}
	return 0, io.ErrUnexpectedEOF
}

var (
	ErrInvalidLengthService        = fmt.Errorf("proto: negative length found during unmarshaling")
	ErrIntOverflowService          = fmt.Errorf("proto: integer overflow")
	ErrUnexpectedEndOfGroupService = fmt.Errorf("proto: unexpected end of group")
)
