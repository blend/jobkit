package jobkit

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/blend/go-sdk/cron"
	"github.com/blend/go-sdk/email"
	"github.com/blend/go-sdk/ex"
	"github.com/blend/go-sdk/logger"
	"github.com/blend/go-sdk/mathutil"
	"github.com/blend/go-sdk/r2"
	"github.com/blend/go-sdk/sentry"
	"github.com/blend/go-sdk/slack"
	"github.com/blend/go-sdk/stats"
	"github.com/blend/go-sdk/stringutil"
)

var (
	_ cron.Job               = (*Job)(nil)
	_ cron.ScheduleProvider  = (*Job)(nil)
	_ cron.ConfigProvider    = (*Job)(nil)
	_ cron.LifecycleProvider = (*Job)(nil)
)

// MustNewJob returns a new job with a given set of options and panics if ther
// is an error.
func MustNewJob(wrapped cron.Job, options ...JobOption) *Job {
	job, err := NewJob(wrapped, options...)
	if err != nil {
		panic(err)
	}
	return job
}

// NewJob returns a new job.
func NewJob(wrapped cron.Job, options ...JobOption) (*Job, error) {
	job := &Job{
		Job:           wrapped,
		HistoryLookup: make(map[string]*JobInvocation),
	}
	if typed, ok := job.Job.(cron.ScheduleProvider); ok {
		job.JobSchedule = typed.Schedule()
	}
	var err error
	for _, opt := range options {
		if err = opt(job); err != nil {
			return nil, err
		}
	}
	return job, nil
}

// OptJobConfig sets the job config.
func OptJobConfig(cfg JobConfig) JobOption {
	return func(job *Job) error {
		job.JobConfig = cfg
		return nil
	}
}

// OptJobParsedSchedule sets the schedule to a parsed cron string.
func OptJobParsedSchedule(schedule string) JobOption {
	return func(job *Job) error {
		schedule, err := cron.ParseString(schedule)
		if err != nil {
			return err
		}
		job.JobSchedule = schedule
		return nil
	}
}

// JobOption is a function that mutates a job.
type JobOption func(*Job) error

// Job is the main job body.
type Job struct {
	sync.Mutex

	Job         cron.Job
	JobConfig   JobConfig
	JobSchedule cron.Schedule

	Log         logger.Log
	StatsClient stats.Collector

	SlackDefaults   slack.Message
	EmailDefaults   email.Message
	WebhookDefaults Webhook

	SlackClient  slack.Sender
	SentryClient sentry.Sender
	EmailClient  email.Sender

	NotificationsQueueEmail   *RetryQueue
	NotificationsQueueSlack   *RetryQueue
	NotificationsQueueWebhook *RetryQueue

	History       []*JobInvocation
	HistoryLookup map[string]*JobInvocation

	HistoryProvider HistoryProvider
}

// Name returns the job name.
func (job *Job) Name() string {
	return job.Job.Name()
}

// Schedule returns the job schedule.
func (job *Job) Schedule() cron.Schedule {
	return job.JobSchedule
}

// Config implements job config provider.
func (job *Job) Config() cron.JobConfig {
	var cfg cron.JobConfig
	if typed, ok := job.Job.(cron.ConfigProvider); ok {
		cfg = typed.Config()
	}
	return cfg
}

// Lifecycle implements cron.LifecycleProvider.
func (job *Job) Lifecycle() (output cron.JobLifecycle) {
	var innerLifecycle cron.JobLifecycle
	if typed, ok := job.Job.(cron.LifecycleProvider); ok {
		innerLifecycle = typed.Lifecycle()
	}

	output.OnLoad = func(ctx context.Context) error {
		if err := job.OnLoad(ctx); err != nil {
			return err
		}
		if innerLifecycle.OnLoad != nil {
			if err := innerLifecycle.OnLoad(ctx); err != nil {
				return err
			}
		}
		return nil
	}
	output.OnUnload = func(ctx context.Context) error {
		if err := job.OnUnload(ctx); err != nil {
			return err
		}
		if innerLifecycle.OnUnload != nil {
			if err := innerLifecycle.OnUnload(ctx); err != nil {
				return err
			}
		}
		return nil
	}

	output.OnBegin = func(ctx context.Context) {
		job.OnBegin(ctx)
		if innerLifecycle.OnBegin != nil {
			innerLifecycle.OnBegin(ctx)
		}
	}
	output.OnComplete = func(ctx context.Context) {
		job.OnComplete(ctx)
		if innerLifecycle.OnComplete != nil {
			innerLifecycle.OnComplete(ctx)
		}
	}
	output.OnSuccess = func(ctx context.Context) {
		job.OnSuccess(ctx)
		if innerLifecycle.OnSuccess != nil {
			innerLifecycle.OnSuccess(ctx)
		}
	}
	output.OnError = func(ctx context.Context) {
		job.OnError(ctx)
		if innerLifecycle.OnError != nil {
			innerLifecycle.OnError(ctx)
		}
	}
	output.OnCancellation = func(ctx context.Context) {
		job.OnCancellation(ctx)
		if innerLifecycle.OnCancellation != nil {
			innerLifecycle.OnCancellation(ctx)
		}
	}
	output.OnBroken = func(ctx context.Context) {
		job.OnBroken(ctx)
		if innerLifecycle.OnBroken != nil {
			innerLifecycle.OnBroken(ctx)
		}
	}
	output.OnFixed = func(ctx context.Context) {
		job.OnFixed(ctx)
		if innerLifecycle.OnFixed != nil {
			innerLifecycle.OnFixed(ctx)
		}
	}
	output.OnEnabled = func(ctx context.Context) {
		job.OnEnabled(ctx)
		if innerLifecycle.OnEnabled != nil {
			innerLifecycle.OnEnabled(ctx)
		}
	}
	output.OnDisabled = func(ctx context.Context) {
		job.OnDisabled(ctx)
		if innerLifecycle.OnDisabled != nil {
			innerLifecycle.OnDisabled(ctx)
		}
	}

	return
}

