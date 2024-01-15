package main

import (
	"context"
	"fmt"
	"math"
	"net"
	"time"

	middleware "github.com/grpc-ecosystem/go-grpc-middleware"
	validator "github.com/grpc-ecosystem/go-grpc-middleware/validator"
	"github.com/lxt1045/utils/cert/test/grpc/filesystem"
	"github.com/lxt1045/utils/cert/test/grpc/pb"
	"github.com/lxt1045/utils/config"
	wegrpc "github.com/lxt1045/utils/grpc"
	"github.com/lxt1045/utils/log"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
)

type server struct{}

func (this *server) SayHello(ctx context.Context, in *pb.HelloReq) (out *pb.HelloRsp, err error) {
	l := log.Ctx(ctx).Info()
	md, ok := metadata.FromIncomingContext(ctx)
	if ok {
		// return nil, status.Error(codes.Internal, "failed to get metadata")
		l = l.Interface("md", md)
	}
	md2, ok := metadata.FromOutgoingContext(ctx)
	if ok {
		// return nil, status.Error(codes.Internal, "failed to get metadata")
		l = l.Interface("md2", md2)
	}
	// authHeader, ok := md["authorization"]
	// if !ok {
	// 	return nil, status.Error(codes.Unauthenticated, "missing authorization header")
	// }
	l.Caller().Msg("SayHello")

	logid, ok := ctx.Value("logid").(int64)
	if ok {
		log.Ctx(ctx).Info().Caller().Int64("extra", logid).Msg("SayHello")
	}
	logid2, ok := ctx.Value("log_id").(string)
	if ok {
		log.Ctx(ctx).Info().Caller().Str("logid2", logid2).Msg("SayHello")
	}

	return &pb.HelloRsp{Msg: "hello"}, nil
}

func main() {
	ln, err := net.Listen("tcp", ":10088")
	if err != nil {
		fmt.Println("network error", err)
	}

	// 从输入证书文件和密钥文件为服务端构造TLS凭证
	// creds, err := credentials.NewServerTLSFromFile("../pkg/tls/server.pem", "../pkg/tls/server.key")
	// if err != nil {
	// 	log.Fatalf("Failed to generate credentials %v", err)
	// }
	certFile, keyFile, caFile := "static/ca/server-cert.pem", "static/ca/server-key.pem", "static/ca/root-cert.pem"
	tlsCert, err := config.LoadTLSConfig(filesystem.Static, certFile, keyFile, caFile)
	if err != nil {
		fmt.Println("Service error", err)
		return
	}
	creds := credentials.NewTLS(tlsCert)

	// l := rszlog.New(os.Stdout)

	// 新建gRPC服务器实例,并开启TLS认证
	grpcSrv := grpc.NewServer(
		grpc.Creds(creds),
		grpc.UnaryInterceptor(
			middleware.ChainUnaryServer(
				validator.UnaryServerInterceptor(),
				wegrpc.LogUnaryServiceInterceptor(),
				// zap.UnaryServerInterceptor(zapLogger),
				// logger.UnaryServerInterceptor(wegrpc.InterceptorLogger(l)),
			),
		),
		grpc.StreamInterceptor(
			middleware.ChainStreamServer(
				validator.StreamServerInterceptor(),
				wegrpc.LogStreamServiceInterceptor(),
			),
		),
		grpc.MaxSendMsgSize(math.MaxInt32),
	)

	// myServer := grpc.NewServer(
	// 	grpc.StreamInterceptor(middleware.ChainStreamServer(
	// 		grpc_ctxtags.StreamServerInterceptor(),
	// 		grpc_opentracing.StreamServerInterceptor(),
	// 		grpc_prometheus.StreamServerInterceptor,
	// 		grpc_zap.StreamServerInterceptor(zapLogger),
	// 		grpc_auth.StreamServerInterceptor(myAuthFunction),
	// 		grpc_recovery.StreamServerInterceptor(),
	// 	)),
	// 	grpc.UnaryInterceptor(grpc_middleware.ChainUnaryServer(
	// 		grpc_ctxtags.UnaryServerInterceptor(),
	// 		grpc_opentracing.UnaryServerInterceptor(),
	// 		grpc_prometheus.UnaryServerInterceptor,
	// 		zap.UnaryServerInterceptor(zapLogger),
	// 		grpc_auth.UnaryServerInterceptor(myAuthFunction),
	// 		grpc_recovery.UnaryServerInterceptor(),
	// 	)),
	// )

	//注册服务
	pb.RegisterHelloServer(grpcSrv, &server{})
	sinfo := grpcSrv.GetServiceInfo()
	log.Ctx(context.TODO()).Info().Caller().Interface("sinfo", sinfo).Send()

	err = grpcSrv.Serve(ln)
	if err != nil {
		fmt.Println("Service error", err)
		time.Sleep(time.Second)
	}

	fmt.Println("end")
	time.Sleep(time.Second)
}
