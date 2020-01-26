package jobkit

import (
	"encoding/json"
	"net/url"
	"sort"

	"github.com/blend/go-sdk/cron"
)

// ParameterValuesFromForm creates a parameter values set from url values.
func ParameterValuesFromForm(parameters []Parameter, formValues url.Values) cron.JobParameters {
	output := make(cron.JobParameters)
	for _, param := range parameters {
		output[param.Name] = param.Value
	}

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
func ParameterValuesFromJSON(parameters []Parameter, data []byte) (cron.JobParameters, error) {
	output := make(cron.JobParameters)
	for _, param := range parameters {
		output[param.Name] = param.Value
	}

	if err := json.Unmarshal(data, &output); err != nil {
		return nil, err
	}
	return output, nil
}

// ParameterValuesAsEnviron returns params as environment values, i.e. key=value.
func ParameterValuesAsEnviron(params cron.JobParameters) (environ []string) {
	var keys []string
	for key := range params {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	var value string
	for _, key := range keys {
		value = params[key]
		environ = append(environ, key+"="+value)
	}
	return
}
