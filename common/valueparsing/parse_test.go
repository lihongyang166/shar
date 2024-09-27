package valueparsing

import (
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestExtract(t *testing.T) {
	text := `"orderId":int(78)`
	key, varType, value, err := extract(text)
	require.NoError(t, err)
	assert.Equal(t, "orderId", key, "Parser returned wrong value of the key")
	assert.Equal(t, "int", varType, "Parser returned wrong value of the type")
	assert.Equal(t, "78", value, "Parser returned wrong value")
}

func TestParse(t *testing.T) {
	args := []string{`"orderId":int(78)`, `"height":float64(103.101)`}
	vars, err := Parse(args)
	require.NoError(t, err)
	expectedOrderID := int64(78)
	expectedHeight := float64(103.101)
	orderId, err := vars.GetInt64("orderId")
	require.NoError(t, err)
	height, err := vars.GetFloat64("height")
	require.NoError(t, err)
	assert.Equal(t, expectedOrderID, orderId, "Parser returned wrong value")
	assert.Equal(t, expectedHeight, height, "Parser returned wrong value")
}
