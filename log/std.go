package log

import (
	"context"
	"io"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/lxt1045/errors/zerolog"
	cmap "github.com/orcaman/concurrent-map/v2"
	"github.com/petermattis/goid"
)

var (
	gidLoggers = cmap.NewWithCustomShardingFunction[int64, *zerolog.Logger](func(key int64) (hash uint32) {
		hash = uint32(key)
		return
	})
	gidLoggersOnce sync.Once
)

func GetStdLogger(ctx context.Context) *zerolog.Logger {
	gid := goid.Get()
	l, _ := gidLoggers.Get(gid)
	if l != nil {
		return l
	}

	return Ctx(ctx)
}

func SetStdLogger(ctx context.Context) {
	gid := goid.Get()
	gidLoggers.Set(gid, Ctx(ctx))
}

func GetStdOutput(name string) io.Writer {
	return output
}

// gid 获取上下文
type StdWriter struct {
	ctx    context.Context
	logger *zerolog.Logger
}

func (w *StdWriter) Write(p []byte) (n int, err error) {
	GetStdLogger(w.ctx).Info().Caller().Msg(string(p))
	return

}

// clearStdLogger 清理无用的日志句柄； 执行速度比较慢不能频繁调用
func clearStdLogger() {
	gids := getAllGIDs()
	m := make(map[int64]struct{})
	for _, gid := range gids {
		m[int64(gid)] = struct{}{}
	}
	for item := range gidLoggers.IterBuffered() {
		k := item.Key
		if _, ok := m[k]; !ok {
			gidLoggers.Remove(k)
		}
	}
}

func NewStdWriter(ctx context.Context) *StdWriter {
	gidLoggersOnce.Do(func() {
		go func() {
			for {
				time.Sleep(time.Second * 60)

				clearStdLogger()
			}
		}()
	})
	return &StdWriter{
		ctx:    ctx,
		logger: Ctx(ctx),
	}
}

func getAllGIDs() []int64 {
	// 获取所有goroutine的堆栈信息
	buf := make([]byte, 1<<20) // 1MB缓冲区
	stackSize := runtime.Stack(buf, true)
	stack := string(buf[:stackSize])

	var gids []int64
	lines := strings.Split(stack, "\n")

	for _, line := range lines {
		if strings.HasPrefix(line, "goroutine ") {
			// 示例行: "goroutine 1 [running]:"
			parts := strings.Fields(line)
			if len(parts) < 2 {
				continue
			}

			// 提取goroutine ID
			idStr := parts[1]
			if id, err := strconv.ParseInt(idStr, 10, 64); err == nil {
				gids = append(gids, id)
			}
		}
	}
	return gids
}
