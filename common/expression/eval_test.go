package expression

import (
	"context"
	"github.com/stretchr/testify/assert"
	"testing"
)

var eng = &ExprEngine{}

func TestPositive(t *testing.T) {
	ctx := context.Background()
	vrs := make(map[string]interface{})
	res, err := Eval[bool](ctx, eng, "97 == 97", vrs)
	assert.NoError(t, err)
	assert.Equal(t, true, res)
}

func TestNoVariable(t *testing.T) {
	ctx := context.Background()
	vrs := make(map[string]interface{})
	res, err := Eval[bool](ctx, eng, "a == 4.5", vrs)
	assert.NoError(t, err)
	assert.Equal(t, false, res)
}

func TestVariable(t *testing.T) {
	ctx := context.Background()
	vrs := make(map[string]interface{})
	vrs["a"] = 4.5
	res, err := Eval[bool](ctx, eng, "a == 4.5", vrs)
	assert.NoError(t, err)
	assert.Equal(t, true, res)
}
