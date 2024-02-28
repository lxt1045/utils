package main

import (
	"context"
	"fmt"
	"io"
	"math"
	"net"
	"strconv"
	"time"

	middleware "github.com/grpc-ecosystem/go-grpc-middleware"
	validator "github.com/grpc-ecosystem/go-grpc-middleware/validator"
	"github.com/lxt1045/errors"
	"github.com/lxt1045/utils/cert/test/grpc/filesystem"
	"github.com/lxt1045/utils/cert/test/grpc/pb"
	"github.com/lxt1045/utils/config"
	wegrpc "github.com/lxt1045/utils/grpc"
	"github.com/lxt1045/utils/log"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"
)

type server struct{}

func (s *server) SayHello(ctx context.Context, in *pb.HelloReq) (out *pb.HelloRsp, err error) {
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
func (s *server) Benchmark(ctx context.Context, req *pb.BenchmarkReq) (*pb.BenchmarkRsp, error) {
	log.Ctx(ctx).Info().Caller().Msg("Benchmark")
	return &pb.BenchmarkRsp{}, nil
}

func (s *server) StreamHello(stream pb.Hello_StreamHelloServer) (err error) {
	for i := 0; ; i++ {
		ctx := stream.Context()
		pr, ok := peer.FromContext(ctx)
		if !ok {
			err = errors.Errorf("[getClinetIP] invoke FromContext() failed")
			return
		}
		res, err1 := stream.Recv()
		if err1 == io.EOF {
			return nil
		}
		log.Ctx(ctx).Info().Caller().Str("res", res.Name).Str("remote", pr.Addr.String()).Msg("StreamHello Recv")

		err = stream.Send(&pb.HelloRsp{
			Msg: "hello " + res.Name + " -> " + strconv.Itoa(i),
		})
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *server) StreamReqHello(stream pb.Hello_StreamReqHelloServer) (err error) {
	for i := 0; ; i++ {
		res, err := stream.Recv()
		//接收消息结束，发送结果，并关闭
		if err == io.EOF {
			return stream.SendAndClose(&pb.HelloRsp{}) // 返回值
		}
		if err != nil {
			return err
		}
		ctx := stream.Context()
		log.Ctx(ctx).Info().Interface("req", res).Msg("UploadFile")
	}
	return nil
}

func (s *server) StreamRespHello(req *pb.HelloReq, stream pb.Hello_StreamRespHelloServer) (err error) {
	for i := 0; i <= 3; i++ {
		err := stream.Send(&pb.HelloRsp{
			Msg: "hello",
		})
		if err != nil {
			return err
		}
	}
	ctx := stream.Context()
	log.Ctx(ctx).Info().Time("time", time.Now()).Caller().Interface("req", req).Msg("success!")
	return nil
}

func main1() {
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

func main() {
	ctx := context.Background()
	conf := config.GRPC{
		Protocol:   "tcp",
		Addr:       ":10087",
		ServerCert: "static/ca/server-cert.pem",
		ServerKey:  "static/ca/server-key.pem",
		CACert:     "static/ca/root-cert.pem",
	}
	svc, err := wegrpc.NewServer(ctx, conf, filesystem.Static)
	if err != nil {
		fmt.Println("network error", err)
	}

	//注册服务
	pb.RegisterHelloServer(svc.GRPC, &server{})
	sinfo := svc.GRPC.GetServiceInfo()
	log.Ctx(context.TODO()).Info().Caller().Interface("sinfo", sinfo).Send()

	err = svc.Run(ctx, nil)
	if err != nil {
		log.Ctx(context.TODO()).Error().Caller().Err(err).Msg("Service error")
		time.Sleep(time.Second)
	}

	svc.GracefulStop()
	time.Sleep(time.Second)
}
