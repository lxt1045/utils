package log

import (
	"bytes"
	"context"
	"log/slog"
	"runtime"
	"strconv"
	"time"

	"github.com/lxt1045/errors"
	eslog "github.com/lxt1045/errors/slog"
	"github.com/lxt1045/errors/zerolog"
	cmap "github.com/orcaman/concurrent-map/v2"
	"github.com/petermattis/goid"
)

var (
	stdLoggers = cmap.NewWithCustomShardingFunction[int64, *zerolog.Logger](
		func(key int64) (hash uint32) {
			return uint32(key)
		})
	_ = func() bool {
		go func() {
			ticker := time.NewTicker(time.Minute)
			lastTs := time.Now().Unix()
			diffTs := int64(30 * 60) // 30min 执行一次
			for {
				<-ticker.C
				if time.Now().Unix() < diffTs+lastTs {
					continue
				}
				// 数量很少的时候，就没必要处理了
				if stdLoggers.Count() > 1024*2 {
					clearStdLogger()
				}

				lastTs = time.Now().Unix()
			}
		}()
		return true
	}()
)

func clearStdLogger() {

	// ids := igoid.AllGoids() // igoid "github.com/isyscore/isc-gobase/goid"
	ids := getAllGIDs()
	m := make(map[int64]bool, len(ids))
	for _, id := range ids {
		m[id] = true
	}
	dels := []int64{}
	for _, k := range stdLoggers.Keys() {
		if !m[k] {
			dels = append(dels, k)
		}
	}
	for _, k := range dels {
		stdLoggers.Remove(k)
	}

	// for item := range gidLoggers.IterBuffered() {
	// 	k := item.Key
	// 	if _, ok := m[k]; !ok {
	// 		gidLoggers.Remove(k)
	// 	}
	// }
}

func StdLogger() (log *zerolog.Logger) {
	return stdLogger(goid.Get())
}

func stdLogger(gid int64) (log *zerolog.Logger) {
	l, _ := stdLoggers.Get(gid)
	if l != nil {
		return l
	}
	log = Ctx(context.TODO())
	stdLoggers.Set(gid, log)
	return
}

func SetStdLogger(log *zerolog.Logger) {
	stdLoggers.Set(goid.Get(), log)
}
func ClearStdLogger() {
	stdLoggers.Remove(goid.Get())
}

var stackSize = 512

func getAllGIDs() []int64 {
	// 获取所有goroutine的堆栈信息

	count := runtime.NumGoroutine()
	size := count * stackSize
	buf := make([]byte, size)
	for {
		n := runtime.Stack(buf, true)
		if n < len(buf) {
			buf = buf[:n]
			if n > count*stackSize {
				stackSize = n/count + 1
			}
			break
		}
		buf = make([]byte, len(buf)*2)
	}
	// stack := string(buf[:n])

	var gids []int64
	con := []byte("goroutine ")
	for {
		i := bytes.Index(buf, con)
		if i < 0 {
			break
		}
		buf = buf[i+len(con):]
		idStr := bytes.SplitN(buf, []byte(" "), 2)[0]
		if id, err := strconv.ParseInt(string(idStr), 10, 64); err == nil {
			gids = append(gids, id)
		}
	}

	// for _, line := range bytes.Split(buf, []byte("\n")) {
	// 	if bytes.HasPrefix(line, []byte("goroutine ")) {
	// 		// 示例行: "goroutine 1 [running]:"
	// 		parts := bytes.Fields(line)
	// 		if len(parts) < 2 {
	// 			continue
	// 		}
	// 		// 提取goroutine ID
	// 		idStr := parts[1]
	// 		if id, err := strconv.ParseInt(string(idStr), 10, 64); err == nil {
	// 			gids = append(gids, id)
	// 		}
	// 	}
	// }
	return gids
}

func Debug(args ...interface{}) {
	StdLogger().Debug().PrintWithPC(errors.GetPC(), args...)
}
func Debugf(format string, args ...interface{}) {
	StdLogger().Debug().PrintfWithPC(errors.GetPC(), format, args...)
}
func Debugln(args ...interface{}) {
	StdLogger().Debug().PrintWithPC(errors.GetPC(), args...)
}
func Error(args ...interface{}) {
	StdLogger().Error().PrintWithPC(errors.GetPC(), args...)
}
func Errorf(format string, args ...interface{}) {
	StdLogger().Error().PrintfWithPC(errors.GetPC(), format, args...)
}
func Errorln(args ...interface{}) {
	StdLogger().Error().PrintWithPC(errors.GetPC(), args...)
}
func Info(args ...interface{}) {
	StdLogger().Info().PrintWithPC(errors.GetPC(), args...)
}
func Infof(format string, args ...interface{}) {
	StdLogger().Info().PrintfWithPC(errors.GetPC(), format, args...)
}
func Infoln(args ...interface{}) {
	StdLogger().Info().PrintWithPC(errors.GetPC(), args...)
}

func Fatal(args ...interface{}) {
	StdLogger().Fatal().PrintWithPC(errors.GetPC(), args...)
}
func Fatalf(format string, args ...interface{}) {
	StdLogger().Fatal().PrintfWithPC(errors.GetPC(), format, args...)
}
func Fatalln(args ...interface{}) {
	StdLogger().Fatal().PrintWithPC(errors.GetPC(), args...)
}

