package cache

import (
	"context"
)

type noOp struct {
	load LoaderFunc
}

func NewNoOp(load LoaderFunc) LoadingCache {
	return noOp{load: load}
}

func (n noOp) Get(ctx context.Context, key string) (Value, error) {
	return n.Load(ctx, key)
}

func (n noOp) Load(ctx context.Context, key string) (Value, error) {
	return n.load(ctx, key)
}

func (n noOp) BatchGet(ctx context.Context, keys []string) (map[string]Value, error) {
	return n.BatchLoad(ctx, keys)
}

func (n noOp) BatchLoad(ctx context.Context, keys []string) (map[string]Value, error) {
	results := make(map[string]Value, len(keys))
	for _, key := range keys {
		value, err := n.Get(ctx, key)
		if err != nil {
			return nil, err
		}
		results[key] = value
	}
	return results, nil
}

func (n noOp) Set(ctx context.Context, key string, value Value) error {
	return nil
}

func (n noOp) SetByData(ctx context.Context, key string, data []byte) error {
	return nil
}

func (n noOp) Delete(ctx context.Context, keys ...string) error {
	return nil
}

func (n noOp) Close(ctx context.Context) error {
	return nil
}
