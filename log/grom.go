package log

import (
	"context"
	"fmt"
	"strings"
	"time"
	"unsafe"

	"github.com/lxt1045/errors"
	uzerolog "github.com/lxt1045/errors/zerolog"
	"github.com/rs/zerolog"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"gorm.io/gorm/utils"
)

var (
	Unexpected = errors.NewCode(0, 0x01010001, "unexpected error")
)
var (
	SlowThreshold = 3 * time.Second // 慢查询边界
	gormSourceDir = "gorm.io"
)

func NewGormLogger(ctx context.Context, logger uzerolog.Logger) *GormLogger {
	return &GormLogger{
		Logger: logger,
		ctx:    ctx,
	}
}

type GormLogger struct {
	uzerolog.Logger
	ctx context.Context
}

// Info print info
func (l GormLogger) Info(ctx context.Context, msg string, data ...interface{}) {
	Ctx(ctx).Info().Msg(fmt.Sprintf(msg, data...))
}

// Warn print warn messages
func (l GormLogger) Warn(ctx context.Context, msg string, data ...interface{}) {
	Ctx(ctx).Warn().Msg(fmt.Sprintf(msg, data...))
}

// Error print error messages
func (l GormLogger) Error(ctx context.Context, msg string, data ...interface{}) {
	Ctx(ctx).Error().Msg(fmt.Sprintf(msg, data...))
}

// Trace print sql message
func (l GormLogger) Trace(ctx context.Context, begin time.Time, fc func() (string, int64), err error) {

	// get zerolog from context
	zlog := Ctx(ctx)

	// return if zerolog is disabled
	if zlog.GetLevel() == zerolog.Disabled {
		return
	}

	elapsed := time.Since(begin)
	sql, rows := fc()

	var event *uzerolog.Event
	var eventError bool
	var eventWarn bool

	// set message level
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		eventError = true
		event = zlog.Error().Err(Unexpected.Clonef("%+v", err))
	} else if SlowThreshold != 0 && elapsed > SlowThreshold {
		eventWarn = true
		event = zlog.Warn().Err(Unexpected.Clone("slow sql"))
	} else {
		event = zlog.Trace()
		if event == nil {
			return
		}
	}

	// add fields

	event = event.Time(zerolog.TimestampFieldName, begin)
	event = event.Int64("duration/ms", int64(elapsed/time.Millisecond))

	cs := errors.CallersSkip(0)
	lasti := 0
	for i, c := range cs {
		if strings.HasPrefix(c.Func, "gorm.") {
			lasti = i
		} else if strings.HasPrefix(c.Func, "gen.(*DO).") {
			lasti = i
		} else if strings.HasSuffix(c.File, ".gen.go") {
			lasti = i
		}
	}
	cs = cs[lasti+1:]
	if eventError || eventWarn {
		strs := make([]string, 0, len(cs))
		for _, c := range cs {
			strs = append(strs, c.FileLine)
		}
		event = event.Strs(
			zerolog.ErrorStackFieldName,
			strs,
		)
	} else {
		event = event.Str(
			zerolog.CallerFieldName,
			cs[0].FileLine,
		)
	}

	// add sql field
	event = event.Str("sql", sql)

	// add rows field
	if rows > -1 {
		event = event.Int64("rows", rows)
	}

	// post the message
	if eventError {
		event.Msg("SQL error")
	} else if eventWarn {
		event.Msg("SQL slow query")
	} else {
		event.Msg("SQL")
	}
}

// LogMode log mode
func (l *GormLogger) LogMode(level logger.LogLevel) logger.Interface {
	switch level {
	case logger.Silent:
		toGormEvent(l.Logger.Trace())
	case logger.Error:
		toGormEvent(l.Logger.Error())
	case logger.Warn:
		toGormEvent(l.Logger.Warn())
	case logger.Info:
		toGormEvent(l.Logger.Info())
	default:
		toGormEvent(l.Logger.Info())
	}
	return l
}

func toGormEvent(event *uzerolog.Event) *GormEvent {
	return (*GormEvent)(unsafe.Pointer(event))
}

type GormEvent struct {
	zerolog.Event
}

// Info print info
func (l GormEvent) Info(ctx context.Context, msg string, data ...interface{}) {
	l.Event.Msg(fmt.Sprintf(msg, data...))
}

// Warn print warn messages
func (l GormEvent) Warn(ctx context.Context, msg string, data ...interface{}) {
	l.Event.Msg(fmt.Sprintf(msg, data...))
}

// Error print error messages
func (l GormEvent) Error(ctx context.Context, msg string, data ...interface{}) {
	l.Event.Msg(fmt.Sprintf(msg, data...))
}

// Trace print sql message
func (l GormEvent) Trace(ctx context.Context, begin time.Time, fc func() (string, int64), err error) {

	// get zerolog from context
	event := &l.Event

	elapsed := time.Since(begin)
	sql, rows := fc()

	var eventError bool
	var eventWarn bool

	// set message level
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		eventError = true
	} else if SlowThreshold != 0 && elapsed > SlowThreshold {
		eventWarn = true
	}
	// add fields

	event = event.Time(zerolog.TimestampFieldName, begin)
	event = event.Int64("duration/ms", int64(elapsed/time.Millisecond))

	skip := 2
	cs := errors.CallersSkip(skip)
	lasti := 0
	for i, c := range cs {
		if strings.HasPrefix(c.FileLine, gormSourceDir) {
			lasti = i
		}
	}
	cs = cs[lasti+1:]
	if eventError || eventWarn {
		strs := make([]string, 0, len(cs))
		for _, c := range cs {
			strs = append(strs, c.FileLine)
		}
		event = event.Strs(
			zerolog.ErrorStackFieldName,
			strs,
		)
	} else {
		event = event.Str(
			zerolog.CallerFieldName,
			cs[0].FileLine,
		)

		// "gorm.io/gorm/utils"
		utils.FileWithLineNum()
	}

	// add sql field
	event = event.Str("sql", sql)

	// add rows field
	if rows > -1 {
		event = event.Int64("rows", rows)
	}

	// post the message
	if eventError {
		event.Msg("SQL error")
	} else if eventWarn {
		event.Msg("SQL slow query")
	} else {
		event.Msg("SQL")
	}
}
