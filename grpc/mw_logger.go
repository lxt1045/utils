package grpc

import (
	"strconv"
	"time"

	"github.com/lxt1045/errors"
	"github.com/lxt1045/utils/gid"
	"github.com/lxt1045/utils/log"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"
)

func i64ToStr(i int64) string {
	return strconv.FormatInt(i, 10)
}

const (
	grpcLogIDKey = "log_id"

	fromLog  = 1
	fromGRPC = 2
)

func getLogid(ctx context.Context) (logid int64, from uint8) {
	logid, ok := log.Logid(ctx)
	if ok {
		from = fromLog
		return
	}
	logids := metadata.ValueFromIncomingContext(ctx, grpcLogIDKey)
	if len(logids) > 0 {
		logid, _ = strconv.ParseInt(logids[0], 0, 64)
		if logid > 0 {
			from = fromGRPC
			return
		}
	}
	logid = gid.GetGID()
	return
}

func GRPCContext(ctx context.Context) context.Context {
	logid, from := getLogid(ctx)
	if from != fromLog {
		ctx, _ = log.WithLogid(ctx, logid)
	}
	ctx = metadata.NewOutgoingContext(ctx, metadata.Pairs(
		grpcLogIDKey, strconv.FormatInt(logid, 10),
	))
	return ctx
}

func LogUnaryClientInterceptor() grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req, reply interface{},
		cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) (err error) {

		start := time.Now()
		ctx = GRPCContext(ctx)
		ctx = metadata.AppendToOutgoingContext(ctx,
			"c_start", i64ToStr(start.UnixNano()),
		)
		defer func() {
			e := recover()

			loss := int64(time.Since(start))
			l := log.DeferLogger(ctx, loss, err, e)

			l.Caller().
				Interface("req", req).
				Interface("reply", reply).
				Str("method", method).
				Msg("end")
		}()

		// 可以看做是当前 RPC 方法，一般在拦截器中调用 invoker 能达到调用 RPC 方法的效果，当然底层也是 gRPC 在处理。
		// 调用RPC方法(invoking RPC method)
		err = invoker(ctx, method, req, reply, cc, opts...)

		return err
	}
}
func LogStreamClientInterceptor() grpc.StreamClientInterceptor {
	return func(ctx context.Context, desc *grpc.StreamDesc, cc *grpc.ClientConn,
		method string, streamer grpc.Streamer, opts ...grpc.CallOption) (s grpc.ClientStream, err error) {

		start := time.Now()
		ctx = GRPCContext(ctx)
		ctx = metadata.AppendToOutgoingContext(ctx,
			"c_start", i64ToStr(start.UnixNano()),
		)
		defer func() {
			e := recover()

			loss := int64(time.Since(start))
			l := log.DeferLogger(ctx, loss, err, e)

			l.Caller().Str("method", method).Msg("end")
		}()

		stream, err := streamer(ctx, desc, cc, method)

		return &streamClient{ClientStream: stream, ctx: ctx}, err
	}
}

// 嵌入式 streamClient 允许我们访问SendMsg和RecvMsg函数
type streamClient struct {
	grpc.ClientStream
	ctx context.Context
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
		if e != nil {
			loss := int64(time.Since(start))
			l := log.DeferLogger(ctx, loss, err, e)

			l.Caller().Interface("req", m).Msg("RecvMsg")
		}
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
		if e != nil {
			loss := int64(time.Since(start))
			l := log.DeferLogger(ctx, loss, err, e)

			l.Caller().Interface("resp", m).Msg("SendMsg")
		}
	}()

	if err := s.ClientStream.SendMsg(m); err != nil {
		return err
	}

	return nil
}

func LogUnaryServiceInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler) (resp interface{}, err error) {

		start := time.Now()
		ctx = GRPCContext(ctx)
		ctx = metadata.AppendToOutgoingContext(ctx,
			"s_start", i64ToStr(start.UnixNano()),
		)

		remote, _ := peer.FromContext(ctx)
		remoteAddr := remote.Addr.String()

		defer func() {
			e := recover()

			loss := int64(time.Since(start))
			l := log.DeferLogger(ctx, loss, err, e)

			if e != nil {
				if err != nil {
					err = errors.Errorf("recover:%+v, err:%+v", e, err)
				} else {
					err = errors.Errorf("recover:%+v", e)
				}
			}

			l.Caller().
				Str("ip", remoteAddr).
				Str("method", info.FullMethod).
				Interface("in", req).
				Interface("out", resp).
				Msg("end")
		}()

		resp, err = handler(ctx, req)
		return
	}
}

func LogStreamServiceInterceptor() grpc.StreamServerInterceptor {
	return func(srv interface{}, stream grpc.ServerStream, info *grpc.StreamServerInfo,
		handler grpc.StreamHandler) (err error) {

		start := time.Now()
		ctx := stream.Context()
		ctx = GRPCContext(ctx)
		ctx = metadata.AppendToOutgoingContext(ctx,
			"s_start", i64ToStr(start.UnixNano()),
		)

		defer func() {
			e := recover()

			loss := int64(time.Since(start))
			l := log.DeferLogger(ctx, loss, err, e)

			l.Caller().Msg("end")
		}()
		wrapper := &streamService{
			ServerStream: stream,
			ctx:          ctx,
		}
		return handler(srv, wrapper)
	}
}

type streamService struct {
	grpc.ServerStream
	ctx context.Context
}

func (s *streamService) Context() context.Context {
	return s.ctx
}
func (s *streamService) RecvMsg(m interface{}) (err error) {
	start := time.Now()
	ctx := s.Context()
	defer func() {
		e := recover()
		if e != nil {
			loss := int64(time.Since(start))
			l := log.DeferLogger(ctx, loss, err, e)

			l.Caller().Interface("req", m).Msg("RecvMsg")
		}
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
		if e != nil {
			loss := int64(time.Since(start))
			l := log.DeferLogger(ctx, loss, err, e)

			l.Caller().Interface("resp", m).Msg("SendMsg")
		}
	}()

	if err = s.ServerStream.SendMsg(m); err != nil {
		return err
	}
	return nil
}
