package grpc

import (
	"context"
	"strconv"
	"strings"
	"time"

	"github.com/lxt1045/errors"
	"github.com/lxt1045/errors/zerolog"
	elog "github.com/lxt1045/errors/zerolog"
	"github.com/lxt1045/utils/log"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"
)

func LogUnaryClientInterceptor(name string) grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req, reply interface{},
		cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) (err error) {

		start := time.Now()
		ctx = GRPCContext(ctx)
		ctx = metadata.AppendToOutgoingContext(ctx, cliStartKey, i64ToStr(start.UnixMilli()))
		ctx = metadata.AppendToOutgoingContext(ctx, cliNameKey, name)

		defer func() {
			e := recover()
			loss := int64(time.Since(start)) / int64(time.Millisecond)
			var l *elog.Event
			switch {
			case e != nil:
				l = log.Ctx(ctx).Error().Any("recover", e).Array("stack", errors.ZerologStack(5))
			case err != nil:
				l = log.Ctx(ctx).Error().Err(err)
			case loss >= warnTimeout:
				l = log.Ctx(ctx).Warn()
			default:
				l = log.Ctx(ctx).Trace()
			}

			l.Str("caller", getCaller(l, method, "")).
				Int64("duration/ms", loss).
				Interface("req", req).
				Interface("reply", reply).
				Str("method", method).
				Str("client", name).
				Str("service", trimServiceName(cc.CanonicalTarget())).
				Msg("grpc client")
		}()

		// 可以看做是当前 RPC 方法，一般在拦截器中调用 invoker 能达到调用 RPC 方法的效果，当然底层也是 gRPC 在处理。
		// 调用RPC方法(invoking RPC method)
		err = invoker(ctx, method, req, reply, cc, opts...)

		return err
	}
}

func LogStreamClientInterceptor(name string) grpc.StreamClientInterceptor {
	return func(ctx context.Context, desc *grpc.StreamDesc, cc *grpc.ClientConn,
		method string, streamer grpc.Streamer, opts ...grpc.CallOption) (s grpc.ClientStream, err error) {

		start := time.Now()
		ctx = GRPCContext(ctx)
		ctx = metadata.AppendToOutgoingContext(ctx, cliStartKey, i64ToStr(start.UnixMilli()))
		ctx = metadata.AppendToOutgoingContext(ctx, cliNameKey, name)

		defer func() {
			e := recover()
			loss := int64(time.Since(start)) / int64(time.Millisecond)

			var l *elog.Event
			switch {
			case e != nil:
				l = log.Ctx(ctx).Error().Any("recover", e).Array("stack", errors.ZerologStack(5))
			case err != nil:
				l = log.Ctx(ctx).Error().Err(err)
			case loss >= warnTimeout:
				l = log.Ctx(ctx).Warn()
			default:
				l = log.Ctx(ctx).Trace()
			}

			l.Str("caller", getCaller(l, method, "")).
				Int64("duration/ms", loss).
				Str("method", method).
				Str("stream_name", desc.StreamName).
				Bool("client_stream", desc.ClientStreams).
				Str("client", name).
				Str("service", trimServiceName(cc.CanonicalTarget())).
				Msg("grpc stream client")
		}()

		stream, err := streamer(ctx, desc, cc, method)

		return &streamClient{
			ClientStream: stream,
			ctx:          ctx,

			method:        method,
			streamName:    desc.StreamName,
			clientStreams: desc.ClientStreams,
			client:        name,
			service:       trimServiceName(cc.CanonicalTarget()),
		}, err
	}
}

func trimServiceName(str string) string {
	if pre := "grpc:///"; strings.HasPrefix(str, pre) {
		return str[len(pre):]
	}
	if pre := "etcd:///" + etcdPathPre; strings.HasPrefix(str, pre) {
		return str[len(pre):]
	}
	return str
}

// 嵌入式 streamClient 允许我们访问SendMsg和RecvMsg函数
type streamClient struct {
	grpc.ClientStream
	ctx context.Context

	method        string
	streamName    string
	clientStreams bool
	client        string
	service       string
}

func (s *streamClient) Context() context.Context {
	if s.ctx != nil {
		return s.ctx
	}
	return s.ClientStream.Context()
}

