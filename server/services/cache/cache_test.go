package cache

import (
	"errors"
	"fmt"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestCacheHit(t *testing.T) {
	var backend = &MockBackend[string, any]{}
	cche := NewSharCache[string, any](backend)

	isCachableFnCalled := false
	key := "key"
	val := "value"
	backend.On("Get", key).Return(val, true)

	cacheableFn := func() (string, error) {
		isCachableFnCalled = true
		return "h", nil
	}

	v, err := Cacheable[string, string](key, cacheableFn, cche)

	backend.AssertExpectations(t)
	assert.NoError(t, err)
	assert.Equal(t, val, v)
	assert.Equal(t, false, isCachableFnCalled, "cacheable fn should not have been called")
}

func TestCacheHitIntKeyStringValue(t *testing.T) {
	var backend = &MockBackend[int, any]{}
	cche := NewSharCache[int, any](backend)

	isCachableFnCalled := false
	key := 3
	val := "value"
	backend.On("Get", key).Return(val, true)

	cacheableFn := func() (string, error) {
		isCachableFnCalled = true
		return "3", nil
	}

	v, err := Cacheable[int, string](key, cacheableFn, cche)

	backend.AssertExpectations(t)
	assert.NoError(t, err)
	assert.Equal(t, val, v)
	assert.Equal(t, false, isCachableFnCalled, "cacheable fn should not have been called")
}

func TestCacheMiss(t *testing.T) {
	var backend = &MockBackend[string, any]{}
	cache := NewSharCache[string, any](backend)

	isCachableFnCalled := false
	key := "key"
	val := "value"
	backend.On("Get", key).Return("", false)
	backend.On("Set", key, val).Return(true)

	cacheableFn := func() (string, error) {
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
	var backend = &MockBackend[string, any]{}
	cache := NewSharCache[string, any](backend)

	isCachableFnCalled := false
	key := "key"
	backend.On("Get", key).Return("", false)

	cacheableFn := func() (string, error) {
		isCachableFnCalled = true
		return "", fmt.Errorf("test chache missL %w", errors.New("cacheableFn err"))
	}

	v, err := Cacheable[string, string](key, cacheableFn, cache)

	backend.AssertExpectations(t)
	backend.AssertNotCalled(t, "Set")
	assert.Equal(t, "", v)
	assert.Error(t, err)
	assert.Equal(t, true, isCachableFnCalled, "cacheable fn should have been called")
}
