package log

import (
	"context"
	"io"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/lxt1045/errors"
	"github.com/lxt1045/errors/zerolog"
	"github.com/lxt1045/utils/config"
	"github.com/lxt1045/utils/gid"
	"github.com/natefinch/lumberjack"
	rszlog "github.com/rs/zerolog"
)

var (
	output io.Writer = os.Stdout

	_ = func() bool {
		rszlog.TimeFieldFormat = time.RFC3339Nano
		return true
	}
)

func GetOutput() io.Writer {
	return output
}

func Init(ctx context.Context, conf config.Log) (err error) {
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
			output = rszlog.MultiLevelWriter(os.Stdout, fileWriter)
		} else {
			output = fileWriter
		}
	}

	if conf.LogLevel != "" {
		err = setGlobalLevel(conf.LogLevel)
		if err != nil {
			return
		}
	}

	return
}

type logID struct{}

const ginLogID = "logid"
const ginLogger = "logger"

func New(writer ...io.Writer) *zerolog.Logger {
	w := output
	if len(writer) > 0 && writer[0] != nil {
		w = writer[0]
	}
	l := zerolog.New(w)
	return &l
}

func setGlobalLevel(level string) (err error) {
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
	return zerolog.Level(zerolog.DebugLevel), errors.Errorf("err")
}

func Ctx(ctx context.Context) *zerolog.Logger {
	_, ok := ctx.Value(logID{}).(int64)
	if ok {
		return zerolog.Ctx(ctx)
	}
	_, l := WithLogid(ctx, gid.GetGID())
	return l
}

func Logid(ctx context.Context) (logid int64, ok bool) {
	logid, ok = ctx.Value(logID{}).(int64)
	return
}

func WithLogid(ctx context.Context, logid int64) (context.Context, *zerolog.Logger) {
	ctx = context.WithValue(ctx, logID{}, logid)

	l := zerolog.New(output)
	l = l.Hook(logidHook{logid: logid})
	// l = l.Hook(th) // l = l.With().Timestamp().Logger()

	return l.Logger.WithContext(ctx), &l
}

func GinGet(ctx *gin.Context) *zerolog.Logger {
	v, _ := ctx.Get(ginLogger)
	logger, ok := v.(*zerolog.Logger)
	if ok {
		return logger
	}

	vid, _ := ctx.Get(ginLogID)
	logid, _ := vid.(int64)
	if logid == 0 {
		logid = gid.GetGID()
	}

	return GinWithLogid(ctx, logid)
}

func GinWithLogid(ctx *gin.Context, logid int64) *zerolog.Logger {
	ctx.Set(ginLogID, logid)

	l := zerolog.New(output)
	l = l.Hook(logidHook{logid: logid})
	l = l.Hook(th) // l = l.With().Timestamp().Logger()

	ctx.Set(ginLogger, &l)
	return &l
}

type logidHook struct {
	logid int64
}

func (ch logidHook) Run(e *rszlog.Event, _ rszlog.Level, _ string) {
	e.Int64("logid", ch.logid)
}

type timestampHook struct{}

func (ts timestampHook) Run(e *rszlog.Event, _ rszlog.Level, _ string) {
	e.Timestamp()
}

var th = timestampHook{}
