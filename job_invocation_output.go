package jobkit

import (
	"github.com/blend/go-sdk/bufferutil"
)

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