// RecvMsg从流中接收消息
func (s *streamClient) RecvMsg(m interface{}) (err error) {
	start := time.Now()
	ctx := s.Context()
	defer func() {
		e := recover()
		var l *zerolog.Event
		if e != nil {
			l = log.Ctx(ctx).Error().
				Array("stack", errors.ZerologStack(5)).
				Interface("recover", e).
				Int64("duration/ms", int64(time.Since(start))/int64(time.Millisecond))
		} else {
			l = log.Ctx(ctx).Trace()
		}
		l.Str("caller", getCaller(l, s.method, "Recv")).
			Str("method", s.method).
			Str("stream_name", s.streamName).
			Bool("client_stream", s.clientStreams).
			Str("client", s.client).
			Str("service", s.service).
			Interface("msg", m).Msg("grpc stream RecvMsg")
	}()

	if err = s.ClientStream.RecvMsg(m); err != nil {
		return err
	}
	return nil
}

// RecvMsg从流中接收消息
func (s *streamClient) SendMsg(m interface{}) (err error) {
	start := time.Now()
	ctx := s.Context()
	defer func() {
		e := recover()
		var l *zerolog.Event
		if e != nil {
			l = log.Ctx(ctx).Error().
				Array("stack", errors.ZerologStack(5)).
				Interface("recover", e).
				Int64("duration/ms", int64(time.Since(start))/int64(time.Millisecond))
		} else {
			l = log.Ctx(ctx).Trace()
		}
		l.Str("caller", getCaller(l, s.method, "Send")).
			Str("method", s.method).
			Str("stream_name", s.streamName).
			Bool("client_stream", s.clientStreams).
			Str("client", s.client).
			Str("service", s.service).
			Interface("msg", m).Msg("grpc stream SendMsg")
	}()

	if err := s.ClientStream.SendMsg(m); err != nil {
		return err
	}

	return nil
}

func LogUnaryServiceInterceptor(name string) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler) (resp interface{}, err error) {

		start := time.Now()
		ms := start.UnixMilli()
		ctx = GRPCContext(ctx)
		// ctx = metadata.AppendToOutgoingContext(ctx, svcStart, i64ToStr(ms))
		cStarts := metadata.ValueFromIncomingContext(ctx, cliStartKey)
		if len(cStarts) > 0 {
			cStartTs, _ := strconv.ParseInt(cStarts[0], 0, 64)
			ms = ms - cStartTs
		}
		cliName := ""
		if cliNames := metadata.ValueFromIncomingContext(ctx, cliNameKey); len(cliNames) > 0 {
			cliName = cliNames[0]
		}

		remoteAddr := ""
		if remote, _ := peer.FromContext(ctx); remote != nil {
			remoteAddr = remote.Addr.String()
		}

		defer func() {
			e := recover()
			loss := int64(time.Since(start)) / int64(time.Millisecond)

			var l *elog.Event
			switch {
			case e != nil:
				l = log.Ctx(ctx).Error().Any("recover", e).Array("stack", errors.ZerologStack(5))
			case err != nil:
				l = log.Ctx(ctx).Error().Err(err)
			case loss >= warnTimeout:
				l = log.Ctx(ctx).Warn()
			case ms >= 500: // 网络延时+排队延时
				l = log.Ctx(ctx).Warn()
			default:
				l = log.Ctx(ctx).Trace()
			}

			l.Str("caller", getMethod(l, info.Server, info.FullMethod)).
				Int64("duration/ms", loss).
				Int64("network/ms", ms).
				Str("addr", remoteAddr).
				Str("method", info.FullMethod).
				Any("req", req).
				Any("resp", resp).
				Str("client", cliName).
				Str("sevice", name).
				Msg("grpc service")
		}()

		resp, err = handler(ctx, req)
		return
	}
}

