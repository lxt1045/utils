package grpc

import (
	"context"
	"io/fs"
	"math"
	"net"
	"strings"
	"time"

	protovalidatego "buf.build/go/protovalidate"
	middleware "github.com/grpc-ecosystem/go-grpc-middleware"
	"github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/protovalidate"
	"github.com/lxt1045/errors"
	"github.com/lxt1045/utils/config"
	"github.com/lxt1045/utils/log"
	etcdcli "go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/client/v3/naming/endpoints"
	etcdnaming "go.etcd.io/etcd/client/v3/naming/resolver"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

type ConfSvc struct {
	CACert     string
	ServerKey  string
	ServerCert string
	Addr       string
	SvcName    string
}

type ConfCli struct {
	CACert     string
	ClientKey  string
	ClientCert string
	SvcName    string
	CliName    string
	SvcAddrs   []string
}

const (
	etcdPathPre = "/services/grpc/"
)

type Server struct {
	ln          net.Listener
	GRPC        *grpc.Server
	leaseCancel context.CancelFunc // 用于取消 keepalive
}

func (s *Server) Run(ctx context.Context, cancel func()) (err error) {
	if cancel != nil {
		defer cancel() // 如果退出了，就需要通知全局
	}

	err = s.GRPC.Serve(s.ln)
	if err != nil {
		err = errors.Errorf(err.Error())
		log.Ctx(ctx).Error().Err(err).Msg("grpc server got err")
	}
	return
}

func (s *Server) GracefulStop() {
	if s.leaseCancel != nil {
		s.leaseCancel() // 停止 keepalive，让 lease 过期从而删除 key
	}
	s.GRPC.GracefulStop()
}

func NewServerTLS(ctx context.Context, conf ConfSvc, efs fs.FS, cli *etcdcli.Client) (svc *Server, err error) {
	tlsConfig, err := config.LoadTLSConfig(efs, conf.ServerCert, conf.ServerKey, conf.CACert)
	if err != nil {
		return
	}

	tlsConfig.ServerName = conf.SvcName
	creds := credentials.NewTLS(tlsConfig)

	return NewServer(ctx, conf.SvcName, conf.Addr, cli, grpc.Creds(creds))

}

func NewServer(ctx context.Context, serviceName, addr string, cli *etcdcli.Client, opts ...grpc.ServerOption) (svc *Server, err error) {
	strs := strings.Split(addr, ":")
	ln, err := net.Listen("tcp", ":"+strs[len(strs)-1])
	if err != nil {
		err = errors.Errorf(err.Error())
		return
	}

	protoValidator, err := protovalidatego.New() // 创建 protovalidate 验证器实例
	if err != nil {
		err = errors.Errorf(err.Error())
		return
	}

	SrcOpts := []grpc.ServerOption{
		grpc.UnaryInterceptor(
			middleware.ChainUnaryServer(
				protovalidate.UnaryServerInterceptor(protoValidator),
				LogUnaryServiceInterceptor(serviceName),
			),
		),
		grpc.StreamInterceptor(
			middleware.ChainStreamServer(
				protovalidate.StreamServerInterceptor(protoValidator),
				LogStreamServiceInterceptor(serviceName),
			),
		),
		grpc.MaxSendMsgSize(math.MaxInt32),
	}

	grpcSrv := grpc.NewServer(append(opts, SrcOpts...)...)
	svc = &Server{
		ln:   ln,
		GRPC: grpcSrv,
	}
	if cli != nil {
		var leaseCancel context.CancelFunc
		leaseCancel, err = RegisterServiceAddr(ctx, serviceName, addr, cli)
		if err != nil {
			err = errors.Errorf(err.Error())
			return
		}
		svc.leaseCancel = leaseCancel
	}
	return
}

// 服务端：将自身注册到 etcd，使用 lease 实现自动过期
func RegisterServiceAddr(ctx context.Context, serviceName, addr string, cli *etcdcli.Client) (cancel context.CancelFunc, err error) {
	// 创建一个 5 秒 TTL 的 lease
	leaseResp, err := cli.Grant(ctx, 5)
	if err != nil {
		return nil, errors.WithErr(err)
	}

	// 使用统一的目录前缀 "/services/" 来组织所有服务
	serviceKey := etcdPathPre + serviceName

	// 使用 endpoints.Manager 来管理服务实例
	em, err := endpoints.NewManager(cli, serviceKey)
	if err != nil {
		return nil, errors.WithErr(err)
	}

	// 添加服务端点，并关联 lease
	err = em.AddEndpoint(ctx, serviceKey+"/"+addr,
		endpoints.Endpoint{Addr: addr}, etcdcli.WithLease(leaseResp.ID))
	if err != nil {
		return nil, errors.WithErr(err)
	}

	// 启动 keepalive 保持 lease 活跃
	keepaliveCtx, keepaliveCancel := context.WithCancel(context.Background())
	keepaliveCh, err := cli.KeepAlive(keepaliveCtx, leaseResp.ID)
	if err != nil {
		keepaliveCancel()
		return nil, errors.WithErr(err)
	}

	// 启动 goroutine 消费 keepalive 响应
	go func() {
		for range keepaliveCh {
			// 消费 keepalive 响应，保持 lease 活跃
		}
	}()

	return keepaliveCancel, nil
}

// CheckServiceExists 检查 etcd 中是否存在服务端点
func CheckServiceExists(ctx context.Context, serviceKey string, cli *etcdcli.Client) error {
	em, err := endpoints.NewManager(cli, serviceKey)
	if err != nil {
		return errors.WithErr(err)
	}

	// 列出该服务下的所有端点
	endpointsMap, err := em.List(ctx)
	if err != nil {
		return errors.WithErr(err)
	}

	if len(endpointsMap) == 0 {
		return errors.Errorf("service not found: %s", serviceKey)
	}

	return nil
}

// 客户端：创建 etcd resolver 并建立 gRPC 连接
func EtcdDial(serviceName string, cli *etcdcli.Client, opts ...grpc.DialOption) (conn *grpc.ClientConn, err error) {
	// 创建 etcd 的 resolver builder
	r, err := etcdnaming.NewBuilder(cli)
	if err != nil {
		err = errors.WithErr(err)
		return
	}
	// 建立 gRPC 连接，通过服务名 "etcd:///serviceName" 寻址
	// 同时可配置负载均衡策略为 round_robin[reference:5]
	conn, err = grpc.NewClient(
		r.Scheme()+":///"+serviceName,
		append(opts, grpc.WithResolvers(r), grpc.WithDefaultServiceConfig(`{"loadBalancingPolicy":"round_robin"}`))...,
	)
	if err != nil {
		err = errors.WithErr(err)
		return
	}
	return
}

func NewClientTLS(ctx context.Context, conf ConfCli, efs fs.FS, cli *etcdcli.Client) (conn *grpc.ClientConn, err error) {
	if cli == nil {
		err = RegisterDNS(map[string][]string{
			conf.SvcName: conf.SvcAddrs,
		})
		if err != nil {
			err = errors.Errorf("RegisterDNS:", err.Error())
			return
		}
	}
	tlsConfig, err := config.LoadTLSConfig(efs, conf.ClientCert, conf.ClientKey, conf.CACert)
	if err != nil {
		err = errors.Errorf(err.Error())
		return
	}
	tlsConfig.ServerName = conf.SvcName
	creds := credentials.NewTLS(tlsConfig)
	return NewClient(ctx, conf.CliName, conf.SvcName, cli, grpc.WithTransportCredentials(creds))
}

func NewClient(ctx context.Context, clientName, serviceName string, cli *etcdcli.Client, opts ...grpc.DialOption) (conn *grpc.ClientConn, err error) {
	CliOpts := []grpc.DialOption{
		grpc.WithConnectParams(grpc.ConnectParams{MinConnectTimeout: time.Second * 3}),
		grpc.WithUnaryInterceptor(LogUnaryClientInterceptor(clientName)),
		grpc.WithStreamInterceptor(LogStreamClientInterceptor(clientName)),
	}
	if cli == nil {
		conn, err = grpc.NewClient("grpc:///"+serviceName, append(opts, CliOpts...)...)
		if err != nil {
			err = errors.Errorf("network:", err.Error())
			return
		}
		return
	}
	// 使用与注册时相同的目录前缀
	return EtcdDial(etcdPathPre+serviceName, cli, append(opts, CliOpts...)...)
}
