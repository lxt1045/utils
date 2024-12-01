package cache

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"
	"unsafe"

	"github.com/allegro/bigcache/v3"
	"github.com/benbjohnson/clock"
	"github.com/stretchr/testify/assert"
)

type mockedClock struct {
	*clock.Mock
}

func (mc *mockedClock) Epoch() int64 {
	return mc.Now().Unix()
}

func mockClock(bc *bigcache.BigCache, mc *mockedClock) {
	shards := reflect.ValueOf(bc).Elem().FieldByName("shards")
	for i := 0; i < shards.Len(); i++ {
		clockField := shards.Index(i).Elem().FieldByName("clock")
		clockField = reflect.NewAt(clockField.Type(), unsafe.Pointer(clockField.UnsafeAddr())).Elem()
		clockField.Set(reflect.ValueOf(mc))
	}
}

type unmarshal struct {
	Chan chan int
}

func (u unmarshal) CacheKey() string {
	return "bad json"
}

var _ Value = (*unmarshal)(nil)

func timeIt(fn func()) time.Duration {
	ts := time.Now()
	fn()
	return time.Since(ts)
}

func TestLoadingBigCache(t *testing.T) {

	var (
		bc     *bigcache.BigCache
		mc     *clock.Mock
		ctx    context.Context = context.Background()
		cancel context.CancelFunc
		err    error
	)

	const (
		TTL = 10 * time.Minute
	)

	factory := Factory(func() Value {
		return StrValue("")
	})
	load := LoaderFunc(func(ctx context.Context, key string) (Value, error) {
		v := StrValue(key)
		return v, nil
	})

	BeforeEach := func() {
		cfg := bigcache.DefaultConfig(TTL)
		cfg.CleanWindow = TTL
		bc, err = bigcache.New(ctx, cfg)
		if err != nil {
			t.Fatal(err)
		}

		mc = clock.NewMock()
		SetClock(mc)
		mockClock(bc, &mockedClock{mc})
		ctx, cancel = context.WithCancel(ctx)
	}
	AfterEach := func() {
		cancel()
	}

	t.Run("Get", func(t *testing.T) {
		BeforeEach()
		defer AfterEach()

		t.Run("Get", func(t *testing.T) {
			const sleep = time.Millisecond
			intFactory := Factory(func() Value {
				return IntValue(0)
			})

			intLoad := LoaderFunc(func(ctx context.Context, key string) (Value, error) {
				time.Sleep(sleep)
				return IntValue(1), nil
			})
			cc := NewLoading(NewBigCache(ctx, bc), intFactory, intLoad)

			t.Run("First", func(t *testing.T) {
				duration := timeIt(func() {
					a, err := cc.Get(ctx, "key")
					assert.Nil(t, err)
					assert.Equal(t, 1, ToInt(a))
				})
				assert.Greater(t, duration, sleep)
			})

			t.Run("Second", func(t *testing.T) {
				duration := timeIt(func() {
					a, err := cc.Get(ctx, "key")
					assert.Nil(t, err)
					assert.Equal(t, 1, ToInt(a))
				})
				assert.Less(t, duration, sleep)
			})

			t.Run("Refresh", func(t *testing.T) {
				mc.Add(TTL)
				duration := timeIt(func() {
					_, err := cc.Get(ctx, "key")
					assert.Nil(t, err, "expired")
				})
				assert.Less(t, duration, sleep)
			})

			t.Run("Delete", func(t *testing.T) {
				stringFactory := Factory(func() Value {
					return StrValue("")
				})
				cc := NewLoading(NewBigCache(ctx, bc), stringFactory, intLoad)
				_, err := cc.Get(ctx, "key")
				assert.NotNil(t, err)
			})
		})

		t.Run("Err", func(t *testing.T) {
			errLoad := LoaderFunc(func(ctx context.Context, key string) (Value, error) {
				return nil, errors.New("i'm a teapot")
			})
			cc := NewLoading(NewBigCache(ctx, bc), factory, errLoad)

			a, err := cc.Get(ctx, "key")
			assert.Error(t, err)
			assert.Zero(t, a)
		})
	})

	t.Run("Set", func(t *testing.T) {
		BeforeEach()
		defer AfterEach()

		var cc LoadingCache

		postLoad := func(ctx context.Context, key string, data []byte) error {
			return nil
		}
		cc = NewLoading(NewBigCache(ctx, bc), factory, load, WithPostLoad(postLoad))

		t.Run("ok", func(t *testing.T) {
			assert.Nil(t, cc.Set(ctx, "a", StrValue("a")))
		})

		t.Run("marshal error", func(t *testing.T) {
			err := cc.Set(ctx, "a", unmarshal{})
			assert.Error(t, err)
		})
	})

	t.Run("Other", func(t *testing.T) {
		cc := NewLoading(NewBigCache(ctx, bc), factory, load)
		assert.Nil(t, cc.Delete(ctx, "a", "b"))
		assert.Nil(t, cc.Close(ctx))
	})
}
