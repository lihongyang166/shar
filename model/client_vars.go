package model

import (
	"context"
	"encoding/json"
	"fmt"
	"iter"
	"log/slog"
	"maps"
	"reflect"

	"github.com/vmihailenco/msgpack/v5"
	"gitlab.com/shar-workflow/shar/common/logx"
)

// ClientVars holds a map of client-defined variables with string keys and values of workflow variables.
type ClientVars struct {
	vals map[string]any
}

// NewVars creates and returns a new instance of ClientVars.
func NewVars() *ClientVars {
	return &ClientVars{
		vals: make(map[string]any),
	}
}

// New creates and returns a new instance of ClientVars populated with the given map of vars.
func New(vars map[string]any) (*ClientVars, error) {
	for k, v := range vars {
		switch v.(type) {
		case string, bool, int64, float64:
			continue
		default:
			return nil, fmt.Errorf("%s is not a suppported type, please convert to string, bool, int64 or float64", k)
		}
	}

	return &ClientVars{
		vals: vars,
	}, nil
}

// get takes the desired return type as parameter and safely searches the map and returns the value
// if it is found and is of the desired type.
func get[V any](vars *ClientVars, key string) (V, error) { //nolint:ireturn
	// v is the return type value
	var v V

	if vars.vals[key] == nil {
		return v, fmt.Errorf("workflow var %s found nil", key)
	}

	v, ok := vars.vals[key].(V)
	if !ok {
		return v, fmt.Errorf("workflow var %s not present: %w", key, ErrVarNotFound)
	}

	return v, nil
}

// GetString validates that a key has an underlying value in the map[string]interface{} vars
// and safely returns the result.
func (vars *ClientVars) GetString(key string) (string, error) {
	v, err := get[string](vars, key)
	if err != nil {
		return "", fmt.Errorf("getString: %w", err)
	}
	return v, nil
}

// GetInt64 validates that a key has an underlying value in the map[int]interface{} vars
// and safely returns the result.
func (vars *ClientVars) GetInt64(key string) (int64, error) {
	xt, ok := vars.vals[key]
	if !ok {
		return 0, fmt.Errorf("workflow var %s not present: %w", key, ErrVarNotFound)
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
		return 0, fmt.Errorf("workflow var %s is %s not int64: %w", key, reflect.TypeOf(xt).Name(), ErrVarNotFound)
	}
}

// GetBool validates that a key has an underlying value in the map[int]interface{} vars
// and safely returns the result.
func (vars *ClientVars) GetBool(key string) (bool, error) {
	v, err := get[bool](vars, key)
	if err != nil {
		return false, fmt.Errorf("getBool: %w", err)
	}
	return v, nil
}

// GetFloat64 validates that a key has an underlying value in the map[int]interface{} vars
// and safely returns the result.
func (vars *ClientVars) GetFloat64(key string) (float64, error) {
	return get[float64](vars, key)
}

// SetString sets a string value for the specified key in the Vars map.
func (vars *ClientVars) SetString(key string, value string) {
	vars.vals[key] = value
}

// SetInt64 sets an int64 value for the specified key in the Vars map.
func (vars *ClientVars) SetInt64(key string, value int64) {
	vars.vals[key] = value
}

// SetFloat64 sets a float64 value for the specified key in the Vars map.
func (vars *ClientVars) SetFloat64(key string, value float64) {
	vars.vals[key] = value
}

// SetBool sets a boolean value for the specified key in the Vars map.
func (vars *ClientVars) SetBool(key string, value bool) {
	vars.vals[key] = value
}

// Encode encodes the map of workflow variables into a go binary to be sent across the wire.
func (vars *ClientVars) Encode(ctx context.Context) ([]byte, error) {
	b, err := msgpack.Marshal(vars.vals)
	if err != nil {
		return nil, logx.Err(ctx, "encode vars", err, slog.Any("vars", vars))
	}
	return b, nil
}

// Decode decodes a go binary object containing workflow variables.
func (vars *ClientVars) Decode(ctx context.Context, b []byte) error {
	if len(b) == 0 {
		return nil
	}

	if err := msgpack.Unmarshal(b, &vars.vals); err != nil {
		return logx.Err(ctx, "decode vars", err, slog.Any("vars", vars))
	}
	return nil
}

// Len returns the number of key-value pairs in ClientVars.
func (vars *ClientVars) Len() int {
	return len(vars.vals)
}

// Keys returns an iterator sequence of all the keys in the ClientVars map.
func (vars *ClientVars) Keys() iter.Seq[string] {
	return maps.Keys(vars.vals)
}

// GetStruct unmarshals a SHAR variable into a struct.
func GetStruct[T any](vars *ClientVars, key string) (*T, error) {
	t := new(T)
	k, ok := vars.vals[key]
	if !ok {
		return t, fmt.Errorf("workflow var %s found nil: %w", key, ErrVarNotFound)
	}
	switch k.(type) {
	case map[string]interface{}:
		b, err := json.Marshal(k)
		if err != nil {
			return nil, fmt.Errorf("marshal json %s: %w", key, err)
		}
		if err := json.Unmarshal(b, t); err != nil {
			return nil, fmt.Errorf("unmarshal json %s: %w", key, err)
		}
	default:
		return nil, fmt.Errorf("workflow var %s is %s not map[string]interface{}: %w", key, reflect.TypeOf(k).Name(), ErrVarNotFound)
	}
	return t, nil
}

// SetStruct marshals a struct into a SHAR variable.
func SetStruct[T any](v *ClientVars, key string, t *T) error {
	if err := scanType(t); err != nil {
		return fmt.Errorf("set struct: %w", err)
	}
	b, err := json.Marshal(t)
	if err != nil {
		return fmt.Errorf("marshal struct json %s: %w", key, err)
	}
	mp := make(map[string]interface{})
	if err := json.Unmarshal(b, &mp); err != nil {
		return fmt.Errorf("unmarshal struct json %s: %w", key, err)
	}
	(*v).vals[key] = mp
	return nil
}

func scanType[T any](t *T) error {
	tp := reflect.TypeOf(t).Elem()
	if err := scanReflect(tp); err != nil {
		return fmt.Errorf("type contains fields incompatible with SHAR variables: %w", err)
	}
	return nil
}

func scanReflect(t reflect.Type) error {
	fmt.Println(t.Name())
	if t.Kind() != reflect.Struct {
		return fmt.Errorf("type %s is not a struct", t.Name())
	}
	for i := 0; i < t.NumField(); i++ {
		fld := t.Field(i)
		switch fld.Type.Kind() {
		case reflect.Int64:
		case reflect.Float64:
		case reflect.Bool:
		case reflect.String:
		case reflect.Struct:
			if err := scanReflect(fld.Type); err != nil {
				return fmt.Errorf("validate struct %s: %w", t.Name(), err) // nolint
			}
		default:
			return fmt.Errorf("field %s (%s) is not of a permitted type", fld.Name, fld.Type.Name())
		}
	}
	return nil
}
