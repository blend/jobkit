package jobkit

import (
	"context"
	"encoding/json"
	"net/url"
)

// ParameterValuesFromForm creates a parameter values set from url values.
func ParameterValuesFromForm(formValues url.Values) ParameterValues {
	output := make(ParameterValues)
	for key, values := range formValues {
		if len(values) == 0 {
			output[key] = ""
			continue
		}
		output[key] = values[0]
	}
	return output
}

// ParameterValuesFromJSON creates a parameter values set from json data.
func ParameterValuesFromJSON(data []byte) (ParameterValues, error) {
	output := make(ParameterValues)
	if err := json.Unmarshal(data, &output); err != nil {
		return nil, err
	}
	return output, nil
}

// ParameterValues is a loose association to map[string]string.
type ParameterValues = map[string]string

type contextKeyParameters struct{}

// WithParameterValues adds job invocation parameter values to a context.
func WithParameterValues(ctx context.Context, values ParameterValues) context.Context {
	return context.WithValue(ctx, contextKeyParameters{}, values)
}

// GetParameterValues gets parameter values from a given context.
func GetParameterValues(ctx context.Context) ParameterValues {
	if value := ctx.Value(contextKeyParameters{}); value != nil {
		if typed, ok := value.(ParameterValues); ok {
			return typed
		}
	}
	return nil
}
