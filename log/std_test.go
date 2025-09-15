package log

import (
	"context"
	"expvar"
	"fmt"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"golang.org/x/sync/errgroup"
)

func BenchmarkGids(b *testing.B) {
	b.Run("zerolog", func(b *testing.B) {
		b.StopTimer()
		b.ReportAllocs()
		b.StartTimer()
		for i := 0; i < b.N; i++ {
			getAllGIDs()
		}
	})
}

func Test_clearStdLogger(t *testing.T) {
	g := new(errgroup.Group)
	for i := 0; i < 100; i++ {
		g.Go(func() error {
			SetStdLogger(context.Background())
			return nil
		})
	}
	err := g.Wait()
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("before clearStdLogger: %+v", gidLoggers.Items())
	clearStdLogger()
	t.Logf("after clearStdLogger: %+v", gidLoggers.Items())
}

func TestGids(t *testing.T) {
	t.Run("ee", func(t *testing.T) {
		ss := getAllGIDs()
		t.Logf("%+v", ss)

	})
	Init11()
	time.Sleep(time.Second * 30)
}

func Init11() {
	expvar.Publish("goroutines", expvar.Func(func() interface{} {
		buf := make([]byte, 1<<16)
		n := runtime.Stack(buf, true)
		return parseGoroutines(buf[:n])
	}))
}

func parseGoroutines(stack []byte) []uint64 {
	var gids []uint64
	lines := strings.Split(string(stack), "\n")

	for i := 0; i < len(lines); i++ {
		if strings.HasPrefix(lines[i], "goroutine ") {
			parts := strings.Fields(lines[i])
			if len(parts) < 2 {
				continue
			}
			id, err := strconv.ParseUint(parts[1], 10, 64)
			if err == nil {
				gids = append(gids, id)
			}
		}
	}
	fmt.Printf("%+v\n", gids)
	return gids

}
