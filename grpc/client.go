package grpc

import (
	"context"
	"embed"

	"github.com/lxt1045/errors"
	"github.com/lxt1045/utils/config"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

func InitClient(ctx context.Context, efs embed.FS, conf config.Conn) (conn *grpc.ClientConn, err error) {
	cmtls := conf.TLS
	tlsGRPCConfig, err := config.LoadTLSConfig(efs, cmtls.ClientCert, cmtls.ClientKey, cmtls.CACert)
	if err != nil {
		return
	}
	tlsGRPCConfig.ServerName = conf.Host

	grpcCreds := credentials.NewTLS(tlsGRPCConfig)

	err = RegisterDNS(map[string][]string{
		conf.Host: {conf.Addr},
	})
	if err != nil {
		return
	}

	conn, err = grpc.Dial("grpc:///"+conf.Host,
		grpc.WithTransportCredentials(grpcCreds),
		grpc.WithUnaryInterceptor(LogUnaryClientInterceptor()),
		grpc.WithStreamInterceptor(LogStreamClientInterceptor()),
	)
	if err != nil {
		err = errors.Errorf(err.Error())
		return
	}

	return
}
