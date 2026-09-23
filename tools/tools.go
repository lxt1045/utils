package tools

import (
	"context"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"
	"unsafe"

	"github.com/lxt1045/utils/log"
)

func ToMill(t time.Time) int64 {
	return t.UnixNano() / int64(time.Millisecond)
}

func WaitSysSignal(ctx context.Context) {
	// SIGINT	2	Term	用户发送INTR字符(Ctrl+C)触发
	// SIGTERM	15	Term	结束程序(可以被捕获、阻塞或忽略)
	// SIGHUP	1	Term	终端控制进程结束(终端连接断开)
	// SIGQUIT	3	Core	用户发送QUIT字符(Ctrl+/)触发
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP, syscall.SIGQUIT)
	for exit := false; !exit; {
		select {
		case s := <-ch:
			switch s {
			case syscall.SIGQUIT:
				log.Ctx(ctx).Info().Caller().Msg("SIGSTOP")
				exit = true
			case syscall.SIGHUP:
				log.Ctx(ctx).Info().Caller().Msg("SIGHUP")
				// exit = true
			case syscall.SIGINT:
				log.Ctx(ctx).Info().Caller().Msg("SIGINT")
				exit = true
			case syscall.SIGTERM:
				log.Ctx(ctx).Info().Caller().Msg("SIGINT")
				exit = true
			default:
				log.Ctx(ctx).Info().Caller().Msgf("default:%v", s)
				// exit = true
			}
		case <-ctx.Done():
			exit = true
		}
	}
}

func PathExists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil { //文件或者目录存在
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

func EqualStrs(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i, s := range a {
		if s != b[i] {
			return false
		}
	}
	return true
}

func Must[T any](p *T) (t T) {
	if p == nil {
		return
	}
	return *p
}

func ToP[T any](t T) (p *T) {
	return &t
}

func Split(x, y string) (xs []string) {
	if x == "" {
		return make([]string, 0)
	}
	xs = strings.Split(x, y)
	return
}

func FirstV[T comparable](x, y T) (t T) {
	if x != t {
		return x
	}
	return y
}

func FirstValue[T comparable](xs ...T) (t T) {
	for _, x := range xs {
		if x != t {
			t = x
			return
		}
	}
	return
}

func IfV[T comparable](x, y, z T) (t T) {
	if x == y {
		return z
	}
	return x
}

func JoinInt64(is []int64) (str string) {
	//  1895606206828122957
	//  12345678901234567890
	bs := make([]byte, 0, (20+1)*len(is))
	for _, id := range is {
		str := strconv.FormatInt(id, 10)
		if len(bs) > 0 {
			bs = append(bs, ',')
		}
		bs = append(bs, str...)
	}

	return unsafe.String(unsafe.SliceData(bs), len(bs))
}
