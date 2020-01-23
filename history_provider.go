package jobkit

import "context"

// HistoryProvider is a provider for jobkit history specifically.
type HistoryProvider interface {
	RestoreHistory(context.Context) ([]JobInvocation, error)
	PersistHistory(context.Context, []JobInvocation) error
}
