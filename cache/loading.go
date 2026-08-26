package cache

import (
	"bytes"
	"context"
	"encoding/gob"
	"sync"

	"github.com/lxt1045/errors"
	"golang.org/x/sync/singleflight"
)

// LoaderFunc 加载 key 对应的 value，value **必须** 是指针类型
type LoaderFunc[T Value] func(ctx context.Context, key string) (T, error)

// BatchLoadFunc 加载多个 key 对应的 value，返回值字典里的 value **必须** 是指针类型
type BatchLoadFunc[T Value] func(ctx context.Context, keys []string) ([]T, error)

// PostLoad 调用 LoaderFunc 获取值的时候
type PostLoadFunc[T Value] func(ctx context.Context, key string, d T) error

type loading[T Value] struct {
	cache Cache[T]

	load      LoaderFunc[T]
	batchLoad BatchLoadFunc[T]
	postLoad  PostLoadFunc[T]
	flight    singleflight.Group
	expires   sync.Map
}

// maxBytes capacity in bytes.
func NewLoading[T Value](ctx context.Context, cache Cache[T], loadFunc LoaderFunc[T], opts ...Option[T]) (c *loading[T], err error) {
	conf := config[T]{}
	for _, opt := range opts {
		opt(&conf)
	}
	if loadFunc == nil {
		err = errors.New("loadFunc is nil")
		return
	}
	c = &loading[T]{
		cache:     cache,
		load:      loadFunc,
		batchLoad: conf.BatchLoader,
		postLoad:  conf.PostLoad,
	}
	return
}

func (c *loading[T]) Get(ctx context.Context, k string) (d T, err error) {
	d, expired, err := c.cache.GetWithInfo(k)
	if err == nil {
		if expired {
			// XXX: singleflight doesn't offer inflight check api, use extra map instead.
			if _, loaded := c.expires.LoadOrStore(k, true); !loaded {
				go func() {
					_, _ = c.Load(ctx, k)
					c.expires.Delete(k)
				}()
			}
		}
		return
	}
	return c.Load(ctx, k)
}

func (c *loading[T]) Load(ctx context.Context, k string) (d T, err error) {
	defer c.flight.Forget(k)
	v, err, _ := c.flight.Do(k, func() (interface{}, error) {
		return c.load(ctx, k)
	})
	if err != nil {
		err = errors.WithErr(err)
		return
	}
	vv := v.(T)
	return vv, c.Set(ctx, k, vv)
}

func (c *loading[T]) Set(ctx context.Context, k string, v T) (err error) {
	var buffer bytes.Buffer
	err = gob.NewEncoder(&buffer).Encode(v)
	if err != nil {
		return errors.WithErr(err)
	}
	if c.postLoad != nil {
		_ = c.postLoad(ctx, k, v)
	}
	return c.cache.Set(k, v)
}

func (c *loading[T]) BatchLoad(ctx context.Context, ks []string) (vs []T, err error) {
	if c.batchLoad == nil {
		m := make(map[string]T, len(ks))
		for _, key := range ks {
			value, err := c.Load(ctx, key)
			if err != nil {
				return nil, err
			}
			m[key] = value
		}
		vs = make([]T, 0, len(m))
		for _, k := range ks {
			vs = append(vs, m[k])
		}
		return vs, nil
	}
	results, err := c.batchLoad(ctx, ks)
	if len(ks) < len(results) {
		results = results[:len(ks)]
	}
	for i, v := range results {
		_ = c.Set(ctx, ks[i], v)
	}
	return results, err
}

func (c *loading[T]) BatchGet(ctx context.Context, ks []string) (vs []T, err error) {
	vs = make([]T, len(ks))
	if c.batchLoad == nil {
		for _, key := range ks {
			v, err := c.Get(ctx, key)
			if err != nil {
				return nil, errors.WithErr(err)
			}
			vs = append(vs, v)
		}
		return
	}

	var errs []error

	missingIdx := make([]int, 0, len(ks))
	missing := make([]string, 0, len(ks))
	expires := make([]string, 0, len(ks))
	for i, k := range ks {
		d, expired, err1 := c.cache.GetWithInfo(k)
		if err1 != nil {
			missingIdx = append(missingIdx, i)
			missing = append(missing, k)
			errs = append(errs, err1)
			continue
		}
		if expired {
			expires = append(expires, k)
		}
		vs[i] = d
	}

	if len(missing) > 0 {
		loads, err := c.batchLoad(ctx, missing)
		if err != nil {
			return nil, err
		}
		for i, v := range loads {
			_ = c.Set(ctx, missing[i], v)
			vs[missingIdx[i]] = v
		}
	}

	if len(expires) > 0 {
		go func() {
			loads, err := c.batchLoad(ctx, expires)
			if err != nil {
				return
			}
			for i, v := range loads {
				_ = c.Set(ctx, expires[i], v)
			}
		}()
	}
	return
}
