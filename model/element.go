package model

import (
	"errors"
	"fmt"
	"reflect"
)

// Vars is a map of variables. The variables must be primitive go types.
type Vars map[string]any

// ErrVarNotFound is returned when a variable is not found in the provided Vars map.
var ErrVarNotFound = errors.New("variable not found")

// Get takes the desired return type as parameter and safely searches the map and returns the value
// if it is found and is of the desired type.
func Get[V any](vars Vars, key string) (V, error) { //nolint:ireturn
	// v is the return type value
	var v V

	if vars[key] == nil {
		return v, fmt.Errorf("workflow var %s found nil", key)
	}

	v, ok := vars[key].(V)
	if !ok {
		return v, fmt.Errorf("workflow var %s not present: %w", key, ErrVarNotFound)
	}

	return v, nil
}

// GetString validates that a key has an underlying value in the map[string]interface{} vars
// and safely returns the result.
func (vars Vars) GetString(key string) (string, error) {
	v, err := Get[string](vars, key)
	if err != nil {
		return "", fmt.Errorf("getString: %w", err)
	}
	return v, nil
}

// GetInt64 validates that a key has an underlying value in the map[int]interface{} vars
// and safely returns the result.
func (vars Vars) GetInt64(key string) (int64, error) {
	xt, ok := vars[key]
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
func (vars Vars) GetBool(key string) (bool, error) {
	v, err := Get[bool](vars, key)
	if err != nil {
		return false, fmt.Errorf("getBool: %w", err)
	}
	return v, nil
}

// GetFloat64 validates that a key has an underlying value in the map[int]interface{} vars
// and safely returns the result.
func (vars Vars) GetFloat64(key string) (float64, error) {
	return Get[float64](vars, key)
}

// SetString sets a string value for the specified key in the Vars map.
func (vars Vars) SetString(key string, value string) {
	vars[key] = value
}

// SetInt64 sets an int64 value for the specified key in the Vars map.
func (vars Vars) SetInt64(key string, value int64) {
	vars[key] = value
}

// SetFloat64 sets a float64 value for the specified key in the Vars map.
func (vars Vars) SetFloat64(key string, value float64) {
	vars[key] = value
}

// SetBool sets a boolean value for the specified key in the Vars map.
func (vars Vars) SetBool(key string, value bool) {
	vars[key] = value
}

// Unmarshal unmarshals a SHAR compatible map-portable (map[string]interface{}) var into a struct
func Unmarshal[T any](vars Vars, key string) (*T, error) {
	t := new(T)
	err := fromMap(vars[key].(map[string]interface{}), t)
	if err != nil {
		return nil, err
	}
	return t, nil
}

// Marshal marshals a struct into a SHAR compatible map-portable (map[string]interface{}) var
func Marshal[T any](v *Vars, key string, t *T) error {
	m, err := toMap(t)
	if err != nil {
		return err
	}
	(*v)[key] = m
	return nil
}

// toMap struct to map[string]interface{}
func toMap(in interface{}) (map[string]interface{}, error) {
	out := make(map[string]interface{})
	v := reflect.ValueOf(in)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}
	if v.Kind() != reflect.Struct { // Non-structural return error
		return nil, fmt.Errorf("ToMap only accepts struct or struct pointer; got %T", v)
	}
	t := v.Type()
	for i := 0; i < v.NumField(); i++ {
		fi := t.Field(i)
		out[fi.Name] = v.Field(i).Interface()
	}
	return out, nil
}

// toMap map[string]interface{} to struct
func fromMap(m map[string]interface{}, s interface{}) error {
	stValue := reflect.ValueOf(s).Elem()
	sType := stValue.Type()
	for i := 0; i < sType.NumField(); i++ {
		field := sType.Field(i)
		if value, ok := m[field.Name]; ok {
			stValue.Field(i).Set(reflect.ValueOf(value))
		}
	}
	return nil
}

// NewVars creates and returns a new instance of Vars,
func NewVars() Vars {
	return Vars{}
}
