package model

import (
	"context"
	"fmt"
	"github.com/vmihailenco/msgpack/v5"
	"gitlab.com/shar-workflow/shar/common/logx"
	"gitlab.com/shar-workflow/shar/model"
	"iter"
	"log/slog"
	"maps"
	"reflect"
)

// ServerVars manages server variables with a map of key-value pairs.
type ServerVars struct {
	Vals map[string]any
}

// NewServerVars creates and returns a new instance of Vars,
func NewServerVars() *ServerVars {
	return &ServerVars{
		Vals: make(map[string]any),
	}
}

// Get takes the desired return type as parameter and safely searches the map and returns the value
// if it is found and is of the desired type.
func Get[V any](vars *ServerVars, key string) (V, error) { //nolint:ireturn
	// v is the return type value
	var v V

	if vars.Vals[key] == nil {
		return v, fmt.Errorf("workflow var %s found nil", key)
	}

	v, ok := vars.Vals[key].(V)
	if !ok {
		return v, fmt.Errorf("workflow var %s not present: %w", key, model.ErrVarNotFound)
	}

	return v, nil
}

// GetString validates that a key has an underlying value in the map[string]interface{} vars
// and safely returns the result.
func (vars *ServerVars) GetString(key string) (string, error) {
	v, err := Get[string](vars, key)
	if err != nil {
		return "", fmt.Errorf("getString: %w", err)
	}
	return v, nil
}

// GetInt64 validates that a key has an underlying value in the map[int]interface{} vars
// and safely returns the result.
func (vars *ServerVars) GetInt64(key string) (int64, error) {
	xt, ok := vars.Vals[key]
	if !ok {
		return 0, fmt.Errorf("workflow var %s not present: %w", key, model.ErrVarNotFound)
	}
	switch ut := xt.(type) {
	case int8:
		return int64(ut), nil
	case int16:
		return int64(ut), nil
	case int32:
		return int64(ut), nil
	case int64:
		return ut, nil
	case uint8:
		return int64(ut), nil
	case uint16:
		return int64(ut), nil
	case uint32:
		return int64(ut), nil
	default:
		return 0, fmt.Errorf("workflow var %s is %s not int64: %w", key, reflect.TypeOf(xt).Name(), model.ErrVarNotFound)
	}
}

// GetBool validates that a key has an underlying value in the map[int]interface{} vars
// and safely returns the result.
func (vars *ServerVars) GetBool(key string) (bool, error) {
	v, err := Get[bool](vars, key)
	if err != nil {
		return false, fmt.Errorf("getBool: %w", err)
	}
	return v, nil
}

// GetFloat64 validates that a key has an underlying value in the map[int]interface{} vars
// and safely returns the result.
func (vars *ServerVars) GetFloat64(key string) (float64, error) {
	return Get[float64](vars, key)
}

// SetString sets a string value for the specified key in the Vars map.
func (vars *ServerVars) SetString(key string, value string) {
	vars.Vals[key] = value
}

// SetInt64 sets an int64 value for the specified key in the Vars map.
func (vars *ServerVars) SetInt64(key string, value int64) {
	vars.Vals[key] = value
}

// SetFloat64 sets a float64 value for the specified key in the Vars map.
func (vars *ServerVars) SetFloat64(key string, value float64) {
	vars.Vals[key] = value
}

// SetBool sets a boolean value for the specified key in the Vars map.
func (vars *ServerVars) SetBool(key string, value bool) {
	vars.Vals[key] = value
}

// Encode encodes the map of workflow variables into a go binary to be sent across the wire.
func (vars *ServerVars) Encode(ctx context.Context) ([]byte, error) {
	b, err := msgpack.Marshal(vars.Vals)
	if err != nil {
		return nil, logx.Err(ctx, "encode vars", err, slog.Any("vars", vars))
	}
	return b, nil
}

// Decode decodes a go binary object containing workflow variables.
func (vars *ServerVars) Decode(ctx context.Context, b []byte) error {
	if len(b) == 0 {
		return nil
	}

	if err := msgpack.Unmarshal(b, &vars.Vals); err != nil {
		return logx.Err(ctx, "decode vars", err, slog.Any("vars", vars))
	}
	return nil
}

// Keys returns a sequence of all keys present in the Vars map.
func (vars *ServerVars) Keys() iter.Seq[string] {
	return maps.Keys(vars.Vals)
}

// Len returns the number of key-value pairs in the Vals map of ServerVars.
func (vars *ServerVars) Len() int {
	return len(vars.Vals)
}
