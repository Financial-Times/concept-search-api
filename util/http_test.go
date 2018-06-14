package util

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

var httpTestBasePath = "http://localhost/my_path"

func TestGetSingleQueryValueNoParam(t *testing.T) {
	req, _ := http.NewRequest("GET", httpTestBasePath, nil)
	value, found, err := GetSingleValueQueryParameter(req, "test-param")
	assert.Empty(t, value)
	assert.False(t, found)
	assert.NoError(t, err)
}

func TestGetSingleQueryValueMultiParams(t *testing.T) {
	req, _ := http.NewRequest("GET", httpTestBasePath+"?test-param=a&test-param=b", nil)
	value, found, err := GetSingleValueQueryParameter(req, "test-param")
	assert.Empty(t, value)
	assert.True(t, found)
	assert.Error(t, err)
	assert.Equal(t, err.Error(), "specified multiple test-param query parameters in the URL")
}

func TestGetSingleQueryValueNoAllowedValues(t *testing.T) {
	req, _ := http.NewRequest("GET", httpTestBasePath+"?test-param=a", nil)
	value, found, err := GetSingleValueQueryParameter(req, "test-param", "x")
	assert.Empty(t, value)
	assert.True(t, found)
	assert.Error(t, err)
	assert.Equal(t, err.Error(), "'a' is not a valid value for parameter 'test-param'")
}

func TestGetSingleQueryValueAllowedValue(t *testing.T) {
	req, _ := http.NewRequest("GET", httpTestBasePath+"?test-param=a", nil)
	value, found, err := GetSingleValueQueryParameter(req, "test-param", "x", "a")
	assert.Equal(t, "a", value)
	assert.True(t, found)
	assert.NoError(t, err)
}

func TestGetBoolValueMultiParams(t *testing.T) {
	req, _ := http.NewRequest("GET", httpTestBasePath+"?test-param=true&test-param=false", nil)
	value, found, err := GetBoolQueryParameter(req, "test-param", false)
	assert.False(t, value)
	assert.True(t, found)
	assert.Equal(t, "specified multiple test-param query parameters in the URL", err.Error())
}

func TestGetBoolValueNotBoolValueGiven(t *testing.T) {
	req, _ := http.NewRequest("GET", httpTestBasePath+"?test-param=a", nil)
	value, found, err := GetBoolQueryParameter(req, "test-param", false)
	assert.False(t, value)
	assert.False(t, found)
	assert.Equal(t, "strconv.ParseBool: parsing \"a\": invalid syntax", err.Error())
}

func TestGetBoolValueOkValue(t *testing.T) {
	req, _ := http.NewRequest("GET", httpTestBasePath+"?test-param=true", nil)
	value, found, err := GetBoolQueryParameter(req, "test-param", false)
	assert.True(t, value)
	assert.True(t, found)
	assert.NoError(t, err)
}
