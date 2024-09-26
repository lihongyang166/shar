package model_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"gitlab.com/shar-workflow/shar/model"
)

func TestVarsGet(t *testing.T) {
	/*
		type testType struct {
			int
			string
		}
	*/
	vars := model.NewVars()
	vars.SetString("1", "value")
	vars.SetFloat64("2", 77777.77777)
	//TODO: Re-implement structs
	//vars.SetStruct("4": testType{1, "2"}}

	s, err := vars.GetString("1")

	assert.NoError(t, err)
	assert.Equal(t, "value", s)

	f, err := vars.GetFloat64("2")

	assert.NoError(t, err)
	assert.Equal(t, 77777.77777, f)

	//y, err := model.get[testType](vars, "4")
	//assert.NoError(t, err)
	//assert.Equal(t, testType{1, "2"}, y)
}
