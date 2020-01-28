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
	sync.Mutex
	Config  JobConfig
	History []*JobInvocation
	Lookup  map[string]*JobInvocation
}

// AddMany adds multiple invocations to the history store.
func (hm *HistoryMemory) AddMany(_ context.Context, invocations ...*JobInvocation) error {
	hm.Lock()
	defer hm.Unlock()
	if hm.Lookup == nil {
		hm.Lookup = make(map[string]*JobInvocation)
	}
	for _, ji := range invocations {
		hm.History = append(hm.History, ji)
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
	hm.History = append(hm.History, ji)
	hm.Lookup[ji.ID] = ji
	return nil
}

// Get returns all history for a given job.
func (hm *HistoryMemory) Get(_ context.Context) ([]*JobInvocation, error) {
	hm.Lock()
	defer hm.Unlock()
	return hm.History, nil
}

// GetByID gets a job invocation by ID.
func (hm *HistoryMemory) GetByID(_ context.Context, invocationID string) (*JobInvocation, error) {
	hm.Lock()
	defer hm.Unlock()

	if ji, ok := hm.Lookup[invocationID]; ok {
		return ji, nil
	}
	return nil, ex.New(cron.ErrJobNotFound)
}

// Cull culls history.
func (hm *HistoryMemory) Cull(_ context.Context) error {
	hm.Lock()
	defer hm.Unlock()

	count := len(hm.History)
	maxCount := hm.Config.HistoryMaxCountOrDefault()
	maxAge := hm.Config.HistoryMaxAgeOrDefault()

	now := time.Now().UTC()
	var filtered []*JobInvocation
	var removed []string
	for index, h := range hm.History {
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
	hm.History = filtered
	return nil
}
