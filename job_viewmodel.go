package jobkit

import (
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
	return &JobViewModel{
		Name:          typed.Name(),
		Labels:        js.Labels(),
		Config:        typed.JobConfig,
		Stats:         typed.Stats(),
		Schedule:      typed.JobSchedule,
		NextRuntime:   js.NextRuntime,
		Current:       NewJobInvocation(js.Current),
		Last:          NewJobInvocation(js.Last),
		History:       typed.History,
		HistoryLookup: typed.HistoryLookup,
	}, nil
}

// JobViewModel is a viewmodel that represents a job.
type JobViewModel struct {
	Name          string
	Labels        map[string]string
	Config        JobConfig
	Stats         JobStats
	Schedule      cron.Schedule
	NextRuntime   time.Time
	Current       *JobInvocation
	Last          *JobInvocation
	History       []*JobInvocation
	HistoryLookup map[string]*JobInvocation
}
