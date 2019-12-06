package cron

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/blend/go-sdk/bufferutil"
	"github.com/blend/go-sdk/ex"
)

// NewJobInvocation returns a new job invocation.
func NewJobInvocation(jobName string) *JobInvocation {
	output := new(bufferutil.Buffer)
	handlers := new(bufferutil.BufferHandlers)
	output.Handler = handlers.Handle
	return &JobInvocation{
		ID:             NewJobInvocationID(),
		Started:        Now(),
		State:          JobInvocationStateRunning,
		JobName:        jobName,
		Output:         output,
		OutputHandlers: handlers,
		Done:           make(chan struct{}),
	}
}

// JobInvocation is metadata for a job invocation (or instance of a job running).
type JobInvocation struct {
	ID             string
	JobName        string
	Started        time.Time
	Finished       time.Time
	Cancelled      time.Time
	Timeout        time.Time
	Err            error
	Elapsed        time.Duration
	Parameters     JobParameters
	State          JobInvocationState
	Output         *bufferutil.Buffer
	OutputHandlers *bufferutil.BufferHandlers
	Context        context.Context
	Cancel         context.CancelFunc
	Done           chan struct{}
}

// MarshalJSON marshals the invocation as json.
func (ji JobInvocation) MarshalJSON() ([]byte, error) {
	values := map[string]interface{}{
		"id":      ji.ID,
		"jobName": ji.JobName,
		"started": ji.Started,
		"elapsed": ji.Elapsed,
		"state":   ji.State,
		"output":  ji.Output,
	}
	if !ji.Finished.IsZero() {
		values["finished"] = ji.Finished
	}
	if !ji.Cancelled.IsZero() {
		values["cancelled"] = ji.Cancelled
	}
	if !ji.Timeout.IsZero() {
		values["timeout"] = ji.Timeout
	}
	if ji.Err != nil {
		values["err"] = ji.Err.Error()
	}
	contents, err := json.Marshal(values)
	return contents, ex.New(err)
}

// UnmarshalJSON unmarhsals
func (ji *JobInvocation) UnmarshalJSON(contents []byte) error {
	var values struct {
		ID        string             `json:"id"`
		JobName   string             `json:"jobName"`
		Started   time.Time          `json:"started"`
		Finished  time.Time          `json:"finished"`
		Cancelled time.Time          `json:"cancelled"`
		Timeout   time.Time          `json:"timeout"`
		Elapsed   time.Duration      `json:"elapsed"`
		State     JobInvocationState `json:"state"`
		Error     string             `json:"err"`
		Output    json.RawMessage    `json:"output"`
	}
	if err := json.Unmarshal(contents, &values); err != nil {
		return ex.New(err)
	}
	ji.ID = values.ID
	ji.JobName = values.JobName
	ji.Started = values.Started
	ji.Finished = values.Finished
	ji.Cancelled = values.Cancelled
	ji.Timeout = values.Timeout
	ji.Elapsed = values.Elapsed
	ji.State = values.State
	if values.Error != "" {
		ji.Err = errors.New(values.Error)
	}
	ji.Output = new(bufferutil.Buffer)
	if err := json.Unmarshal([]byte(values.Output), ji.Output); err != nil {
		return ex.New(err)
	}
	handlers := new(bufferutil.BufferHandlers)
	ji.Output.Handler = handlers.Handle
	ji.OutputHandlers = handlers
	return nil

}
