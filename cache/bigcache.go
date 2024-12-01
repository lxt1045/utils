package cache

import (
	"context"
	"time"

	"github.com/allegro/bigcache/v3"
	"github.com/cespare/xxhash/v2"
	"github.com/lxt1045/errors"
	"github.com/lxt1045/errors/zerolog"
)

type xxHash struct{}

func (*xxHash) Sum64(s string) uint64 {
	return xxhash.Sum64String(s)
}

func BigCache(ctx context.Context, config bigcache.Config) (*bigcache.BigCache, error) { //nolint:gocritic
	if config.LifeWindow == 0 {
		config.LifeWindow = time.Minute * 15
	}
	if config.CleanWindow == 0 {
		config.CleanWindow = config.LifeWindow * 2
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
			zerolog.Ctx(ctx).Info().Caller().Msgf("key %s (request count: %d) is removed due to no space.", key, keyMetadata.RequestCount)
		}
	}

	bc, err := bigcache.New(ctx, config)
	if err != nil {
		err = errors.NewCode(0, 0x05010001, err.Error())
		zerolog.Ctx(ctx).Error().Caller().Err(err).Msgf("fail to init bigcache: %v", config)
		return nil, err
	}
	return bc, nil
}

type BigCacheOption func(context.Context, *bigCache)

type bigCache struct {
	*bigcache.BigCache
}

var _ Cache = (*bigCache)(nil)

func WithMetrics(logger *zerolog.Logger, name string) BigCacheOption {
	return func(ctx context.Context, c *bigCache) {
		go c.EmitMetrics(ctx, logger, name)
	}
}

func NewBigCache(ctx context.Context, cache *bigcache.BigCache, opts ...BigCacheOption) Cache {
	bc := &bigCache{
		BigCache: cache,
	}
	for _, opt := range opts {
		opt(ctx, bc)
	}
	return bc
}

func (bc *bigCache) GetWithInfo(key string) (data []byte, expired bool, err error) {
	data, info, err := bc.BigCache.GetWithInfo(key)
	if err == nil {
		expired = info.EntryStatus == bigcache.Expired
	}
	return
}

func (bc *bigCache) Delete(keys ...string) error {
	for _, key := range keys {
		_ = bc.BigCache.Delete(key)
	}
	return nil
}

func (bc *bigCache) EmitMetrics(ctx context.Context, logger *zerolog.Logger, name string) {
	ticker := sysClock.Ticker(time.Minute)
	for {
		select {
		case <-ctx.Done():
			ticker.Stop()
			return
		case <-ticker.C:
			stat := bc.Stats()
			logger.Info().Int("cache.hits", int(stat.Hits)).
				Int("cache.hits", int(stat.Hits)).
				Int("cache.misses", int(stat.Misses)).
				Int("cache.delHits", int(stat.DelHits)).
				Int("cache.delMisses", int(stat.DelMisses)).
				Int("cache.collisions", int(stat.Collisions)).
				Msg(name)
		}
	}
}
