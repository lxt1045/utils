package cache

import (
	"reflect"
	"runtime"
	"strings"
	"sync/atomic"
	"unsafe"

	"github.com/lxt1045/errors"
)

var (
	NotExist = errors.New("key not exist")
)

type Cache[T Value] interface {
	GetWithInfo(key string) (d T, expired bool, err error)
	Set(key string, d T) error
	Close() error
	Del(keys ...string) error
}

var (
	mFuncName = func() (p atomic.Pointer[map[uintptr]string]) {
		p.Store(&map[uintptr]string{})
		return
	}()
)

func GetFuncName(i interface{}) (name string) {
	p := reflect.ValueOf(i).Pointer()
	m := *mFuncName.Load()
	if name = m[p]; name != "" {
		return name
	}

	name = runtime.FuncForPC(p).Name() // 通过 reflect.ValueOf 获取函数的 PC（程序计数器）
	if idx := strings.LastIndex(name, "/"); idx > 0 {
		name = name[idx+1:]
	}

	mm := make(map[uintptr]string, len(m)+1)
	for k, v := range m {
		mm[k] = v
	}
	mm[p] = name
	mFuncName.Store(&mm)
	return
}

func toBs(str string) []byte {
	return unsafe.Slice(unsafe.StringData(str), len(str))
}
