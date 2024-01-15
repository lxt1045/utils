package call

import (
	"context"
	"reflect"
	"sync"
	"unsafe"

	"github.com/lxt1045/errors"
	"github.com/lxt1045/utils/grpc"
	"github.com/lxt1045/utils/log"
)

var (
	svcMethodsLock sync.Mutex
	svcMethods     = make([]Method, 0, 32)
	svcInterfaces  = make(map[string]uint32) // map[func]id
)

type Method struct {
	Func       func(unsafe.Pointer, context.Context, unsafe.Pointer) (unsafe.Pointer, error)
	SvcPointer unsafe.Pointer
	ReqType    reflect.Type
	RespType   reflect.Type
	Idx        uint32
	Name       string
}

// Rigester fun: pb.RegisterHelloService, service: implementation
func Rigester(ctx context.Context, fun interface{}, service interface{}) (err error) {

	return
}
func rigester(ctx context.Context, fun interface{}, service interface{}) (err error) {
	regMethodType := reflect.TypeOf(fun)
	if regMethodType.Kind() != reflect.Func {
		err = errors.Errorf("arg fun must be func")
		return
	}
	if regMethodType.NumIn() != 2 || regMethodType.NumOut() != 0 {
		err = errors.Errorf("arg fun args error")
		return
	}
	arg0 := regMethodType.In(0)
	argGrpcService := reflect.TypeOf(&grpc.Server{})
	if arg0.String() != argGrpcService.String() {
		err = errors.Errorf("arg fun args error, arg0:%s, argGrpcService:%s", arg0, argGrpcService)
		return
	}

	ifaceType := regMethodType.In(1)
	if ifaceType.Kind() != reflect.Interface {
		err = errors.Errorf("arg fun args error")
		return
	}
	svcMethodNum := ifaceType.NumMethod()
	if svcMethodNum == 0 {
		err = errors.Errorf("service %s has no funcs", ifaceType.String())
		return
	}

	// 判断是否实现了接口
	svcImpType := reflect.TypeOf(service)
	if !svcImpType.Implements(ifaceType) {
		err = errors.Errorf("the handler of type %v that does not satisfy %v", svcImpType, ifaceType)
		return
	}
	svcValue := reflect.ValueOf(service)

	log.Ctx(ctx).Info().Caller().Str("type", svcImpType.String()).Send()

	svcMethodsLock.Lock()
	defer svcMethodsLock.Unlock()

	for i := 0; i < ifaceType.NumMethod(); i++ {
		method := ifaceType.Method(i)
		mType := method.Type

		methodKey := ifaceType.String() + "." + method.Name

		// 输入参数 和 输出参数 都需要注册到 msg 里去，以便序列化和反序列化

		if _, ok := svcInterfaces[methodKey]; ok {
			err = errors.Errorf("The function is registered: %s", methodKey)
			return
		}

		svcImpMethod, ok := svcImpType.MethodByName(method.Name)
		if !ok {
			err = errors.Errorf("svcImpType.MethodByName not ok")
			return
		}
		pMethodValue := svcImpMethod.Func.Interface()
		fNewValue := reflect.ValueOf(func(unsafe.Pointer, context.Context, unsafe.Pointer) (unsafe.Pointer, error) { return nil, nil }).Interface()

		ifMethod := (*[2]uintptr)(unsafe.Pointer(&pMethodValue))
		ifNew := (*[2]uintptr)(unsafe.Pointer(&fNewValue))
		ifNew[1] = ifMethod[1]
		f, ok := fNewValue.(func(unsafe.Pointer, context.Context, unsafe.Pointer) (unsafe.Pointer, error))
		if !ok {
			err = errors.Errorf("svcImpType.MethodByName not ok")
			return
		}

		idx := uint32(len(svcMethods))
		if int(idx) != len(svcMethods) {
			err = errors.Errorf("The number of methods exceeds the limit")
			return
		}
		svcInterfaces[methodKey] = idx
		svcMethods = append(svcMethods, Method{
			Func:       f,
			SvcPointer: svcValue.UnsafePointer(),
			ReqType:    mType.In(1).Elem(), // 形参是指针，所以要换成原始数据结构形式
			RespType:   mType.Out(0).Elem(),
		})
	}

	return
}

func MethodIdx(method string) (idx uint32, exist bool) {
	idx, exist = svcInterfaces[method]
	return
}

func Call(ctx context.Context, methodIdx uint32, req unsafe.Pointer) (resp unsafe.Pointer, err error) {
	// if int(methodIdx) >= len(methods) {
	// 	err = errors.Errorf("methodIdx is out of range")
	// 	return
	// }

	method := svcMethods[methodIdx]
	resp, err = method.Func(method.SvcPointer, ctx, req)
	return
}

type MethodOut struct {
	Method
	Name string
}

func AllInterfaces() (is []MethodOut) {
	keys := make([]string, len(svcInterfaces))
	for k, v := range svcInterfaces {
		keys[v] = k
	}
	for i, m := range svcMethods {
		is = append(is, MethodOut{
			Method: m,
			Name:   keys[i],
		})
	}
	return
}
