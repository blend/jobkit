package jobkit

import (
	"encoding/json"
	"errors"
	"time"

	"github.com/blend/go-sdk/bufferutil"
	"github.com/blend/go-sdk/cron"
	"github.com/blend/go-sdk/ex"
)

// NewJobInvocation creates a new jobkit job invocation from a cron job invocation.
func NewJobInvocation(ji *cron.JobInvocation) *JobInvocation {
	if ji == nil {
		return nil
	}
	invocation := &JobInvocation{
		JobInvocation: *ji.Clone(),
	}
	if typed, ok := invocation.State.(*JobInvocationOutput); ok && typed != nil {
		invocation.JobInvocationOutput = *typed
	}
	return invocation
}

var (
	_ json.Marshaler   = (*JobInvocation)(nil)
	_ json.Unmarshaler = (*JobInvocation)(nil)
)

// JobInvocation is a serialized form of a job invocation.
type JobInvocation struct {
	cron.JobInvocation
	JobInvocationOutput
}

// MarshalJSON implements json.Marshaler.
func (ji JobInvocation) MarshalJSON() ([]byte, error) {
	values := map[string]interface{}{
		"id":         ji.ID,
		"jobName":    ji.JobName,
		"started":    ji.Started,
		"status":     ji.Status,
		"parameters": ji.Parameters,
		"output":     ji.Output,
	}
	if !ji.Complete.IsZero() {
		values["complete"] = ji.Complete
	}
	if ji.Err != nil {
		values["err"] = ji.Err.Error()
	}
	contents, err := json.Marshal(values)
	if err != nil {
		return nil, ex.New(err)
	}
	return contents, nil
}

// UnmarshalJSON unmarhsals
func (ji *JobInvocation) UnmarshalJSON(contents []byte) error {
	var values struct {
		ID         string                   `json:"id"`
		JobName    string                   `json:"jobName"`
		Started    time.Time                `json:"started"`
		Complete   time.Time                `json:"complete"`
		Status     cron.JobInvocationStatus `json:"status"`
		Error      string                   `json:"err"`
		Parameters map[string]string        `json:"parameters"`
		Output     json.RawMessage          `json:"output"`
	}
	if err := json.Unmarshal(contents, &values); err != nil {
		return ex.New(err)
	}
	ji.ID = values.ID
	ji.JobName = values.JobName
	ji.Started = values.Started
	ji.Complete = values.Complete
	ji.Status = values.Status
	if values.Error != "" {
		ji.Err = errors.New(values.Error)
	}
	ji.Parameters = values.Parameters
	ji.Output = new(bufferutil.Buffer)
	if err := json.Unmarshal([]byte(values.Output), ji.JobInvocationOutput.Output); err != nil {
		return ex.New(err)
	}
	handlers := new(bufferutil.BufferHandlers)
	ji.Output.Handler = handlers.Handle
	ji.OutputHandlers = handlers
	return nil
}
