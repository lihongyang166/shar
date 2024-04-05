package cache

import (
	"fmt"
	"github.com/dgraph-io/ristretto"
)

//go:generate mockery
type CacheBackend interface {
	Get(key interface{}) (interface{}, bool)
	Set(key interface{}, value interface{}) bool
}

type RistrettoCacheBackend struct {
	c *ristretto.Cache
}

func (rcb *RistrettoCacheBackend) Get(key interface{}) (interface{}, bool) {
	return rcb.c.Get(key)
}

func (rcb *RistrettoCacheBackend) Set(key interface{}, value interface{}) bool {
	// TODO what do we want to do about cost...seems to be a Ristretto specific property
	// need to understand this further...
	return rcb.c.Set(key, value, 1)
}

func NewRistrettoCacheBackend() *RistrettoCacheBackend {
	cache, _ := ristretto.NewCache(
		&ristretto.Config{
			NumCounters: 1e7,
			MaxCost:     1 << 30,
			BufferItems: 64,
		})
	return &RistrettoCacheBackend{c: cache}
}

type SharCache struct {
	cacheBackend CacheBackend
}

func NewSharCache(backend CacheBackend) *SharCache {
	return &SharCache{
		cacheBackend: backend,
	}
}

func (c *SharCache) Cacheable(key interface{}, fn func() (interface{}, error)) (interface{}, error) {
	val, cacheHit := c.cacheBackend.Get(key)

	if !cacheHit {
		retrievedVal, err := fn()
		if err != nil {
			return nil, fmt.Errorf("error retrieving value for key %s: %w", key, err)
		}
		val = retrievedVal
		c.cacheBackend.Set(key, retrievedVal)
	}

	return val, nil
}
