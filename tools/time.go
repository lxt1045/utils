package tools

import (
	"time"
)

var TzBJ = time.FixedZone("beijing", 8*60*60)

func TimePairToTime(layout, v1, v2 string) (startTime, endTime time.Time, err error) {
	startTime, err = time.Parse(layout, v1)
	if err != nil {
		return
	}
	endTime, err = time.Parse(layout, v2)
	if err != nil {
		return
	}
	return
}

func TimePairToUnix(layout, v1, v2 string) (t1, t2 int64, err error) {
	startTime, endTime, err := TimePairToTime(layout, v1, v2)
	if err != nil {
		return
	}
	return startTime.Unix(), endTime.Unix(), nil
}

func TimePairToUnixNano(layout, v1, v2 string) (t1, t2 int64, err error) {
	startTime, endTime, err := TimePairToTime(layout, v1, v2)
	if err != nil {
		return
	}
	return startTime.UnixNano(), endTime.UnixNano(), nil
}

func TsToday(t time.Time, l *time.Location) (ts int64) {
	year, month, day := t.In(l).Date()
	ts = time.Date(year, month, day, 0, 0, 0, 0, TzBJ).Unix()
	return
}
