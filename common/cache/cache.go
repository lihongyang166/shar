package cache

import (
	"fmt"

	"github.com/dgraph-io/ristretto"
)

// Backend defines interface for a Backend
type Backend[K ristretto.Key, V any] interface {
	Get(key K) (V, bool)
	Set(key K, value V) bool
}

// ristrettoCacheBackend is a RistrettoCache implemenentation of Backend
type ristrettoCacheBackend[K ristretto.Key, V any] struct {
	c *ristretto.Cache[K, V]
}

// Get a value from the cache
func (rcb *ristrettoCacheBackend[K, V]) Get(key K) (V, bool) { //nolint:ireturn
	return rcb.c.Get(key)
}

// Set a value in the cache
func (rcb *ristrettoCacheBackend[K, V]) Set(key K, value V) bool {
	return rcb.c.Set(key, value, 1)
}

// NewRistrettoCacheBackend construct an instance of a ristrettoCacheBackend
func NewRistrettoCacheBackend[K ristretto.Key, V any]() (*ristrettoCacheBackend[K, V], error) {
	cache, err := ristretto.NewCache(
		&ristretto.Config[K, V]{
			NumCounters: 1e7,
			MaxCost:     1 << 30,
			BufferItems: 64,
		})
	if err != nil {
		return nil, fmt.Errorf("error initialising ristretto cache: %w", err)
	}
	return &ristrettoCacheBackend[K, V]{c: cache}, nil
}

// Cacheable makes a function cacheable by the given key
//
//nolint:ireturn
func Cacheable[K ristretto.Key, V any](key K, fn func() (V, error), c Backend[K, any]) (V, error) {
	var val V
	tmpVal, cacheHit := c.Get(key)
	if !cacheHit {
		retrievedVal, err := fn()
		if err != nil {
			return val, fmt.Errorf("error retrieving cacheable value for key %v: %w", key, err)
		}
		c.Set(key, retrievedVal)
		val = retrievedVal
	} else {
		val = tmpVal.(V)
	}
	return val, nil
}