// OnLoad implements job on load handler.
func (job *Job) OnLoad(ctx context.Context) error {
	if err := job.RestoreHistory(ctx); err != nil {
		return err
	}

	job.NotificationsQueueEmail = NewRetryQueue(job.notifyEmail)
	job.NotificationsQueueEmail.Log = job.Log
	go job.NotificationsQueueEmail.Start()
	<-job.NotificationsQueueEmail.NotifyStarted()

	job.NotificationsQueueSlack = NewRetryQueue(job.notifySlack)
	job.NotificationsQueueSlack.Log = job.Log
	go job.NotificationsQueueSlack.Start()
	<-job.NotificationsQueueSlack.NotifyStarted()

	job.NotificationsQueueWebhook = NewRetryQueue(job.notifyWebhook)
	job.NotificationsQueueWebhook.Log = job.Log
	go job.NotificationsQueueWebhook.Start()
	<-job.NotificationsQueueWebhook.NotifyStarted()

	return nil
}

// OnUnload implements job on unload handler.
func (job *Job) OnUnload(ctx context.Context) error {
	if job.NotificationsQueueEmail != nil {
		job.NotificationsQueueEmail.Stop()
	}
	if job.NotificationsQueueSlack != nil {
		job.NotificationsQueueSlack.Stop()
	}
	if job.NotificationsQueueWebhook != nil {
		job.NotificationsQueueWebhook.Stop()
	}
	return nil
}

// OnBegin is a lifecycle event handler.
func (job *Job) OnBegin(ctx context.Context) {
	job.sendStats(ctx, cron.FlagBegin)
	if job.JobConfig.Notifications.OnBeginOrDefault() {
		job.notify(ctx, cron.FlagBegin)
	}
}

// OnSuccess is a lifecycle event handler.
func (job *Job) OnSuccess(ctx context.Context) {
	job.sendStats(ctx, cron.FlagSuccess)
	if job.JobConfig.Notifications.OnSuccessOrDefault() {
		job.notify(ctx, cron.FlagSuccess)
	}
}

// OnComplete is a lifecycle event handler.
func (job *Job) OnComplete(ctx context.Context) {
	// SPECIAL ON COMPLETE STEPS!
	job.AddHistoryResult(NewJobInvocation(cron.GetJobInvocation(ctx)))
	job.PersistHistory(ctx)

	job.sendStats(ctx, cron.FlagComplete)
	if job.JobConfig.Notifications.OnCompleteOrDefault() {
		job.notify(ctx, cron.FlagComplete)
	}
}

// OnError is a lifecycle event handler.
func (job *Job) OnError(ctx context.Context) {
	job.sendStats(ctx, cron.FlagErrored)
	if job.JobConfig.Notifications.OnErrorOrDefault() {
		job.notify(ctx, cron.FlagErrored)
	}
}

// OnBroken is a lifecycle event handler.
func (job *Job) OnBroken(ctx context.Context) {
	job.sendStats(ctx, cron.FlagBroken)
	if job.JobConfig.Notifications.OnBrokenOrDefault() {
		job.notify(ctx, cron.FlagBroken)
	}
}

// OnFixed is a lifecycle event handler.
func (job *Job) OnFixed(ctx context.Context) {
	job.sendStats(ctx, cron.FlagFixed)
	if job.JobConfig.Notifications.OnFixedOrDefault() {
		job.notify(ctx, cron.FlagFixed)
	}
}

