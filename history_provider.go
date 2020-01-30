package jobkit

import (
	"context"
	"time"
)

// HistoryProvider is a provider for jobkit history specifically.
type HistoryProvider interface {
	Initialize(ctx context.Context) error
	Add(context.Context, *JobInvocation) error
	Get(context.Context, string) ([]*JobInvocation, error)
	GetByID(context.Context, string, string) (*JobInvocation, error)
	Cull(context.Context, string, int, time.Duration) error
}
