package jobkit

import (
	"sort"
	"time"

	"github.com/blend/go-sdk/cron"
)

// NewJobViewModels returns the job view models.
func NewJobViewModels(jobs map[string]*cron.JobScheduler) (output []*JobViewModel) {
	var jobSchedulers []*cron.JobScheduler
	for _, jobScheduler := range jobs {
		jobSchedulers = append(jobSchedulers, jobScheduler)
	}
	sort.Sort(cron.JobSchedulersByJobNameAsc(jobSchedulers))
	for _, jobScheduler := range jobSchedulers {
		output = append(output, NewJobViewModel(jobScheduler))
	}
	return
}

// NewJobViewModel returns a job view model from a job scheduler.
func NewJobViewModel(js *cron.JobScheduler) *JobViewModel {
	typed, ok := js.Job.(*Job)
	if !ok {
		panic("NewJobViewModel; job scheduler job is not a *Job")
	}
	return &JobViewModel{
		Name:          typed.Name(),
		Config:        typed.JobConfig,
		Stats:         typed.Stats(),
		Schedule:      typed.JobSchedule,
		NextRuntime:   js.NextRuntime,
		Current:       NewJobInvocation(js.Current),
		Last:          NewJobInvocation(js.Last),
		History:       typed.History,
		HistoryLookup: typed.HistoryLookup,
	}
}

// JobViewModel is a viewmodel that represents a job.
type JobViewModel struct {
	Name          string
	Config        JobConfig
	Stats         JobStats
	Schedule      cron.Schedule
	NextRuntime   time.Time
	Current       *JobInvocation
	Last          *JobInvocation
	History       []*JobInvocation
	HistoryLookup map[string]*JobInvocation
}
