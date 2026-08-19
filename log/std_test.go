package log

import (
	"context"
	"testing"

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
func Test_Debug(t *testing.T) {
	Debug("x", "y", 1, 2)
}

func Test_getAllGIDs(t *testing.T) {
	t.Logf("getAllGIDsr: %+v", getAllGIDs())
}

func Test_clearStdLogger(t *testing.T) {
	SetStdLogger(Ctx(context.Background()))

	g := new(errgroup.Group)
	for i := 0; i < 100; i++ {
		g.Go(func() error {
			SetStdLogger(Ctx(context.Background()))
			return nil
		})
	}
	err := g.Wait()
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("before clearStdLogger: %+v", stdLoggers.Keys())
	clearStdLogger()
	t.Logf("after clearStdLogger: %+v", stdLoggers.Keys())
}
