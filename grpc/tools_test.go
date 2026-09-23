package grpc

import (
	"runtime"
	"testing"

	"github.com/lxt1045/errors/zerolog"
)

func callDeepCaller(deep int, full, method string) string {
	if deep > 0 {
		return callDeepCaller(deep-1, full, method)
	}
	return getCaller(&zerolog.Event{}, full, method)
}

func callDeepCallerWarp(deep int, full, method string) string {
	return callDeepCaller(deep, full, method)
}

func Test_getCaller(t *testing.T) {
	t.Logf("caller: %s", callDeepCallerWarp(5, "/grpc.Gateway/callDeepCallerWarp", ""))
	t.Logf("caller: %s", callDeepCallerWarp(5, "/grpc.Gateway/xxxxx", "callDeepCallerWarp"))
}

func Benchmark_getCaller(b *testing.B) {
	b.Run("1-5", func(b *testing.B) {
		for range b.N {
			callDeepCallerWarp(5, "/grpc.Gateway/callDeepCallerWarp", "")
		}
	})
	b.Run("2-5", func(b *testing.B) {
		for range b.N {
			callDeepCallerWarp(5, "/grpc.Gateway/xxxxx", "callDeepCallerWarp")
		}
	})
	b.Run("3-5", func(b *testing.B) {
		s := [6]uintptr{}
		for range b.N {
			runtime.Callers(5, s[:])
		}
	})
	b.Run("1-10", func(b *testing.B) {
		for range b.N {
			callDeepCallerWarp(10, "/grpc.Gateway/callDeepCallerWarp", "")
		}
	})
	b.Run("2-10", func(b *testing.B) {
		for range b.N {
			callDeepCallerWarp(10, "/grpc.Gateway/xxxxx", "callDeepCallerWarp")
		}
	})
	b.Run("3-10", func(b *testing.B) {
		s := [11]uintptr{}
		for range b.N {
			runtime.Callers(10, s[:])
		}
	})
	b.Run("1-20", func(b *testing.B) {
		for range b.N {
			callDeepCallerWarp(20, "/grpc.Gateway/callDeepCallerWarp", "")
		}
	})
	b.Run("2-20", func(b *testing.B) {
		for range b.N {
			callDeepCallerWarp(20, "/grpc.Gateway/xxxxx", "callDeepCallerWarp")
		}
	})
	b.Run("3-20", func(b *testing.B) {
		s := [21]uintptr{}
		for range b.N {
			runtime.Callers(20, s[:])
		}
	})
}
