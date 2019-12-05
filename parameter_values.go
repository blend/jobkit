package jobkit

import (
	"encoding/json"
	"net/url"

	"github.com/blend/go-sdk/cron"
)

// ParameterValuesFromForm creates a parameter values set from url values.
func ParameterValuesFromForm(formValues url.Values) cron.JobParameters {
	output := make(cron.JobParameters)
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
func ParameterValuesFromJSON(data []byte) (cron.JobParameters, error) {
	output := make(cron.JobParameters)
	if err := json.Unmarshal(data, &output); err != nil {
		return nil, err
	}
	return output, nil
}

// ParameterValuesAsEnviron returns params as environment values, i.e. key=value.
func ParameterValuesAsEnviron(params cron.JobParameters) (environ []string) {
	for key, value := range params {
		environ = append(environ, key+"="+value)
	}
	return
}
