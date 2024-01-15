package coder

import (
	"context"
	"reflect"
	"unsafe"
)

func Call(ctx context.Context, method string, req unsafe.Pointer) (resp unsafe.Pointer, err error) {

	// method := methods[methodIdx]
	// resp, err = method.Func(method.SvcPointer, ctx, req)

	typ := reflect.StructOf([]reflect.StructField{})
	reflect.MakeFunc()
	return
}
