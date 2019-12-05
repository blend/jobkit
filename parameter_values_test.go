package jobkit

import (
	"net/url"
	"testing"

	"github.com/blend/go-sdk/assert"
)

func Test_ParameterValuesFromForm(t *testing.T) {
	assert := assert.New(t)

	values := url.Values{
		"one":  nil,
		"foo":  []string{"bar", "baz"},
		"buzz": []string{"fuzz", "wuzz"},
	}

	parameterValues := ParameterValuesFromForm(values)
	assert.Len(parameterValues, 3)

	value, ok := parameterValues["one"]
	assert.True(ok)
	assert.Empty(value)

	assert.Equal("bar", parameterValues["foo"])
	assert.Equal("fuzz", parameterValues["buzz"])
}

func Test_ParameterValuesFromJSON(t *testing.T) {
	assert := assert.New(t)

	data := []byte(`{"one":null,"foo":"bar", "buzz":"fuzz"}`)

	parameterValues, err := ParameterValuesFromJSON(data)
	assert.Nil(err)
	assert.Len(parameterValues, 3)

	value, ok := parameterValues["one"]
	assert.True(ok)
	assert.Empty(value)

	assert.Equal("bar", parameterValues["foo"])
	assert.Equal("fuzz", parameterValues["buzz"])
}
