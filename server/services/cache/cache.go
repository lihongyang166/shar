package cache

import (
	"fmt"

	"github.com/dgraph-io/ristretto"
)

// Backend defines interface for a Backend
//
//go:generate mockery
type Backend interface {
	Get(key interface{}) (interface{}, bool)
	Set(key interface{}, value interface{}) bool
}

// ristrettoCacheBackend is a RistrettoCache implemenentation of Backend
type ristrettoCacheBackend struct {
	c *ristretto.Cache[string, any]
}

// Get a value from the cache
func (rcb *ristrettoCacheBackend) Get(key interface{}) (interface{}, bool) {
	return rcb.c.Get(key.(string))
}

// Set a value in the cache
func (rcb *ristrettoCacheBackend) Set(key interface{}, value interface{}) bool {
	return rcb.c.Set(key.(string), value, 1)
}

// NewRistrettoCacheBackend construct an instance of a ristrettoCacheBackend
func NewRistrettoCacheBackend() (*ristrettoCacheBackend, error) {
	cache, err := ristretto.NewCache(
		&ristretto.Config[string, any]{
			NumCounters: 1e7,
			MaxCost:     1 << 30,
			BufferItems: 64,
		})
	if err != nil {
		return nil, fmt.Errorf("error initialising ristretto cache: %w", err)
	}
	return &ristrettoCacheBackend{c: cache}, nil
}

// SharCache provides caching capabilities
type SharCache struct {
	cacheBackend Backend
}

// NewSharCache constructs a new SharCache
func NewSharCache(backend Backend) *SharCache {
	return &SharCache{
		cacheBackend: backend,
	}
}

// Cacheable makes a function cacheable by the given key
//
//nolint:ireturn
func Cacheable[K any, V any](key K, fn func() (V, error), c *SharCache) (V, error) {
	var val V
	tmpVal, cacheHit := c.cacheBackend.Get(key)
	if !cacheHit {
		retrievedVal, err := fn()
		if err != nil {
			return val, fmt.Errorf("error retrieving cacheable value for key %v: %w", key, err)
		}
		c.cacheBackend.Set(key, retrievedVal)
		val = retrievedVal
	} else {
		val = tmpVal.(V)
	}
	return val, nil
}
