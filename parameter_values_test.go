package jobkit

import (
	"net/url"
	"testing"

	"github.com/blend/go-sdk/assert"
)

func Test_ParameterValuesFromForm(t *testing.T) {
	assert := assert.New(t)

	params := []Parameter{
		{Name: "p0", Value: "p0value"},
		{Name: "p1", Value: "p1value"},
	}

	values := url.Values{
		"one":  nil,
		"foo":  []string{"bar", "baz"},
		"buzz": []string{"fuzz", "wuzz"},
	}

	parameterValues := ParameterValuesFromForm(params, values)
	assert.Len(parameterValues, 5)

	value, ok := parameterValues["one"]
	assert.True(ok)
	assert.Empty(value)

	assert.Equal("p0value", parameterValues["p0"])
	assert.Equal("p1value", parameterValues["p1"])
	assert.Equal("bar", parameterValues["foo"])
	assert.Equal("fuzz", parameterValues["buzz"])
}

func Test_ParameterValuesFromJSON(t *testing.T) {
	assert := assert.New(t)

	params := []Parameter{
		{Name: "p0", Value: "p0value"},
		{Name: "p1", Value: "p1value"},
	}

	data := []byte(`{"one":null,"foo":"bar", "buzz":"fuzz"}`)

	parameterValues, err := ParameterValuesFromJSON(params, data)
	assert.Nil(err)
	assert.Len(parameterValues, 5)

	value, ok := parameterValues["one"]
	assert.True(ok)
	assert.Empty(value)

	assert.Equal("p0value", parameterValues["p0"])
	assert.Equal("p1value", parameterValues["p1"])
	assert.Equal("bar", parameterValues["foo"])
	assert.Equal("fuzz", parameterValues["buzz"])
}
