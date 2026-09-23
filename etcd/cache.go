package etcd

import (
	"strings"
	"sync"
)

type cache[T any] struct {
	m map[string]T
	l sync.RWMutex
}

func NewCache[T any]() *cache[T] {
	return &cache[T]{
		m: make(map[string]T),
	}
}

func (c *cache[T]) Add(key string, value T) {
	c.l.Lock()
	defer c.l.Unlock()
	c.m[key] = value
}
func (c *cache[T]) Del(key string) {
	c.l.Lock()
	defer c.l.Unlock()
	delete(c.m, key)
}

func (c *cache[T]) GetBykey(key string) (value T) {
	c.l.RLock()
	defer c.l.RUnlock()
	return c.m[key]
}

func (c *cache[T]) GetPre(keyPre string) (keys []string) {
	c.l.RLock()
	defer c.l.RUnlock()
	for k := range c.m {
		if strings.HasPrefix(k, keyPre) {
			keys = append(keys, k)
		}
	}
	return
}