// OnCancellation is a lifecycle event handler.
func (job *Job) OnCancellation(ctx context.Context) {
	job.sendStats(ctx, cron.FlagCancelled)
	if job.JobConfig.Notifications.OnCancellationOrDefault() {
		job.notify(ctx, cron.FlagCancelled)
	}
}

// OnEnabled is a lifecycle event handler.
func (job *Job) OnEnabled(ctx context.Context) {
	job.sendStats(ctx, cron.FlagEnabled)
	if job.JobConfig.Notifications.OnEnabledOrDefault() {
		job.notify(ctx, cron.FlagEnabled)
	}
}

// OnDisabled is a lifecycle event handler.
func (job *Job) OnDisabled(ctx context.Context) {
	job.sendStats(ctx, cron.FlagDisabled)
	if job.JobConfig.Notifications.OnDisabledOrDefault() {
		job.notify(ctx, cron.FlagDisabled)
	}
}

// GetJobInvocationByID returns a job invocation by id.
func (job *Job) GetJobInvocationByID(invocationID string) *JobInvocation {
	job.Lock()
	defer job.Unlock()

	if ji, ok := job.HistoryLookup[invocationID]; ok {
		return ji
	}
	return nil
}

// Execute is the job body.
func (job *Job) Execute(ctx context.Context) (err error) {
	invocationOutput := NewJobInvocationOutput()

	ctx = WithJobInvocationOutput(ctx, invocationOutput)
	ji := cron.GetJobInvocation(ctx)
	ji.State = invocationOutput

	if err = job.Job.Execute(ctx); err != nil {
		return
	}
	return
}

//
// exported utility methods
//

// Debugf logs a debug message if the logger is set.
func (job *Job) Debugf(ctx context.Context, format string, args ...interface{}) {
	if job.Log != nil {
		job.Log.WithContext(ctx).Debugf(format, args...)
	}
}

// Error logs an error if the logger i set.
func (job *Job) Error(ctx context.Context, err error) error {
	if job.Log != nil && err != nil {
		job.Log.WithContext(ctx).Error(err)
	}
	return err
}

//
// private utility methods
//

func (job *Job) sendStats(ctx context.Context, flag string) {
	if job.StatsClient != nil {
		job.StatsClient.Increment(string(flag), fmt.Sprintf("%s:%s", stats.TagJob, job.Name()))
		if ji := cron.GetJobInvocation(ctx); ji != nil {
			job.Debugf(ctx, "stats; sending stats to collector")
			job.Error(ctx, job.StatsClient.TimeInMilliseconds(string(flag), ji.Elapsed(), fmt.Sprintf("%s:%s", stats.TagJob, job.Name())))
		}
	} else {
		job.Debugf(ctx, "stats; client unset, skipping logging stats")
	}
}

func (job *Job) notifySlack(ctx context.Context, item interface{}) error {
	if ji := cron.GetJobInvocation(ctx); ji != nil {
		job.Debugf(ctx, "notify (slack); sending slack notification")
		return job.SlackClient.Send(context.Background(), item.(slack.Message))
	}
	return nil
}

func (job *Job) notifyEmail(ctx context.Context, item interface{}) error {
	if ji := cron.GetJobInvocation(ctx); ji != nil {
		message, ok := item.(email.Message)
		if !ok {
			return ex.New("notify (email); invalid work item; not a `email.Message`")
		}
		job.Debugf(ctx, "notify (email); sending email notification to %s (%s)", stringutil.CSV(message.To), message.Subject)
		return job.EmailClient.Send(context.Background(), message)
	}
	return nil
}

func (job *Job) notifyWebhook(ctx context.Context, _ interface{}) error {
	job.Debugf(ctx, "notify (webhook); sending webhook notification")
	res, err := job.WebhookDefaults.Request(r2.OptLog(job.Log)).Discard()
	if err != nil {
		return err
	}
	if res.StatusCode > 299 {
		return fmt.Errorf("non-200 returned from remote")
	}
	return nil
}

