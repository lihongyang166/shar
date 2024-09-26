package model

import (
	"context"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
)

type person struct {
	//Friends []string
	Age     int64
	Address struct {
		HouseNumber int64
		Postcode    string
	}
}

func TestMarshalUnmarshal(t *testing.T) {
	v := NewVars()

	p := person{
		//Friends: []string{"John", "Fred"},
		Age: 40,
		Address: struct {
			HouseNumber int64
			Postcode    string
		}{
			HouseNumber: 21,
			Postcode:    "CO1 1AA",
		},
	}

	err := SetStruct(v, "person", &p)
	assert.NoError(t, err)
	n, err := GetStruct[person](v, "person")
	assert.NoError(t, err)
	assert.Equal(t, *n, p)

	ctx := context.Background()
	b, err := v.Encode(ctx)
	require.NoError(t, err)
	c := NewVars()
	err = c.Decode(ctx, b)
	require.NoError(t, err)
	n, err = GetStruct[person](c, "person")
	assert.NoError(t, err)
	assert.Equal(t, *n, p)
}
