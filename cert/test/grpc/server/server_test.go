package main

import (
	"context"
	"reflect"
	"testing"

	"github.com/lxt1045/utils/log"
)

type X struct {
	A int
}
type I interface {
	Test(x X)
}

type Y struct {
}

func (y *Y) Test(x X) {
}

func (y *Y) Try(x X) {

}

func TestReflect(t *testing.T) {
	var x I = &Y{}
	y := &Y{}

	typ := reflect.TypeOf(x)
	t.Logf("typ Kind:%s", typ.Elem().Kind())

	is := []I{x} // []slice 的Elem 类型和 struct 的Field(i) 的Kind有可能是 Interface，其他很难做成是 Interface

	te := reflect.TypeOf(is).Elem()
	t.Logf("typ Kind:%s", te.Kind())

	for i := 0; i < te.NumMethod(); i++ {
		m := te.Method(i)
		log.Ctx(context.TODO()).Info().Caller().Interface("m", m).Send()
	}

	var xx interface{} = &x
	xxTyp := reflect.TypeOf(xx).Elem()
	t.Logf("typ Kind:%s", xxTyp.Kind())
	for i := 0; i < xxTyp.NumMethod(); i++ {
		m := xxTyp.Method(i)
		log.Ctx(context.TODO()).Info().Caller().Interface("m", m).Send()
	}

	typY := reflect.TypeOf(y)
	t.Logf("typY Kind:%s", typY.Elem().Kind())
	for i := 0; i < typY.NumMethod(); i++ {
		m := typY.Method(i)
		log.Ctx(context.TODO()).Info().Caller().Interface("m", m).Send()
		mType := m.Type
		for j := 0; j < mType.NumIn(); j++ {
			arg := mType.In(j)
			log.Ctx(context.TODO()).Info().Caller().Msgf("arg:%s, %s", arg, arg.Kind())
		}
	}

}
