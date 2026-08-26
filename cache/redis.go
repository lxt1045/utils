package cache

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"time"

	// "github.com/go-redis/redis/v9"
	"github.com/lxt1045/errors"
	"github.com/redis/go-redis/v9"
)

var _ Cache[bool] = &redisCache[bool]{}

type RedisCacheOption[T Value] func(context.Context, *redisCache[T])

type RedisClient interface {
	Get(key string) *redis.StringCmd
	Set(key string, value interface{}, expiration time.Duration) *redis.StatusCmd
	Del(keys ...string) *redis.IntCmd
	Close() error
}

type redisCache[T Value] struct {
	RedisClient

	keyPrefix  string
	expiration time.Duration
	refresh    time.Duration
}

func WithKeyPrefix[T Value](prefix string) RedisCacheOption[T] {
	return func(ctx context.Context, cache *redisCache[T]) {
		cache.keyPrefix = prefix
	}
}

func WithExpiration[T Value](expiration time.Duration, refresh ...time.Duration) RedisCacheOption[T] {
	return func(ctx context.Context, cache *redisCache[T]) {
		cache.expiration = expiration
		if len(refresh) > 0 {
			cache.refresh = refresh[0]
		} else {
			cache.refresh = expiration / 2
		}
	}
}

func NewRedis[T Value](ctx context.Context, client RedisClient, opts ...RedisCacheOption[T]) Cache[T] {
	c := &redisCache[T]{
		RedisClient: client,
		keyPrefix:   "",
		expiration:  time.Minute * 15,
		refresh:     time.Minute * 5,
	}
	for _, opt := range opts {
		opt(ctx, c)
	}
	return c
}

func makeKey(key, prefix string) string {
	return prefix + key
}

func (r *redisCache[T]) GetWithInfo(key string) (d T, expired bool, err error) {
	key = makeKey(key, r.keyPrefix)

	bs, err := r.Get(key).Bytes()
	if err != nil {
		err = errors.WithErr(err)
		return
	}
	if l := len(bs); l >= 9 && bs[l-1] == 0 {
		sec := binary.LittleEndian.Uint64(bs[l-9 : l-1])
		at := time.Unix(int64(sec), 0)
		expired = time.Now().After(at)
		bs = bs[:l-9]
	}
	err = json.Unmarshal(bs, &d)
	if err != nil {
		err = errors.WithErr(err)
		return
	}
	return
}

func (r *redisCache[T]) Set(key string, d T) error {
	bs, err := json.Marshal(&d)
	if err != nil {
		return errors.WithErr(err)
	}
	l := len(bs)
	bs = append(bs, 0, 0, 0, 0, 0, 0, 0, 0, 0) // 9 extra bytes; 存刷新时间，避免单飞等待
	ts := time.Now().Add(r.refresh).Unix()
	binary.LittleEndian.PutUint64(bs[l:l+8], uint64(ts))

	key = makeKey(key, r.keyPrefix)
	return r.RedisClient.Set(key, bs, r.expiration).Err()
}

func (r *redisCache[T]) Del(ks ...string) error {
	if len(r.keyPrefix) > 0 {
		nKeys := make([]string, len(ks))
		for i, k := range ks {
			nKeys[i] = makeKey(k, r.keyPrefix)
		}
		ks = nKeys
	}
	return r.RedisClient.Del(ks...).Err()
}
