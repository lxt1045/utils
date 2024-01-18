package call

import (
	"context"
	"reflect"
	"strings"
	"unsafe"

	"github.com/lxt1045/errors"
	"github.com/lxt1045/utils/grpc"
	"github.com/lxt1045/utils/msg/codec"
)

var _ codec.Caller = Method{} // 检查是由实现接口

type Method struct {
	Func       func(unsafe.Pointer, context.Context, unsafe.Pointer) (unsafe.Pointer, error)
	SvcPointer unsafe.Pointer
	reqType    reflect.Type
	respType   reflect.Type
	// bStream    bool // 是否双向流
}

func (m Method) ReqType() reflect.Type {
	return m.reqType
}
func (m Method) RespType() reflect.Type {
	return m.respType
}

func (m Method) NewReq() codec.Msg {
	msg := reflect.New(m.reqType).Interface().(codec.Msg)
	return msg
}
func (m Method) NewResp() codec.Msg {
	if m.respType == nil {
		return nil
	}
	msg := reflect.New(m.respType).Interface().(codec.Msg)
	return msg
}

func (m Method) SvcInvoke(ctx context.Context, req codec.Msg) (resp codec.Msg, err error) {
	p := (*[2]unsafe.Pointer)(unsafe.Pointer(&req))[1]
	pr, err := m.Func(m.SvcPointer, ctx, p)
	if err != nil || pr == nil {
		return
	}
	resp = reflect.NewAt(m.respType, pr).Interface().(codec.Msg)
	return
}

type MethodFull struct {
	Method
	Name   string
	CallID uint16
}

func getMethods(ctx context.Context, fun interface{}, service interface{}) (methods []MethodFull, err error) {
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
	if ifaceType.NumMethod() == 0 {
		err = errors.Errorf("service %s has no funcs", ifaceType.String())
		return
	}

	// 判断是否实现了接口
	if service != nil {
		if svcImpType := reflect.TypeOf(service); !svcImpType.Implements(ifaceType) {
			err = errors.Errorf("the handler of type %v that does not satisfy %v", svcImpType, ifaceType)
			return
		}
	}

	for i := 0; i < ifaceType.NumMethod(); i++ {
		method := ifaceType.Method(i)
		mType := method.Type

		// stream 模式，抛弃
		if mType.NumOut() == 1 || mType.NumIn() < 2 {
			continue
		}

		reqType := mType.In(1).Elem()   // 形参是指针，所以要换成原始数据结构形式
		respType := mType.Out(0).Elem() // 同上

		// 判断该 Call 是否需要返回，如果不需要返回则会直接不管返回值，提高返回性能
		if respType.NumField() <= 3 && strings.HasSuffix(respType.String(), ".Empty") {
			bFound := false
			for i := 0; i < respType.NumField(); i++ {
				field := respType.Field(i)
				if reflect.StructTag(field.Tag).Get("protobuf") != "" {
					bFound = true
				}
			}
			if !bFound {
				respType = nil // 返回值成员为空的时候
			}
		}

		m := MethodFull{
			Method: Method{
				reqType:  reqType,
				respType: respType,
			},
			Name: ifaceType.String() + "." + method.Name,
		}

		if service != nil {
			methodTarget := reflect.ValueOf(
				func(unsafe.Pointer, context.Context, unsafe.Pointer) (unsafe.Pointer, error) {
					return nil, nil
				},
			).Interface()

			svcImpType := reflect.TypeOf(service)
			svcImpMethod, ok := svcImpType.MethodByName(method.Name)
			if !ok {
				err = errors.Errorf("svcImpType.MethodByName not ok")
				return
			}
			methodSource := svcImpMethod.Func.Interface()
			ifMethod := (*[2]uintptr)(unsafe.Pointer(&methodSource))
			ifNew := (*[2]uintptr)(unsafe.Pointer(&methodTarget))
			ifNew[1] = ifMethod[1] // 改变 type 保留原始指针 ptr
			f, ok := methodTarget.(func(unsafe.Pointer, context.Context, unsafe.Pointer) (unsafe.Pointer, error))
			if !ok {
				err = errors.Errorf("svcImpType.MethodByName not ok")
				return
			}
			m.Func = f
			m.SvcPointer = reflect.ValueOf(service).UnsafePointer()
		}

		methods = append(methods, m)
	}

	return
}
