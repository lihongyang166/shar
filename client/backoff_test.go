package client

import (
	"github.com/stretchr/testify/assert"
	"gitlab.com/shar-workflow/shar/model"
	"testing"
	"time"
)

func TestExponential(t *testing.T) {
	samples := 10
	actual := make([]int64, samples)
	expected := []int64{0, 2000, 4000, 8000, 16000, 32000, 64000, 128000, 256000, 512000}
	for i := 0; i < 10; i++ {
		offset := getOffset(model.RetryStrategy_Exponential, 0, 1000, int64(i+1), 512000, time.Now())
		actual[i] = offset.Milliseconds()
	}
	assert.Equal(t, expected, actual)
}

func TestLinear(t *testing.T) {
	samples := 10
	actual := make([]int64, samples)
	expected := []int64{0, 1000, 2000, 3000, 4000, 5000, 6000, 7000, 8000, 9000}
	for i := 0; i < 10; i++ {
		offset := getOffset(model.RetryStrategy_Linear, 0, 1000, int64(i+1), 50000, time.Now())
		actual[i] = offset.Milliseconds()
	}
	assert.Equal(t, expected, actual)
}

func TestLinearCeiling(t *testing.T) {
	samples := 10
	actual := make([]int64, samples)
	expected := []int64{0, 1000, 2000, 3000, 4000, 5000, 6000, 6000, 6000, 6000}
	for i := 0; i < 10; i++ {
		offset := getOffset(model.RetryStrategy_Linear, 0, 1000, int64(i+1), 6000, time.Now())
		actual[i] = offset.Milliseconds()
	}
	assert.Equal(t, expected, actual)
}
