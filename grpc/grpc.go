package grpc

import (
	"context"
	"embed"
	"math"
	"net"

	middleware "github.com/grpc-ecosystem/go-grpc-middleware"
	validator "github.com/grpc-ecosystem/go-grpc-middleware/validator"
	"github.com/lxt1045/errors"
	"github.com/lxt1045/utils/config"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

type Server struct {
	ln   net.Listener
	GRPC *grpc.Server
}

func (s *Server) Run(ctx context.Context, cancel func()) (err error) {
	if cancel != nil {
		defer cancel() // 如果退出了，就需要通知全局
	}

	err = s.GRPC.Serve(s.ln)
	if err != nil {
		err = errors.Errorf(err.Error())
	}
	return
}

func (s *Server) GracefulStop() {
	s.GRPC.GracefulStop()
}

func NewServer(ctx context.Context, conf config.GRPC, efs embed.FS) (svc *Server, err error) {
	ln, err := net.Listen(conf.Protocol, conf.Addr)
	if err != nil {
		err = errors.Errorf(err.Error())
		return
	}

	// 从输入证书文件和密钥文件为服务端构造TLS凭证
	// creds, err := credentials.NewServerTLSFromFile("../pkg/tls/server.pem", "../pkg/tls/server.key")
	// if err != nil {
	// 	log.Fatalf("Failed to generate credentials %v", err)
	// }
	tlsConfig, err := config.LoadTLSConfig(efs, conf.ServerCert, conf.ServerKey, conf.CACert)
	if err != nil {
		return
	}
	creds := credentials.NewTLS(tlsConfig)

	// l := rszlog.New(os.Stdout)

	// 新建gRPC服务器实例,并开启TLS认证
	grpcSrv := grpc.NewServer(
		grpc.Creds(creds),
		grpc.UnaryInterceptor(
			middleware.ChainUnaryServer(
				validator.UnaryServerInterceptor(),
				LogUnaryServiceInterceptor(),
				// zap.UnaryServerInterceptor(zapLogger),
				// logger.UnaryServerInterceptor(InterceptorLogger(l)),
			),
		),
		grpc.StreamInterceptor(
			middleware.ChainStreamServer(
				validator.StreamServerInterceptor(),
				LogStreamServiceInterceptor(),
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

	svc = &Server{
		ln:   ln,
		GRPC: grpcSrv,
	}

	return
}

func NewClient(ctx context.Context, conf config.GRPC, efs embed.FS) (conn *grpc.ClientConn, err error) {
	// 从输入证书文件和密钥文件为服务端构造TLS凭证
	// creds, err := credentials.NewServerTLSFromFile("../pkg/tls/server.pem", "../pkg/tls/server.key")
	// if err != nil {
	// 	log.Fatalf("Failed to generate credentials %v", err)
	// }
	tlsConfig, err := config.LoadTLSConfig(efs, conf.ServerCert, conf.ServerKey, conf.CACert)
	if err != nil {
		err = errors.Errorf(err.Error())
		return
	}
	creds := credentials.NewTLS(tlsConfig)
	err = RegisterDNS(map[string][]string{
		conf.Host: conf.HostAddrs,
	})
	if err != nil {
		err = errors.Errorf("RegisterDNS:", err.Error())
		return
	}
	// 连接服务器
	conn, err = grpc.Dial("grpc:///"+conf.Host,
		// grpc.WithBalancerName("round_robin"),
		grpc.WithTransportCredentials(creds),
		grpc.WithUnaryInterceptor(LogUnaryClientInterceptor()),
		grpc.WithStreamInterceptor(LogStreamClientInterceptor()),
	)
	// conn.ServerName = ""
	if err != nil {
		err = errors.Errorf("network:", err.Error())
		return
	}
	return
}
