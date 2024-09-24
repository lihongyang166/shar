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

/*
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
*/
