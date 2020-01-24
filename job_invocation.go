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
func NewJobInvocation(ji *cron.JobInvocation, jio *JobInvocationOutput) *JobInvocation {
	var invocation JobInvocation
	if ji != nil {
		invocation.JobInvocation = *ji
	}
	if jio != nil {
		invocation.JobInvocationOutput = *jio
	}
	return &invocation
}

var (
	_ json.Marshaler   = (*JobInvocation)(nil)
	_ json.Unmarshaler = (*JobInvocation)(nil)
)

// JobInvocation is a serialized form of a job invocation.
type JobInvocation struct {
	JobInvocation       cron.JobInvocation
	JobInvocationOutput JobInvocationOutput
}

// MarshalJSON implements json.Marshaler.
func (ji JobInvocation) MarshalJSON() ([]byte, error) {
	values := map[string]interface{}{
		"id":      ji.JobInvocation.ID,
		"jobName": ji.JobInvocation.JobName,
		"started": ji.JobInvocation.Started,
		"status":  ji.JobInvocation.Status,
		"output":  ji.JobInvocationOutput.Output,
	}
	if !ji.JobInvocation.Complete.IsZero() {
		values["complete"] = ji.JobInvocation.Complete
	}
	if ji.JobInvocation.Err != nil {
		values["err"] = ji.JobInvocation.Err.Error()
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
		ID       string                   `json:"id"`
		JobName  string                   `json:"jobName"`
		Started  time.Time                `json:"started"`
		Complete time.Time                `json:"complete"`
		Status   cron.JobInvocationStatus `json:"status"`
		Error    string                   `json:"err"`
		Output   json.RawMessage          `json:"output"`
	}
	if err := json.Unmarshal(contents, &values); err != nil {
		return ex.New(err)
	}
	ji.JobInvocation.ID = values.ID
	ji.JobInvocation.JobName = values.JobName
	ji.JobInvocation.Started = values.Started
	ji.JobInvocation.Complete = values.Complete
	ji.JobInvocation.Status = values.Status
	if values.Error != "" {
		ji.JobInvocation.Err = errors.New(values.Error)
	}
	ji.JobInvocationOutput.Output = new(bufferutil.Buffer)
	if err := json.Unmarshal([]byte(values.Output), ji.JobInvocationOutput.Output); err != nil {
		return ex.New(err)
	}
	handlers := new(bufferutil.BufferHandlers)
	ji.JobInvocationOutput.Output.Handler = handlers.Handle
	ji.JobInvocationOutput.OutputHandlers = handlers
	return nil
}
