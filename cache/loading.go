package cache

import (
	"context"
	"encoding/json"
	"sync"

	"golang.org/x/sync/singleflight"
)

type loadingCache struct {
	factory   Factory
	load      LoaderFunc
	batchLoad BatchLoaderFunc
	postLoad  PostLoad
	cache     Cache

	marshal   marshalFunc
	unmarshal unmarshalFunc
	flight    singleflight.Group
	expires   sync.Map
}

type LoadingCacheOption func(*loadingCache)
type marshalFunc func(interface{}) ([]byte, error)
type unmarshalFunc func([]byte, interface{}) error

func WithBatchLoader(batchLoad BatchLoaderFunc) LoadingCacheOption {
	return func(c *loadingCache) {
		c.batchLoad = batchLoad
	}
}

func WithPostLoad(postLoad PostLoad) LoadingCacheOption {
	return func(c *loadingCache) {
		c.postLoad = postLoad
	}
}

func WithMarshalUnmarshal(marshal marshalFunc, unmarshal unmarshalFunc) LoadingCacheOption {
	return func(c *loadingCache) {
		c.marshal = marshal
		c.unmarshal = unmarshal
	}
}

func NewLoading(cache Cache, factory Factory, load LoaderFunc, opts ...LoadingCacheOption) LoadingCache {
	if cache == nil {
		panic("big cache is nil")
	}
	if factory == nil {
		panic("factory func is nil")
	}
	if load == nil {
		panic("loader func is nil")
	}
	bc := &loadingCache{
		factory:   factory,
		load:      load,
		cache:     cache,
		marshal:   json.Marshal,
		unmarshal: json.Unmarshal,
	}
	for _, opt := range opts {
		opt(bc)
	}
	return bc
}

func (c *loadingCache) Get(ctx context.Context, key string) (Value, error) {
	data, expired, err := c.cache.GetWithInfo(key)
	if err == nil {
		if expired {
			// XXX: singleflight doesn't offer inflight check api, use extra map instead.
			if _, loaded := c.expires.LoadOrStore(key, true); !loaded {
				go func() {
					_, _ = c.Load(ctx, key)
					c.expires.Delete(key)
				}()
			}
		}
		value := c.factory()
		err = c.unmarshal(data, value)
		if err != nil {
			_ = c.cache.Delete(key)
		}
		return value, err
	}
	return c.Load(ctx, key)
}

func (c *loadingCache) Load(ctx context.Context, key string) (Value, error) {
	defer c.flight.Forget(key)
	v, err, _ := c.flight.Do(key, func() (interface{}, error) {
		return c.load(ctx, key)
	})
	if err != nil {
		return nil, err
	}
	vv := v.(Value)
	return vv, c.Set(ctx, key, vv)
}

func (c *loadingCache) Set(ctx context.Context, key string, value Value) error {
	data, err := c.marshal(value)
	if err != nil {
		return err
	}
	if c.postLoad != nil {
		_ = c.postLoad(ctx, key, data)
	}
	return c.cache.Set(key, data)
}

func (c *loadingCache) SetByData(ctx context.Context, key string, data []byte) error {
	return c.cache.Set(key, data)
}

func (c *loadingCache) BatchLoad(ctx context.Context, keys []string) (map[string]Value, error) {
	if c.batchLoad == nil {
		results := make(map[string]Value, len(keys))
		for _, key := range keys {
			value, err := c.Load(ctx, key)
			if err != nil {
				return nil, err
			}
			results[key] = value
		}
		return results, nil
	}
	results, err := c.batchLoad(ctx, keys)
	for k, v := range results {
		_ = c.Set(ctx, k, v)
	}
	return results, err
}

func (c *loadingCache) BatchGet(ctx context.Context, keys []string) (map[string]Value, error) {
	results := make(map[string]Value, len(keys))
	if c.batchLoad == nil {
		for _, key := range keys {
			value, err := c.Get(ctx, key)
			if err != nil {
				return nil, err
			}
			results[key] = value
		}
		return results, nil
	}

	missing := make([]string, 0, len(keys))
	expires := make([]string, 0, len(keys))
	for _, key := range keys {
		data, expired, err := c.cache.GetWithInfo(key)
		if err != nil {
			missing = append(missing, key)
			continue
		}
		if expired {
			expires = append(expires, key)
		}
		value := c.factory()
		err = c.unmarshal(data, value)
		if err != nil {
			if err != nil {
				_ = c.cache.Delete(key)
			}
			return nil, err
		}
		results[key] = value
	}

	if len(missing) > 0 {
		loads, err := c.batchLoad(ctx, missing)
		if err != nil {
			return nil, err
		}
		for k, v := range loads {
			_ = c.Set(ctx, k, v)
			results[k] = v
		}
	}

	if len(expires) > 0 {
		go func() {
			loads, err := c.batchLoad(ctx, expires)
			if err != nil {
				return
			}
			for k, v := range loads {
				_ = c.Set(ctx, k, v)
			}
		}()
	}
	return results, nil
}

func (c *loadingCache) Delete(ctx context.Context, keys ...string) error {
	return c.cache.Delete(keys...)
}

func (c *loadingCache) Close(ctx context.Context) error {
	return c.cache.Close()
}
