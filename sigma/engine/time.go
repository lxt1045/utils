package engine

import (
	"time"
)

var fTimeNow func() time.Time

func timeNow() time.Time {
	if fTimeNow != nil {
		return fTimeNow()
	}
	return time.Now()
}
