package cache

import (
	"errors"
	"fmt"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestCacheHit(t *testing.T) {
	var backend = &MockCacheBackend{}
	cche := NewSharCache(backend)

	isCachableFnCalled := false
	key := "key"
	val := "value"
	backend.On("Get", key).Return(val, true)

	cacheableFn := func() (interface{}, error) {
		isCachableFnCalled = true
		return nil, nil
	}

	v, err := Cacheable(key, cacheableFn, cche)

	backend.AssertExpectations(t)
	assert.NoError(t, err)
	assert.Equal(t, val, v)
	assert.Equal(t, false, isCachableFnCalled, "cacheable fn should not have been called")
}

func TestCacheMiss(t *testing.T) {
	var backend = &MockCacheBackend{}
	cache := NewSharCache(backend)

	isCachableFnCalled := false
	key := "key"
	val := "value"
	backend.On("Get", key).Return(nil, false)
	backend.On("Set", key, val).Return(true)

	cacheableFn := func() (interface{}, error) {
		isCachableFnCalled = true
		return val, nil
	}

	v, err := Cacheable(key, cacheableFn, cache)

	backend.AssertExpectations(t)
	assert.NoError(t, err)
	assert.Equal(t, val, v)
	assert.Equal(t, true, isCachableFnCalled, "cacheable fn should have been called")
}

func TestCacheMissError(t *testing.T) {
	var backend = &MockCacheBackend{}
	cache := NewSharCache(backend)

	isCachableFnCalled := false
	key := "key"
	backend.On("Get", key).Return(nil, false)

	cacheableFn := func() (interface{}, error) {
		isCachableFnCalled = true
		return nil, fmt.Errorf("test chache missL %w", errors.New("cacheableFn err"))
	}

	v, err := Cacheable(key, cacheableFn, cache)

	backend.AssertExpectations(t)
	backend.AssertNotCalled(t, "Set")
	assert.Nil(t, v)
	assert.Error(t, err)
	assert.Equal(t, true, isCachableFnCalled, "cacheable fn should have been called")
}
