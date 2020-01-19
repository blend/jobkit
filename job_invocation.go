package jobkit

import (
	"github.com/blend/go-sdk/bufferutil"
	"github.com/blend/go-sdk/cron"
)

// JobInvocation is a wrapping job invocation.
type JobInvocation struct {
	cron.JobInvocation

	Output         *bufferutil.Buffer
	OutputHandlers *bufferutil.BufferHandlers
}
