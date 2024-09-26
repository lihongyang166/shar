package model

import (
	"context"
	"errors"
	"iter"
)

// Vars is a map of variables. The variables must be primitive go types.
type Vars interface {
	GetString(key string) (string, error)
	GetInt64(key string) (int64, error)
	GetBool(key string) (bool, error)
	GetFloat64(key string) (float64, error)
	SetString(key string, value string)
	SetInt64(key string, value int64)
	SetFloat64(key string, value float64)
	SetBool(key string, value bool)
	Decode(ctx context.Context, vars []byte) error
	Encode(ctx context.Context) ([]byte, error)
	Keys() iter.Seq[string]
	Len() int
}

// ErrVarNotFound is returned when a variable is not found in the provided Vars map.
var ErrVarNotFound = errors.New("variable not found")
