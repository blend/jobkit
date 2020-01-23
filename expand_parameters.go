package jobkit

import (
	"os"

	"github.com/blend/go-sdk/cron"
)

// ExpandParameters returns a new parameter expander for a given job invocation.
func ExpandParameters(ji *cron.JobInvocation) func(string) string {
	return func(name string) string {
		if ji.Parameters != nil {
			if value, ok := ji.Parameters[name]; ok {
				return value
			}
		}
		return os.Getenv(name)
	}
}
