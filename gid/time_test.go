package gid

import (
	"testing"
	"time"
)

func TestTime(t *testing.T) {
	t.Run("time", func(t *testing.T) {
		tt := time.Now()
		tt.Unix()

		time.Since(tt)

		ts1 := time.Now().Unix()
		ts2 := int64(time.Since(timeMonotonic)/time.Second) + tsMonotonic
		ts3 := (RuntimeNano()-tsRuntimeNano)/int64(time.Second) + tsMonotonic
		t.Logf("time.Now():%d", ts1)
		t.Logf("time.Since():%d", ts2)
		t.Logf("time.runtimeNano():%d", ts3)
	})

	t.Run("runtimeNano", func(t *testing.T) {
		tt := time.Now()

		ts := RuntimeNano()

		t.Logf("time.Now():%d, runtimeNano:%d", tt.UnixNano(), ts)
	})

}

// 1699953810532569900
//      83558339191800
// 1699954191881142510
//      12886905775803
