package jobkit

import (
	"context"
	"sort"
	"time"

	"github.com/blend/go-sdk/cron"
	"github.com/blend/go-sdk/ex"
)

// NewJobViewModels returns the job view models.
func NewJobViewModels(jobs map[string]*cron.JobScheduler) ([]*JobViewModel, error) {
	var jobSchedulers []*cron.JobScheduler
	for _, jobScheduler := range jobs {
		jobSchedulers = append(jobSchedulers, jobScheduler)
	}
	sort.Sort(cron.JobSchedulersByJobNameAsc(jobSchedulers))
	var output []*JobViewModel
	for _, jobScheduler := range jobSchedulers {
		jvm, err := NewJobViewModel(jobScheduler)
		if err != nil {
			return nil, err
		}
		output = append(output, jvm)
	}
	return output, nil
}

// FilterJobViewModels filters a set of job view models by a predicate.
func FilterJobViewModels(jobs []*JobViewModel, predicate func(*JobViewModel) bool) (output []*JobViewModel) {
	for _, job := range jobs {
		if predicate(job) {
			output = append(output, job)
		}
	}
	return
}

// NewJobViewModel returns a job view model from a job scheduler.
func NewJobViewModel(js *cron.JobScheduler) (*JobViewModel, error) {
	typed, ok := js.Job.(*Job)
	if !ok {
		return nil, ex.New("invalid job type; must be a *jobkit.Job")
	}

	var history []*JobInvocation
	var historyLookup map[string]*JobInvocation
	if typed.HistoryProvider != nil {
		var err error
		history, err = typed.HistoryProvider.Get(context.Background(), typed.Name())
		if err != nil {
			return nil, err
		}

		historyLookup = make(map[string]*JobInvocation)
		for _, ji := range history {
			historyLookup[ji.ID] = ji
		}
	}

	current := NewJobInvocation(js.Current())
	last := NewJobInvocation(js.Last())
	return &JobViewModel{
		Name:          typed.Name(),
		Labels:        js.Labels(),
		Disabled:      js.Disabled(),
		Config:        typed.JobConfig,
		Stats:         HistoryStats(history),
		Schedule:      typed.JobSchedule,
		NextRuntime:   js.NextRuntime,
		Current:       current,
		Last:          last,
		History:       history,
		HistoryLookup: historyLookup,
	}, nil
}

// JobViewModel is a viewmodel that represents a job.
type JobViewModel struct {
	Name          string
	Labels        map[string]string
	Disabled      bool
	Config        JobConfig
	Stats         JobStats
	Schedule      cron.Schedule
	NextRuntime   time.Time
	Current       *JobInvocation
	Last          *JobInvocation
	History       []*JobInvocation
	HistoryLookup map[string]*JobInvocation
}
