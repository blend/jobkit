package jobkit

import (
	"context"

	"github.com/blend/go-sdk/bufferutil"
)

type jobInvocationOutputKey struct{}

// GetJobInvocationOutput gets a job invocation from a context.
func GetJobInvocationOutput(ctx context.Context) *JobInvocationOutput {
	if value := ctx.Value(jobInvocationOutputKey{}); value != nil {
		if typed, ok := value.(*JobInvocationOutput); ok {
			return typed
		}
	}
	return nil
}

// WithJobInvocationOutput adds a job invocation to a context.
func WithJobInvocationOutput(ctx context.Context, jio *JobInvocationOutput) context.Context {
	return context.WithValue(ctx, jobInvocationOutputKey{}, jio)
}

// NewJobInvocationOutput returns a new job invocation output.
func NewJobInvocationOutput() *JobInvocationOutput {
	outputHandlers := new(bufferutil.BufferHandlers)
	output := new(bufferutil.Buffer)
	output.Handler = outputHandlers.Handle
	return &JobInvocationOutput{
		Output:         output,
		OutputHandlers: outputHandlers,
	}
}

// JobInvocationOutput is a wrapping job invocation.
type JobInvocationOutput struct {
	Output         *bufferutil.Buffer
	OutputHandlers *bufferutil.BufferHandlers
}