func LogStreamServiceInterceptor(name string) grpc.StreamServerInterceptor {
	return func(srv interface{}, stream grpc.ServerStream, info *grpc.StreamServerInfo,
		handler grpc.StreamHandler) (err error) {

		start := time.Now()
		ms := start.UnixMilli()
		ctx := stream.Context()
		ctx = GRPCContext(ctx)
		// ctx = metadata.AppendToOutgoingContext(ctx, svcStart, i64ToStr(start.UnixNano()))
		cStarts := metadata.ValueFromIncomingContext(ctx, cliStartKey)
		if len(cStarts) > 0 {
			cStartTs, _ := strconv.ParseInt(cStarts[0], 0, 64)
			ms = ms - cStartTs
		}
		cliName := ""
		if cliNames := metadata.ValueFromIncomingContext(ctx, cliNameKey); len(cliNames) > 0 {
			cliName = cliNames[0]
		}

		remoteAddr := ""
		if remote, _ := peer.FromContext(ctx); remote != nil {
			remoteAddr = remote.Addr.String()
		}

		defer func() {
			e := recover()
			loss := int64(time.Since(start)) / int64(time.Millisecond)

			var l *elog.Event
			switch {
			case loss >= warnTimeout:
				l = log.Ctx(ctx).Warn()
			case e != nil:
				l = log.Ctx(ctx).Error().Any("recover", e).Array("stack", errors.ZerologStack(5))
			case err != nil:
				l = log.Ctx(ctx).Error().Err(err)
			case ms >= 500: // 网络延时+排队延时
				l = log.Ctx(ctx).Warn()
			default:
				l = log.Ctx(ctx).Trace()
			}

			l.Str("caller", getMethod(l, srv, info.FullMethod)).
				Int64("duration/ms", loss).Int64("network/ms", ms).
				Str("addr", remoteAddr).
				Str("method", info.FullMethod).
				Bool("client_stream", info.IsClientStream).
				Str("client", cliName).
				Str("sevice", name).
				Msg("grpc stream service end")
		}()

		l := log.Ctx(ctx).Trace()
		l.Str("caller", getMethod(l, srv, info.FullMethod)).
			Int64("network/ms", ms).
			Str("addr", remoteAddr).
			Str("method", info.FullMethod).
			Bool("client_stream", info.IsClientStream).
			Str("client", cliName).
			Str("sevice", name).
			Msg("grpc stream service start")
		wrapper := &streamService{
			ServerStream: stream,
			ctx:          ctx,

			addr:         remoteAddr,
			method:       info.FullMethod,
			clientstream: info.IsClientStream,
			client:       cliName,
			sevice:       name,
		}
		return handler(srv, wrapper)
	}
}

type streamService struct {
	grpc.ServerStream
	ctx context.Context

	addr         string
	method       string
	clientstream bool
	client       string
	sevice       string
}

func (s *streamService) Context() context.Context {
	return s.ctx
}
func (s *streamService) RecvMsg(m interface{}) (err error) {
	start := time.Now()
	ctx := s.Context()
	defer func() {
		e := recover()
		var l *zerolog.Event
		if e != nil {
			l = log.Ctx(ctx).Error().
				Array("stack", errors.ZerologStack(5)).
				Interface("recover", e).
				Int64("duration/ms", int64(time.Since(start))/int64(time.Millisecond))
		} else {
			l = log.Ctx(ctx).Trace()
		}
		l.Str("caller", getCaller(l, s.method, "Recv")).
			Str("addr", s.addr).
			Str("method", s.addr).
			Bool("client_stream", s.clientstream).
			Str("client", s.client).
			Str("sevice", s.sevice).
			Interface("msg", m).Msg("grpc stream RecvMsg")
	}()

	if err = s.ServerStream.RecvMsg(m); err != nil {
		return err
	}
	return nil
}

func (s *streamService) SendMsg(m interface{}) (err error) {
	start := time.Now()
	ctx := s.Context()
	defer func() {
		e := recover()
		var l *zerolog.Event
		if e != nil {
			l = log.Ctx(ctx).Error().
				Array("stack", errors.ZerologStack(5)).
				Interface("recover", e).
				Int64("duration/ms", int64(time.Since(start))/int64(time.Millisecond))
		} else {
			l = log.Ctx(ctx).Trace()
		}
		l.Str("caller", getCaller(l, s.method, "Send")).
			Str("addr", s.addr).
			Str("method", s.method).
			Bool("client_stream", s.clientstream).
			Str("client", s.client).
			Str("sevice", s.sevice).
			Interface("msg", m).Msg("grpc stream SendMsg")
	}()

	if err = s.ServerStream.SendMsg(m); err != nil {
		return err
	}
	return nil
}
