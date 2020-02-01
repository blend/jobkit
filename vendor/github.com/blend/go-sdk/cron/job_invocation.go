package cron

import (
	"context"
	"time"

	"github.com/blend/go-sdk/uuid"
)

// NewJobInvocation returns a new job invocation.
func NewJobInvocation(jobName string) *JobInvocation {
	return &JobInvocation{
		ID:      NewJobInvocationID(),
		Status:  JobInvocationStatusIdle,
		JobName: jobName,
		Done:    make(chan struct{}),
	}
}

// NewJobInvocationID returns a new pseudo-unique job invocation identifier.
func NewJobInvocationID() string {
	return uuid.V4().String()
}

// JobInvocation is metadata for a job invocation (or instance of a job running).
type JobInvocation struct {
	ID      string `json:"id"`
	JobName string `json:"jobName"`

	Started  time.Time `json:"started"`
	Complete time.Time `json:"complete"`
	Err      error     `json:"err"`

	Parameters JobParameters       `json:"parameters"`
	Status     JobInvocationStatus `json:"status"`

	// these cannot be json marshaled.
	State   interface{}        `json:"-"`
	Context context.Context    `json:"-"`
	Cancel  context.CancelFunc `json:"-"`
	Done    chan struct{}      `json:"-"`
}

// Elapsed returns the elapsed time for the invocation.
func (ji JobInvocation) Elapsed() time.Duration {
	if !ji.Complete.IsZero() {
		return ji.Complete.Sub(ji.Started)
	}
	return 0
}
