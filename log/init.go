package log

import (
	"context"
	"io"
	"os"
	"sync/atomic"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/lxt1045/errors"
	"github.com/lxt1045/errors/zerolog"
	"github.com/lxt1045/utils/config"
	"github.com/lxt1045/utils/gid"
	"github.com/natefinch/lumberjack"
	rszlog "github.com/rs/zerolog"
)

type logID struct{}

var (
	output = func() (p atomic.Pointer[io.Writer]) {
		w := io.Writer(os.Stdout)
		p.Store(&w)
		return
	}()

	_ = func() bool {
		rszlog.TimeFieldFormat = time.RFC3339Nano // time 对 RFC3339Nano 类型做了特化处理
		return true
	}()
)

func Init(ctx context.Context, conf config.Log) (err error) {
	if conf.LogLevel != "" {
		err = SetGlobalLevel(conf.LogLevel)
		if err != nil {
			return
		}
	}

	if conf.Filename != "" {
		fileWriter := &lumberjack.Logger{
			Filename:   conf.Filename,
			MaxSize:    conf.MaxSize, // 每个日志文件的最大大小，以MB为单位
			MaxAge:     conf.MaxAge,
			MaxBackups: conf.MaxBackups, // 最大保留的旧日志文件数量
			Compress:   conf.Compress,   // 是否压缩旧的日志文件
			LocalTime:  conf.LocalTime,
		}
		if conf.ToConsole {
			SetOutput(rszlog.MultiLevelWriter(os.Stdout, fileWriter))
		} else {
			SetOutput(fileWriter)
		}
	}

	l := rszlog.New(GetOutput())
	rszlog.DefaultContextLogger = &l

	return
}

func SetOutput(w io.Writer) {
	if w != nil {
		output.Store(&w)
	}
}

func GetOutput() io.Writer {
	return *output.Load()
}

func New(writer ...io.Writer) *zerolog.Logger {
	w := GetOutput()
	if len(writer) > 0 && writer[0] != nil {
		w = writer[0]
	}
	l := zerolog.New(w)
	return &l
}

func SetGlobalLevel(level string) (err error) {
	l, err := Level(level)
	if err != nil {
		err = errors.Errorf(err.Error())
		return
	}
	zerolog.SetGlobalLevel(l)
	return
}

var levels = []rszlog.Level{
	rszlog.Level(zerolog.DebugLevel),
	rszlog.Level(zerolog.InfoLevel),
	rszlog.Level(zerolog.WarnLevel),
	rszlog.Level(zerolog.ErrorLevel),
	rszlog.Level(zerolog.FatalLevel),
	rszlog.Level(zerolog.PanicLevel),
	rszlog.Level(zerolog.NoLevel),
	rszlog.Level(zerolog.Disabled),
	rszlog.Level(zerolog.TraceLevel),
}

func Level(l string) (zerolog.Level, error) {
	for _, level := range levels {
		if level.String() == l {
			return zerolog.Level(level), nil
		}
	}
	return zerolog.Level(zerolog.DebugLevel), errors.Errorf("level not suport: %s", l)
}

func Ctx(ctx context.Context) *zerolog.Logger {
	switch c := ctx.(type) {
	case *gin.Context:
		return GinCtx(c)
	}

	_, ok := ctx.Value(logID{}).(int64)
	if ok {
		return zerolog.Ctx(ctx)
	}
	_, l := WithLogid(ctx, gid.New())
	return l
}

func Logid(ctx context.Context) (logid int64) {
	if c, ok := ctx.(*gin.Context); ok {
		return GinLogID(c)
	}
	logid, _ = ctx.Value(logID{}).(int64)
	return
}

func WithLogid(ctx context.Context, logid int64) (context.Context, *zerolog.Logger) {
	ctx = context.WithValue(ctx, logID{}, logid)

	l := zerolog.New(GetOutput())
	l = l.Hook(logidHook{logid: logid})

	return l.WithContext(ctx), &l
}

func RefleshLogid(ctx context.Context) context.Context {
	ctx, _ = WithLogid(ctx, gid.New())
	return ctx
}

type logidHook struct {
	logid int64
}

func (ch logidHook) Run(e *rszlog.Event, _ rszlog.Level, _ string) {
	e.Int64("logid", ch.logid)
}

