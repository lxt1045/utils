package log

import (
	"context"
	"time"

	"github.com/lxt1045/errors"
	"github.com/lxt1045/errors/zerolog"
)

var loc = time.FixedZone("UTC-8", 8*60*60)
var warnTimeout int64 = 3000 // 单位ms

// formatDate 自己实现的format函数比time.Format()性能要好
func FormatTime(t time.Time, buf []byte) []byte {
	t = t.In(loc)
	year, month, day := t.Date()
	hour, minute, second := t.Clock()
	ns := t.Nanosecond()

	if len(buf) < 35 {
		if cap(buf) >= 35 {
			buf = buf[:35]
		} else {
			buf = make([]byte, 35)
		}
	}

	buf[0] = byte((year/1000)%10) + '0'
	buf[1] = byte((year/100)%10) + '0'
	buf[2] = byte((year/10)%10) + '0'
	buf[3] = byte(year%10) + '0'
	buf[4] = '-'
	buf[5] = byte((month)/10) + '0'
	buf[6] = byte((month)%10) + '0'
	buf[7] = '-'
	buf[8] = byte((day)/10) + '0'
	buf[9] = byte((day)%10) + '0'
	buf[10] = 'T' // ' '
	buf[11] = byte((hour)/10) + '0'
	buf[12] = byte((hour)%10) + '0'
	buf[13] = ':'
	buf[14] = byte((minute)/10) + '0'
	buf[15] = byte((minute)%10) + '0'
	buf[16] = ':'
	buf[17] = byte((second)/10) + '0'
	buf[18] = byte((second)%10) + '0'
	buf[19] = '.'
	buf[20] = byte((ns/100000000)%10) + '0'
	buf[21] = byte((ns/10000000)%10) + '0'
	buf[22] = byte((ns/1000000)%10) + '0'
	buf[23] = byte((ns/100000)%10) + '0'
	buf[24] = byte((ns/10000)%10) + '0'
	buf[25] = byte((ns/1000)%10) + '0'
	buf[26] = byte((ns/100)%10) + '0'
	buf[27] = byte((ns/10)%10) + '0'
	buf[28] = byte((ns)%10) + '0'
	buf[29] = '+' // 'Z'
	buf[30] = '0'
	buf[31] = '8'
	buf[32] = ':'
	buf[33] = '0'
	buf[34] = '0'

	return buf
}

func DeferLogger(ctx context.Context, loss int64, err error, recove interface{}) *zerolog.Event {
	l := Ctx(ctx).Trace()
	loss = loss / int64(time.Millisecond)
	if loss >= warnTimeout {
		l = Ctx(ctx).Warn()
	}
	if err != nil {
		l = Ctx(ctx).Error().Array("stack", errors.ZerologStack(5))
		l = l.Err(err)
	}
	if recove != nil {
		l = Ctx(ctx).Error().Array("stack", errors.ZerologStack(5))
		if err, ok := recove.(error); ok {
			l = l.Err(err)
		} else {
			l = l.Interface("recover", recove)
		}
	}
	l = l.Int64("duration/ms", loss)
	return l
}
