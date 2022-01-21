package util

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestContainsPositive(t *testing.T) {
	slice1 := []string{"foo", "bar"}
	target := "bar"

	contains, err := contains(slice1, target)

	assert.True(t, contains)
	assert.Nil(t, err)
}

func TestContainsNegative(t *testing.T) {
	slice1 := []string{"foo", "bar"}
	target := "baz"

	contains, err := contains(slice1, target)

	assert.False(t, contains)
	assert.Nil(t, err)
}

func TestContainsEmptySlice(t *testing.T) {
	slice1 := []string{}
	target := "baz"

	contains, err := contains(slice1, target)

	assert.False(t, contains)
	assert.Error(t, err)
}

func TestContainsEmptyString(t *testing.T) {
	slice1 := []string{"foo", "bar"}
	target := ""

	contains, err := contains(slice1, target)

	assert.False(t, contains)
	assert.Error(t, err)
}