func (job *Job) notify(ctx context.Context, flag string) {
	ji := NewJobInvocation(cron.GetJobInvocation(ctx))

	if job.SlackClient != nil {
		if ji != nil {
			message := NewSlackMessage(flag, job.SlackDefaults, ji)
			if job.NotificationsQueueSlack != nil && job.NotificationsQueueSlack.Latch.IsStarted() {
				job.Debugf(ctx, "notify (slack); queueing slack notification")
				job.NotificationsQueueSlack.Add(ctx, message)
			} else {
				job.Debugf(ctx, "notify (slack); sending slack notification")
				job.Error(ctx, job.SlackClient.Send(ctx, message))
			}
		}
	} else {
		job.Debugf(ctx, "notify (slack); sender unset skipping queuing slack notification")
	}

	if job.EmailClient != nil {
		if ji != nil {
			message, err := NewEmailMessage(flag, job.EmailDefaults, ji)
			if err != nil {
				job.Error(ctx, err)
			}
			if job.NotificationsQueueEmail != nil && job.NotificationsQueueEmail.Latch.IsStarted() {
				job.Debugf(ctx, "notify (email); queueing email notification")
				job.NotificationsQueueEmail.Add(ctx, message)
			} else {
				job.Debugf(ctx, "notify (email); sending email notification")
				job.Error(ctx, job.EmailClient.Send(ctx, message))
			}
		}
	} else {
		job.Debugf(ctx, "notify (email); sender unset, skipping sending email notification")
	}

	if !job.WebhookDefaults.IsZero() {
		if job.NotificationsQueueEmail != nil && job.NotificationsQueueEmail.Latch.IsStarted() {
			job.Debugf(ctx, "notify (webhook); queueing webhook notification")
			job.NotificationsQueueWebhook.Add(ctx, flag)
		} else {
			job.Error(ctx, job.notifyWebhook(ctx, flag))
		}
	} else {
		job.Debugf(ctx, "notify (webhook); sender unset, skipping sending webhook notification")
	}
}

//
// history utils
//

// RestoreHistory calls the persist handler if it's set.
func (job *Job) RestoreHistory(ctx context.Context) error {
	if job.JobConfig.HistoryDisabledOrDefault() {
		return nil
	}
	if job.JobConfig.HistoryPersistenceDisabledOrDefault() {
		return nil
	}
	if job.HistoryProvider == nil {
		return nil
	}

	job.Lock()
	defer job.Unlock()

	logger.MaybeDebugf(job.Log, "restoring history")
	var err error
	if job.History, err = job.HistoryProvider.RestoreHistory(ctx); err != nil {
		return job.Error(ctx, err)
	}
	return nil
}

// PersistHistory calls the persist handler if it's set.
func (job *Job) PersistHistory(ctx context.Context) error {
	if job.JobConfig.HistoryDisabledOrDefault() {
		return nil
	}
	if job.JobConfig.HistoryPersistenceDisabledOrDefault() {
		return nil
	}
	if job.HistoryProvider == nil {
		return nil
	}

	job.Lock()
	defer job.Unlock()
	logger.MaybeDebugf(job.Log, "persisting history")

	historyCopy := make([]*JobInvocation, len(job.History))
	copy(historyCopy, job.History)
	if err := job.HistoryProvider.PersistHistory(ctx, historyCopy); err != nil {
		return job.Error(ctx, err)
	}
	return nil
}

// AddHistoryResult adds an item to history and culls old items.
func (job *Job) AddHistoryResult(ji *JobInvocation) {
	job.Lock()
	defer job.Unlock()

	if ji == nil {
		panic("AddHistoryResult; passed a nil job invocation; what was missing?")
	}

	job.History = append(job.History, ji)
	job.HistoryLookup[ji.ID] = ji

	count := len(job.History)
	maxCount := job.JobConfig.HistoryMaxCountOrDefault()
	maxAge := job.JobConfig.HistoryMaxAgeOrDefault()

	now := time.Now().UTC()
	var filtered []*JobInvocation
	var removed []string
	for index, h := range job.History {
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
		delete(job.HistoryLookup, id)
	}
	job.History = filtered

}

// Stats returns job stats.
func (job *Job) Stats() JobStats {
	output := JobStats{
		Name:      job.Name(),
		RunsTotal: len(job.History),
	}

	var elapsedTimes []time.Duration
	for _, ji := range job.History {
		switch ji.Status {
		case cron.JobInvocationStatusSuccess:
			output.RunsSuccessful++
		case cron.JobInvocationStatusErrored:
			output.RunsErrored++
		case cron.JobInvocationStatusCancelled:
			output.RunsCancelled++
		}

		elapsedTimes = append(elapsedTimes, ji.JobInvocation.Elapsed())
		if ji.JobInvocation.Elapsed() > output.ElapsedMax {
			output.ElapsedMax = ji.JobInvocation.Elapsed()
		}
		if ji.JobInvocationOutput.Output != nil {
			output.OutputBytes += len(ji.JobInvocationOutput.Output.Bytes())
		}
	}
	if output.RunsTotal > 0 {
		output.SuccessRate = float64(output.RunsSuccessful) / float64(output.RunsTotal)
	}
	output.Elapsed50th = mathutil.PercentileOfDuration(elapsedTimes, 50.0)
	output.Elapsed95th = mathutil.PercentileOfDuration(elapsedTimes, 95.0)
	return output
}
