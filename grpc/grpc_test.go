package grpc_test

import (
	"context"
	"testing"
	"time"

	// "github.com/lxt1045/grpc-pb/base"
	"github.com/lxt1045/rpc/base"
	"github.com/lxt1045/utils/grpc"
	"github.com/lxt1045/utils/grpc/filesystem"
	"github.com/lxt1045/utils/log"
	etcdcli "go.etcd.io/etcd/client/v3"
	googlegrpc "google.golang.org/grpc"
)

var _ base.HelloServer = &Hello{}

type Hello struct {
	base.UnimplementedHelloServer
}

func (h *Hello) SayHello(ctx context.Context, req *base.HelloReq) (resp *base.HelloRsp, err error) {
	time.Sleep(time.Millisecond * 123)
	resp = &base.HelloRsp{
		Msg: "hello word!",
	}
	return
}

func TestGrpc(t *testing.T) {
	ctx := t.Context()
	conf := grpc.ConfSvc{
		Addr:       "127.0.0.1:60080",
		CACert:     "ca/root-cert.pem",
		ServerCert: "ca/server-cert.pem",
		ServerKey:  "ca/server-key.pem",
		SvcName:    "test.rpc",
	}
	grpcSrv, err := grpc.NewServerTLS(ctx, conf, filesystem.CA, nil)
	if err != nil {
		log.Ctx(ctx).Fatal().Err(err).Send()
		return
	}
	//注册服务
	base.RegisterHelloServer(grpcSrv.GRPC, &Hello{})
	go grpcSrv.Run(ctx, nil)
	defer grpcSrv.GracefulStop()

	confCli := grpc.ConfCli{
		CACert:     "ca/root-cert.pem",
		ClientCert: "ca/client-cert.pem",
		ClientKey:  "ca/client-key.pem",
		SvcName:    "test.rpc",
		CliName:    "test_client",
		SvcAddrs:   []string{"127.0.0.1:60080"},
	}
	conn, err := grpc.NewClientTLS(ctx, confCli, filesystem.CA, nil)
	if err != nil {
		log.Ctx(ctx).Fatal().Err(err).Send()
		return
	}
	defer conn.Close()

	cli := base.NewHelloClient(conn)

	// 使用较短的超时 + WaitForReady(false)，在服务不存在时快速失败
	callCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	req := &base.HelloReq{
		Name: "test",
		Test: "hello 12356789",
	}
	resp, err := cli.SayHello(callCtx, req, googlegrpc.WaitForReady(false))
	if err != nil {
		return
	}
	t.Logf("resp: %s", resp.Msg)
}

func TestGrpcEtcd(t *testing.T) {
	ctx := t.Context()
	cliEtcd, err := etcdcli.New(etcdcli.Config{
		Endpoints:   []string{"10.1.1.121:2379"},
		DialTimeout: 1 * time.Second,
		Username:    "root",
		Password:    "Qwe*123!@#",
	})
	if err != nil {
		t.Fatal(err)
		return
	}
	// defer cliEtcd.Close()
	// cliEtcd = nil

	conf := grpc.ConfSvc{
		Addr:       "127.0.0.1:60080",
		CACert:     "ca/root-cert.pem",
		ServerCert: "ca/server-cert.pem",
		ServerKey:  "ca/server-key.pem",
		SvcName:    "test.rpc",
	}
	grpcSrv, err := grpc.NewServerTLS(ctx, conf, filesystem.CA, cliEtcd)
	if err != nil {
		t.Fatal(err)
		return
	}
	//注册服务
	base.RegisterHelloServer(grpcSrv.GRPC, &Hello{})
	go grpcSrv.Run(ctx, nil)
	defer grpcSrv.GracefulStop()

	// 等待服务注册到 etcd 并让 resolver 发现
	time.Sleep(time.Millisecond * 200)

	confCli := grpc.ConfCli{
		CACert:     "ca/root-cert.pem",
		ClientCert: "ca/client-cert.pem",
		ClientKey:  "ca/client-key.pem",
		SvcName:    "test.rpc",
		CliName:    "test_client",
	}
	conn, err := grpc.NewClientTLS(ctx, confCli, filesystem.CA, cliEtcd)
	if err != nil {
		t.Fatal(err)
		return
	}
	defer conn.Close()

	// // 检查 etcd 中是否有该服务的 endpoint
	// err = grpc.CheckServiceExists(ctx, "/services/grpc/"+conf.Host, cliEtcd)
	// if err != nil {
	// 	t.Fatal(err)
	// 	return
	// }

	cli := base.NewHelloClient(conn)

	// 使用较短的超时 + WaitForReady(false)，在服务不存在时快速失败
	callCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	req := &base.HelloReq{
		Name: "test",
		Test: "hello 12356789",
	}
	resp, err := cli.SayHello(callCtx, req, googlegrpc.WaitForReady(false))
	if err != nil {
		t.Fatal(err)
		return
	}
	t.Logf("resp: %s", resp.Msg)
}