func Panic(args ...interface{}) {
	StdLogger().Panic().PrintWithPC(errors.GetPC(), args...)
}
func Panicf(format string, args ...interface{}) {
	StdLogger().Panic().PrintfWithPC(errors.GetPC(), format, args...)
}
func Panicln(args ...interface{}) {
	StdLogger().Panic().PrintWithPC(errors.GetPC(), args...)
}

func Print(level slog.Level, args ...interface{}) {
	StdLogger().WithLevel(eslog.LevelFromSlog(level)).PrintWithPC(errors.GetPC(), args...)
}
func Printf(level slog.Level, format string, args ...interface{}) {
	StdLogger().WithLevel(eslog.LevelFromSlog(level)).PrintfWithPC(errors.GetPC(), format, args...)
}
func Println(level slog.Level, args ...interface{}) {
	StdLogger().WithLevel(eslog.LevelFromSlog(level)).PrintWithPC(errors.GetPC(), args...)
}
func Warn(args ...interface{}) {
	StdLogger().Warn().PrintWithPC(errors.GetPC(), args...)
}
func Warnf(format string, args ...interface{}) {
	StdLogger().Warn().PrintfWithPC(errors.GetPC(), format, args...)
}
func Warnln(args ...interface{}) {
	StdLogger().Warn().PrintWithPC(errors.GetPC(), args...)
}

func DebugContext(ctx context.Context, args ...interface{}) {
	var l *zerolog.Logger
	if logid := Logid(ctx); logid > 0 {
		l = Ctx(ctx)
	} else {
		l = StdLogger()
	}
	l.Debug().PrintWithPC(errors.GetPC(), args...)
}
func InfoContext(ctx context.Context, args ...interface{}) {
	var l *zerolog.Logger
	if logid := Logid(ctx); logid > 0 {
		l = Ctx(ctx)
	} else {
		l = StdLogger()
	}
	l.Info().PrintWithPC(errors.GetPC(), args...)
}
func WarnContext(ctx context.Context, args ...interface{}) {
	var l *zerolog.Logger
	if logid := Logid(ctx); logid > 0 {
		l = Ctx(ctx)
	} else {
		l = StdLogger()
	}
	l.Warn().PrintWithPC(errors.GetPC(), args...)
}
func ErrorContext(ctx context.Context, args ...interface{}) {
	var l *zerolog.Logger
	if logid := Logid(ctx); logid > 0 {
		l = Ctx(ctx)
	} else {
		l = StdLogger()
	}
	l.Error().PrintWithPC(errors.GetPC(), args...)
}
func PrintContext(ctx context.Context, level slog.Level, args ...interface{}) {
	var l *zerolog.Logger
	if logid := Logid(ctx); logid > 0 {
		l = Ctx(ctx)
	} else {
		l = StdLogger()
	}
	l.WithLevel(eslog.LevelFromSlog(level)).PrintWithPC(errors.GetPC(), args...)
}
func LogContext(ctx context.Context, level slog.Level, attrs ...slog.Attr) {
	var l *zerolog.Logger
	if logid := Logid(ctx); logid > 0 {
		l = Ctx(ctx)
	} else {
		l = StdLogger()
	}
	l.WithLevel(eslog.LevelFromSlog(level)).WithCaller(uintptr(errors.GetPC())).SlogAttr(attrs...).Send()
}

func DebugfContext(ctx context.Context, format string, args ...interface{}) {
	var l *zerolog.Logger
	if logid := Logid(ctx); logid > 0 {
		l = Ctx(ctx)
	} else {
		l = StdLogger()
	}
	l.Debug().PrintfWithPC(errors.GetPC(), format, args...)
}
func InfofContext(ctx context.Context, format string, args ...interface{}) {
	var l *zerolog.Logger
	if logid := Logid(ctx); logid > 0 {
		l = Ctx(ctx)
	} else {
		l = StdLogger()
	}
	l.Info().PrintfWithPC(errors.GetPC(), format, args...)
}
func WarnfContext(ctx context.Context, format string, args ...interface{}) {
	var l *zerolog.Logger
	if logid := Logid(ctx); logid > 0 {
		l = Ctx(ctx)
	} else {
		l = StdLogger()
	}
	l.Warn().PrintfWithPC(errors.GetPC(), format, args...)
}
func ErrorfContext(ctx context.Context, format string, args ...interface{}) {
	var l *zerolog.Logger
	if logid := Logid(ctx); logid > 0 {
		l = Ctx(ctx)
	} else {
		l = StdLogger()
	}
	l.Error().PrintfWithPC(errors.GetPC(), format, args...)
}
func PrintfContext(ctx context.Context, level slog.Level, format string, args ...interface{}) {
	var l *zerolog.Logger
	if logid := Logid(ctx); logid > 0 {
		l = Ctx(ctx)
	} else {
		l = StdLogger()
	}
	l.WithLevel(eslog.LevelFromSlog(level)).PrintfWithPC(errors.GetPC(), format, args...)
}
func LogfContext(ctx context.Context, level slog.Level, format string, attrs ...slog.Attr) {
	var l *zerolog.Logger
	if logid := Logid(ctx); logid > 0 {
		l = Ctx(ctx)
	} else {
		l = StdLogger()
	}
	l.WithLevel(eslog.LevelFromSlog(level)).WithCaller(uintptr(errors.GetPC())).SlogAttr(attrs...).Msg(format)
}
