package jobkit

import "context"

// HistoryProvider is a provider for jobkit history specifically.
type HistoryProvider interface {
	Add(context.Context, *JobInvocation) error
	Get(context.Context) ([]*JobInvocation, error)
	GetByID(context.Context, string) (*JobInvocation, error)
	Cull(context.Context) error
}
