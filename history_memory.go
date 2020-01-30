package jobkit

import (
	"context"
	"sync"
	"time"

	"github.com/blend/go-sdk/cron"
	"github.com/blend/go-sdk/ex"
)

var (
	_ HistoryProvider = (*HistoryMemory)(nil)
)

// HistoryMemory is a memory backed history store.
type HistoryMemory struct {
	sync.RWMutex
	History map[string][]*JobInvocation
	Lookup  map[string]*JobInvocation
}

// Initialize initializes the backing maps.
func (hm *HistoryMemory) Initialize(ctx context.Context) error {
	hm.Lock()
	defer hm.Unlock()
	if hm.History == nil {
		hm.History = make(map[string][]*JobInvocation)
	}
	if hm.Lookup == nil {
		hm.Lookup = make(map[string]*JobInvocation)
	}
	return nil
}

// AddMany adds multiple invocations to the history store.
func (hm *HistoryMemory) AddMany(_ context.Context, invocations ...*JobInvocation) error {
	hm.Lock()
	defer hm.Unlock()
	for _, ji := range invocations {
		hm.History[ji.JobName] = append(hm.History[ji.JobName], ji)
		hm.Lookup[ji.ID] = ji
	}
	return nil
}

// Add adds a result.
func (hm *HistoryMemory) Add(_ context.Context, ji *JobInvocation) error {
	hm.Lock()
	defer hm.Unlock()
	if hm.Lookup == nil {
		hm.Lookup = make(map[string]*JobInvocation)
	}
	hm.History[ji.JobName] = append(hm.History[ji.JobName], ji)
	hm.Lookup[ji.ID] = ji
	return nil
}

// Get returns all history for a given job.
func (hm *HistoryMemory) Get(_ context.Context, jobName string) ([]*JobInvocation, error) {
	hm.RLock()
	defer hm.RUnlock()
	if hm.History == nil {
		return nil, nil
	}
	return hm.History[jobName], nil
}

// GetByID gets a job invocation by ID.
func (hm *HistoryMemory) GetByID(_ context.Context, jobName, invocationID string) (*JobInvocation, error) {
	hm.RLock()
	defer hm.RUnlock()

	if hm.Lookup == nil {
		return nil, nil
	}
	if ji, ok := hm.Lookup[invocationID]; ok {
		return ji, nil
	}
	return nil, ex.New(cron.ErrJobNotFound)
}

// Cull culls history.
func (hm *HistoryMemory) Cull(_ context.Context, jobName string, maxCount int, maxAge time.Duration) error {
	hm.Lock()
	defer hm.Unlock()

	count := len(hm.History)

	now := time.Now().UTC()
	var filtered []*JobInvocation
	var removed []string
	for index, h := range hm.History[jobName] {
		if maxCount > 0 {
			if index < (count - maxCount) {
				removed = append(removed, h.JobInvocation.ID)
				continue
			}
		}
		if maxAge > 0 {
			if now.Sub(h.JobInvocation.Started) > maxAge {
				continue
			}
		}
		filtered = append(filtered, h)
	}

	for _, id := range removed {
		delete(hm.Lookup, id)
	}
	hm.History[jobName] = filtered
	return nil
}
