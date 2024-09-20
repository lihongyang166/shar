package vars

import (
	"context"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gitlab.com/shar-workflow/shar/model"
	"testing"
)

func TestEncodeDecodeVars(t *testing.T) {
	v := make(model.Vars)
	ctx := context.Background()
	v.SetInt64("first", 56)
	v.SetString("second", "elvis")
	v.SetFloat64("third", 5.98)

	e, err := Encode(ctx, v)
	require.NoError(t, err)
	d, err := Decode(ctx, e)
	require.NoError(t, err)
	vFirst, err := d.GetInt64("first")
	require.NoError(t, err)
	vSecond, err := d.GetString("second")
	require.NoError(t, err)
	vThird, err := d.GetFloat64("third")
	require.NoError(t, err)
	assert.Equal(t, int64(56), vFirst)
	assert.Equal(t, "elvis", vSecond)
	assert.Equal(t, float64(5.98), vThird)
}
