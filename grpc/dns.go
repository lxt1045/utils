package grpc

import (
	"math/rand"
	"sync"

	"github.com/lxt1045/errors"
	"google.golang.org/grpc/resolver"
)

var (
	addrsStore = map[string][]string{}
	lock       sync.RWMutex
)

func init() {
	// Register the example ResolverBuilder. This is usually done in a package's
	// init() function.
	resolver.Register(&resolverBuilder{})
}

func RegisterDNS(addr map[string][]string) (err error) {
	lock.Lock()
	defer lock.Unlock()

	for k, v := range addr {
		if _, ok := addrsStore[k]; ok {
			return errors.Errorf("addr is already exists")
		}
		addrsStore[k] = v
	}
	return
}

type resolverBuilder struct{}

func (*resolverBuilder) Build(target resolver.Target, cc resolver.ClientConn, opts resolver.BuildOptions) (resolver.Resolver, error) {
	r := &grpcResolver{
		target:     target,
		cc:         cc,
		addrsStore: addrsStore,
	}
	r.start()
	return r, nil
}

// grpc.Dial("grpc:lxt1045.com")
func (*resolverBuilder) Scheme() string {
	return "grpc"
}

type grpcResolver struct {
	target     resolver.Target
	cc         resolver.ClientConn
	addrsStore map[string][]string
}

func (r *grpcResolver) start() {
	ep := r.target.Endpoint()
	lock.RLock()
	addrStrs := r.addrsStore[ep]
	lock.RUnlock()

	idx := rand.Intn(len(addrStrs))
	addrs := make([]resolver.Address, len(addrStrs))
	for i, s := range addrStrs {
		addrs[(i+idx)%len(addrStrs)] = resolver.Address{Addr: s}
	}
	r.cc.UpdateState(resolver.State{Addresses: addrs})
}
func (r *grpcResolver) ResolveNow(o resolver.ResolveNowOptions) {
	// 重新解析一次，可能会多次调用
	r.start()
}
func (*grpcResolver) Close() {}
