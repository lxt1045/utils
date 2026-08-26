package cache

import (
	"bytes"
	"context"
	"encoding/gob"
	"log/slog"
	"time"

	"github.com/VictoriaMetrics/fastcache"
	"github.com/lxt1045/errors"
)

type Value interface {
}

type cache[T Value] struct {
	cache      *fastcache.Cache
	storeFile  string
	lifeWindow int64
}

type data[T Value] struct {
	d       T
	expired int64 // 超时时间
}

// maxBytes capacity in bytes.
func New[T Value](ctx context.Context, maxBytes int, opts ...Option[T]) (c *cache[T], err error) {
	c, _, err = newCache(ctx, maxBytes, opts...)
	return
}
func newCache[T Value](ctx context.Context, maxBytes int, opts ...Option[T]) (c *cache[T], conf config[T], err error) {
	for _, opt := range opts {
		opt(&conf)
	}

	if conf.LifeWindow != 0 || conf.CleanWindow != 0 {
		err = errors.Errorf("default cache not support LifeWindow or CleanWindow")
		return
	}
	if conf.BatchLoader != nil || conf.PostLoad != nil {
		err = errors.Errorf("default cache not support BatchLoader or PostLoad")
		return
	}

	c = &cache[T]{}

	if conf.StoreFile != "" {
		c.cache = fastcache.LoadFromFileOrNew(conf.StoreFile, maxBytes)
		c.storeFile = conf.StoreFile
	} else {
		c.cache = fastcache.New(maxBytes)
	}

	if conf.MetricLogger != nil {
		go c.EmitMetrics(ctx, conf.Name, time.Duration(conf.MetricSec), conf.MetricLogger)
	}
	return
}

// Store 将缓存序列化到文件中
func (c *cache[T]) Store() (err error) {
	if c.storeFile != "" {
		c.cache.SaveToFile(c.storeFile)
	}
	return
}

func (c *cache[T]) Set(k string, v *T) (err error) {
	var buffer bytes.Buffer
	err = gob.NewEncoder(&buffer).Encode(v)
	if err != nil {
		err = errors.WithErr(err)
		return
	}
	c.cache.Set(toBs(k), buffer.Bytes())
	return
}

func (c *cache[T]) Get(k string) (d *T, err error) {
	bs, ok := c.cache.HasGet(nil, toBs(k))
	if !ok {
		return
	}
	d = new(T)
	if len(bs) == 0 {
		return
	}
	err = gob.NewDecoder(bytes.NewReader(bs)).Decode(d)
	if err != nil {
		err = errors.WithErr(err)
		return
	}
	return
}

func (c *cache[T]) Has(k string) bool {
	return c.cache.Has(toBs(k))
}

func (c *cache[T]) Del(k string) {
	c.cache.Del(toBs(k))
}

func (c *cache[T]) EmitMetrics(ctx context.Context, name string, sec time.Duration, logger func(attrs ...slog.Attr)) {
	ticker := time.NewTicker(time.Second * sec)
	for {
		select {
		case <-ctx.Done():
			ticker.Stop()
			return
		case <-ticker.C:
			stat := fastcache.Stats{}
			c.cache.UpdateStats(&stat)
			logger(
				slog.String("name", name),
				slog.Uint64("get", stat.GetCalls),
				slog.Uint64("set", stat.SetCalls),
				slog.Uint64("misses", stat.Misses),
				slog.Uint64("collisions", stat.Collisions),
				slog.Uint64("corruptions", stat.Corruptions),
				slog.Uint64("entries_count", stat.EntriesCount),
				slog.Uint64("bytes_size", stat.BytesSize),
				slog.Uint64("max_bytes_size", stat.MaxBytesSize),
				slog.Uint64("get_big", stat.BigStats.GetBigCalls),
				slog.Uint64("set_big", stat.BigStats.SetBigCalls),
				slog.Uint64("too_big", stat.BigStats.TooBigKeyErrors),
				slog.Uint64("invalid_big", stat.BigStats.InvalidMetavalueErrors),
				slog.Uint64("invalid_big_hash", stat.BigStats.InvalidValueHashErrors),
			)
		}
	}
}
