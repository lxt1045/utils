package grpc

import (
	"context"
	"embed"
	"math"
	"net"
	"time"

	middleware "github.com/grpc-ecosystem/go-grpc-middleware"
	validator "github.com/grpc-ecosystem/go-grpc-middleware/validator"
	"github.com/lxt1045/errors"
	log "github.com/lxt1045/errors/zerolog"
	"github.com/lxt1045/utils/config"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

type Server struct {
	ln   net.Listener
	GRPC *grpc.Server
}

func (s *Server) Run(ctx context.Context, cancel func()) (err error) {
	defer cancel() // 如果退出了，就需要通知全局

	err = s.GRPC.Serve(s.ln)
	if err != nil {
		log.Ctx(ctx).Fatal().Err(err).Send()
	}
	log.Ctx(ctx).Info().Msg("end...")
	time.Sleep(time.Second)
	return
}

func (s *Server) GracefulStop() {
	s.GRPC.GracefulStop()
}

func InitGRPC(ctx context.Context, conf config.GRPC, efs embed.FS) (svc *Server, err error) {
	ln, err := net.Listen(conf.Protocol, conf.Addr)
	if err != nil {
		err = errors.Errorf(err.Error())
		log.Ctx(ctx).Fatal().Err(err).Send()
	}

	// 从输入证书文件和密钥文件为服务端构造TLS凭证
	tlsConfig, err := config.LoadTLSConfig(efs, conf.ServerCert, conf.ServerKey, conf.CACert)
	if err != nil {
		log.Ctx(ctx).Fatal().Err(err).Send()
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
	svc = &Server{
		ln:   ln,
		GRPC: grpcSrv,
	}

	return
}
