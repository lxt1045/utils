package call

import (
	"context"
)

// var (
// 	methodsLock sync.Mutex
// 	methods     = make([]Method, 0, 32)
// 	ifaces      = make(map[string]uint32) // map[func]id
// )

// type Method struct {
// 	Func       func(unsafe.Pointer, context.Context, unsafe.Pointer) (unsafe.Pointer, error)
// 	SvcPointer unsafe.Pointer
// 	ReqType    reflect.Type
// 	RespType   reflect.Type
// }

type Client struct {
	Funcs map[string]Method
}

func (c Client) Invoke(ctx context.Context, method string, req interface{}) (resp interface{}, err error) {

	return
}

// NewClient fun: pb.RegisterHelloServer(s *grpc.Server, srv HelloServer)
func NewClient(ctx context.Context, fun interface{}) (client Client, err error) {

	return
}
