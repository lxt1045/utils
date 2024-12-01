package cache

import (
	"context"
)

// Value 为缓存对象，可反向生成缓存 Key
type Value interface {
	CacheKey() string
}

// Factory 生成加载缓存内容的对象，**必须** 是指针类型
type Factory func() Value

// LoaderFunc 加载 key 对应的 value，value **必须** 是指针类型
type LoaderFunc func(ctx context.Context, key string) (Value, error)

// BatchLoaderFunc 加载多个 key 对应的 value，返回值字典里的 value **必须** 是指针类型
type BatchLoaderFunc func(ctx context.Context, keys []string) (map[string]Value, error)

// PostLoad 调用 LoaderFunc 获取值的时候
type PostLoad func(ctx context.Context, key string, data []byte) error

type Cache interface {
	GetWithInfo(key string) (data []byte, expired bool, err error)
	Set(key string, value []byte) error
	Close() error
	Delete(keys ...string) error
}

type LoadingCache interface {
	Get(ctx context.Context, key string) (Value, error)
	BatchGet(ctx context.Context, keys []string) (map[string]Value, error)
	Load(ctx context.Context, key string) (Value, error)
	BatchLoad(ctx context.Context, keys []string) (map[string]Value, error)
	Set(ctx context.Context, key string, value Value) error
	SetByData(ctx context.Context, key string, data []byte) error
	Delete(ctx context.Context, keys ...string) error
	Close(ctx context.Context) error
}
