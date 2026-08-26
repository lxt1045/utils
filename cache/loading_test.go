package cache

import (
	"context"
	"testing"
)

func Test_loadingcache(t *testing.T) {

	t.Run("*string", func(t *testing.T) {
		local, err := NewExpired[*string](t.Context(), 1024*1024)
		if err != nil {
			t.Fatal(err)
		}
		count := 0
		loadFunc := func(ctx context.Context, key string) (k *string, err error) {
			k = &key
			count++
			return
		}
		c, err := NewLoading(t.Context(), local, loadFunc)
		if err != nil {
			t.Fatal(err)
		}

		ctx := t.Context()

		v, err := c.Get(ctx, "test")
		if err != nil {
			t.Fatal(err)
		}
		t.Logf("v:%s, count:%d", *v, count)
		v, err = c.Get(ctx, "test")
		if err != nil {
			t.Fatal(err)
		}
		t.Logf("v:%s, count:%d", *v, count)
		v, err = c.Get(ctx, "test")
		if err != nil {
			t.Fatal(err)
		}
		t.Logf("v:%s, count:%d", *v, count)
	})

	t.Run("string", func(t *testing.T) {
		local, err := NewExpired[string](t.Context(), 1024*1024)
		if err != nil {
			t.Fatal(err)
		}
		count := 0
		loadFunc := func(ctx context.Context, key string) (k string, err error) {
			k = key
			count++
			return
		}
		c, err := NewLoading(t.Context(), local, loadFunc)
		if err != nil {
			t.Fatal(err)
		}

		ctx := t.Context()

		v, err := c.Get(ctx, "test")
		if err != nil {
			t.Fatal(err)
		}
		t.Logf("v:%s, count:%d", v, count)
		v, err = c.Get(ctx, "test")
		if err != nil {
			t.Fatal(err)
		}
		t.Logf("v:%s, count:%d", v, count)
		v, err = c.Get(ctx, "test")
		if err != nil {
			t.Fatal(err)
		}
		t.Logf("v:%s, count:%d", v, count)
	})
}
