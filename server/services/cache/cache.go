package cache

import (
	"fmt"
	"github.com/dgraph-io/ristretto"
)

// CacheBackend defines interface for a CacheBackend
//
//go:generate mockery
type CacheBackend interface {
	Get(key interface{}) (interface{}, bool)
	Set(key interface{}, value interface{}) bool
}

// RistrettoCacheBackend is a RistrettoCache implemenentation of CacheBackend
type RistrettoCacheBackend struct {
	c *ristretto.Cache
}

// Get a value from the cache
func (rcb *RistrettoCacheBackend) Get(key interface{}) (interface{}, bool) {
	return rcb.c.Get(key)
}

// Set a value in the cache
func (rcb *RistrettoCacheBackend) Set(key interface{}, value interface{}) bool {
	// TODO what do we want to do about cost...seems to be a Ristretto specific property
	// need to understand this further...
	return rcb.c.Set(key, value, 1)
}

// NewRistrettoCacheBackend construct an instance of a RistrettoCacheBackend
func NewRistrettoCacheBackend() (*RistrettoCacheBackend, error) {
	cache, err := ristretto.NewCache(
		&ristretto.Config{
			NumCounters: 1e7,
			MaxCost:     1 << 30,
			BufferItems: 64,
		})
	if err != nil {
		return nil, fmt.Errorf("error initialising ristretto cache: %w", err)
	}
	return &RistrettoCacheBackend{c: cache}, nil
}

// SharCache provides caching capabilities
type SharCache struct {
	cacheBackend CacheBackend
}

// NewSharCache constructs a new SharCache
func NewSharCache(backend CacheBackend) *SharCache {
	return &SharCache{
		cacheBackend: backend,
	}
}

// Cacheable makes a function cacheable by the given key
func (c *SharCache) Cacheable(key interface{}, fn func() (interface{}, error)) (interface{}, error) {
	val, cacheHit := c.cacheBackend.Get(key)

	if !cacheHit {
		retrievedVal, err := fn()
		if err != nil {
			return nil, fmt.Errorf("error retrieving cacheable value for key %s: %w", key, err)
		}
		val = retrievedVal
		c.cacheBackend.Set(key, retrievedVal)
	}

	return val, nil
}
