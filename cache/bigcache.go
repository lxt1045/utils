package cache

import (
	"bytes"
	"context"
	"encoding/gob"
	"log/slog"
	"time"

	"github.com/allegro/bigcache/v3"
	"github.com/cespare/xxhash/v2"
	"github.com/lxt1045/errors"
	"github.com/lxt1045/errors/zerolog"
)

var _ Cache[bool] = &cache[bool]{}

type cache[T Value] struct {
	bcache *bigcache.BigCache

	name   string
	logger func(attrs ...slog.Attr)
}

type xxHash struct{}

func (*xxHash) Sum64(s string) uint64 {
	return xxhash.Sum64String(s)
}

// maxBytes 单位 MByte; refleshSec, cleanSec 单位 秒
func New[T Value](ctx context.Context, refleshSec, cleanSec, maxBytes int, opts ...Option[T]) (c *cache[T], err error) {
	c, _, err = newCache(ctx, refleshSec, cleanSec, maxBytes, opts...)
	return
}

func newCache[T Value](ctx context.Context, refleshSec, cleanSec, maxBytes int, opts ...Option[T]) (c *cache[T], conf config[T], err error) {
	for _, opt := range opts {
		opt(&conf)
	}
	if conf.BatchLoader != nil || conf.PostLoad != nil {
		err = errors.Errorf("expired cache not support BatchLoader or PostLoad")
		return
	}
	bconf := bigcache.Config{
		LifeWindow:       time.Second * time.Duration(refleshSec), // 报超时
		CleanWindow:      time.Second * time.Duration(cleanSec),   // 删除数据
		HardMaxCacheSize: maxBytes,
	}
	bc, err := newBigCache(ctx, bconf, conf.Name)
	if err != nil {
		err = errors.Errorf("expired cache not support BatchLoader or PostLoad")
		return
	}
	c = &cache[T]{
		bcache: bc,
	}

	if conf.MetricLogger != nil {
		go c.EmitMetrics(ctx, conf.Name, time.Duration(conf.MetricSec), conf.MetricLogger)
	}
	return
}

func newBigCache(ctx context.Context, config bigcache.Config, name string) (*bigcache.BigCache, error) { //nolint:gocritic
	if config.LifeWindow == 0 {
		config.LifeWindow = time.Minute * 15 // 刷新数据
	}
	if config.CleanWindow == 0 {
		config.CleanWindow = config.LifeWindow * 2 // 删除数据
	}
	if config.Shards == 0 {
		config.Shards = 1024
	}
	if config.MaxEntriesInWindow == 0 {
		config.MaxEntriesInWindow = 1000 * 10 * 60
	}
	if config.MaxEntrySize == 0 {
		config.MaxEntrySize = 500
	}
	if config.HardMaxCacheSize == 0 {
		config.HardMaxCacheSize = 1024 // MB
	}
	if config.Hasher == nil {
		config.Hasher = &xxHash{}
	}
	if config.Logger == nil {
		config.Logger = zerolog.Ctx(ctx)
	}
	if config.OnRemove == nil && config.OnRemoveWithReason == nil && config.OnRemoveWithMetadata == nil {
		config.OnRemoveFilterSet(bigcache.NoSpace)
		config.OnRemoveWithMetadata = func(key string, entry []byte, keyMetadata bigcache.Metadata) {
			config.Logger.Printf("name: %s, key %s (request count: %d) is removed due to no space.", name, key, keyMetadata.RequestCount)
		}
	}

	config.StatsEnabled = true

	bc, err := bigcache.New(ctx, config)
	if err != nil {
		err = errors.WithErr(err)
		return nil, err
	}
	return bc, nil
}

func (c *cache[T]) Set(k string, v T) (err error) {
	var buffer bytes.Buffer
	err = gob.NewEncoder(&buffer).Encode(v)
	if err != nil {
		err = errors.WithErr(err)
		return
	}
	c.bcache.Set(k, buffer.Bytes())
	return
}

func (c *cache[T]) Get(k string) (d T, err error) {
	d, expired, err := c.GetWithInfo(k)
	if expired && err == nil {
		err = NotExist
	}
	return
}
func (c *cache[T]) GetWithInfo(k string) (d T, expired bool, err error) {
	bs, res, err := c.bcache.GetWithInfo(k)
	if err != nil {
		err = errors.WithErr(err)
		return
	}
	if res.EntryStatus == bigcache.Expired {
		expired = true
	}
	if len(bs) == 0 {
		err = NotExist
		return
	}
	err = gob.NewDecoder(bytes.NewReader(bs)).Decode(&d)
	if err != nil {
		c.bcache.Delete(k)
		err = errors.WithErr(err)
		return
	}
	return
}

func (c *cache[T]) Del(ks ...string) (err error) {
	for _, k := range ks {
		err = c.bcache.Delete(k)
		if err != nil {
			err = errors.WithErr(err)
			return
		}
	}
	return
}

func (c *cache[T]) Close() (err error) {
	return c.bcache.Close()
}

func (c *cache[T]) EmitMetrics(ctx context.Context, name string, sec time.Duration, logger func(attrs ...slog.Attr)) {
	ticker := time.NewTicker(time.Second * sec)
	for {
		select {
		case <-ctx.Done():
			ticker.Stop()
			return
		case <-ticker.C:
			stat := c.bcache.Stats()
			logger(
				slog.String("name", name),
				slog.Int64("hits", stat.Hits),
				slog.Int64("misses", stat.Misses),
				slog.Int64("delete_hits", stat.DelHits),
				slog.Int64("delete_misses", stat.DelMisses),
				slog.Int64("collisions", stat.Collisions),
			)
		}
	}
}
