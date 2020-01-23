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

// NewJob returns a new job.
func NewJob(wrapped cron.Job, options ...JobOption) (*Job, error) {
	job := &Job{
		Job: wrapped,
	}
	var err error
	for _, opt := range options {
		if err = opt(job); err != nil {
			return nil, err
		}
	}
	return job, nil
}

// JobOption is a function that mutates a job.
type JobOption func(*Job) error

// Job is the main job body.
type Job struct {
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

	HistoryMux    sync.Mutex
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
	if typed, ok := job.Job.(cron.ScheduleProvider); ok {
		return typed.Schedule()
	}
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

	output.OnLoad = func() error {
		if err := job.OnLoad(); err != nil {
			return err
		}
		if innerLifecycle.OnLoad != nil {
			if err := innerLifecycle.OnLoad(); err != nil {
				return err
			}
		}
		return nil
	}
	output.OnUnload = func() error {
		if err := job.OnUnload(); err != nil {
			return err
		}
		if innerLifecycle.OnUnload != nil {
			if err := innerLifecycle.OnUnload(); err != nil {
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
func (job *Job) OnLoad() error {
	logger.MaybeDebugf(job.Log, "on load: restoring history")
	if err := job.RestoreHistory(context.Background()); err != nil {
		return err
	}

	logger.MaybeDebugf(job.Log, "on load: starting retry queue workers")
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
func (job *Job) OnUnload() error {
	job.NotificationsQueueEmail.Stop()
	job.NotificationsQueueSlack.Stop()
	job.NotificationsQueueWebhook.Stop()
	return nil
}

// OnBegin is a lifecycle event handler.
func (job Job) OnBegin(ctx context.Context) {
	job.sendStats(ctx, cron.FlagBegin)
	if job.JobConfig.Notifications.OnBeginOrDefault() {
		job.notify(ctx, cron.FlagBegin)
	}
}

// OnSuccess is a lifecycle event handler.
func (job Job) OnSuccess(ctx context.Context) {
	job.sendStats(ctx, cron.FlagSuccess)
	if job.JobConfig.Notifications.OnSuccessOrDefault() {
		job.notify(ctx, cron.FlagSuccess)
	}
}

// OnComplete is a lifecycle event handler.
func (job Job) OnComplete(ctx context.Context) {
	job.sendStats(ctx, cron.FlagComplete)
	if job.JobConfig.Notifications.OnCompleteOrDefault() {
		job.notify(ctx, cron.FlagComplete)
	}
}

// OnError is a lifecycle event handler.
func (job Job) OnError(ctx context.Context) {
	job.sendStats(ctx, cron.FlagErrored)
	if job.JobConfig.Notifications.OnErrorOrDefault() {
		job.notify(ctx, cron.FlagErrored)
	}
}

// OnBroken is a lifecycle event handler.
func (job Job) OnBroken(ctx context.Context) {
	job.sendStats(ctx, cron.FlagBroken)
	if job.JobConfig.Notifications.OnBrokenOrDefault() {
		job.notify(ctx, cron.FlagBroken)
	}
}

// OnFixed is a lifecycle event handler.
func (job Job) OnFixed(ctx context.Context) {
	job.sendStats(ctx, cron.FlagFixed)
	if job.JobConfig.Notifications.OnFixedOrDefault() {
		job.notify(ctx, cron.FlagFixed)
	}
}

// OnCancellation is a lifecycle event handler.
func (job Job) OnCancellation(ctx context.Context) {
	job.sendStats(ctx, cron.FlagCancelled)
	if job.JobConfig.Notifications.OnCancellationOrDefault() {
		job.notify(ctx, cron.FlagCancelled)
	}
}

// OnEnabled is a lifecycle event handler.
func (job Job) OnEnabled(ctx context.Context) {
	job.sendStats(ctx, cron.FlagEnabled)
	if job.JobConfig.Notifications.OnEnabledOrDefault() {
		job.notify(ctx, cron.FlagEnabled)
	}
}

// OnDisabled is a lifecycle event handler.
func (job Job) OnDisabled(ctx context.Context) {
	job.sendStats(ctx, cron.FlagDisabled)
	if job.JobConfig.Notifications.OnDisabledOrDefault() {
		job.notify(ctx, cron.FlagDisabled)
	}
}

// GetJobInvocationByID returns a job invocation by id.
func (job *Job) GetJobInvocationByID(invocationID string) *JobInvocation {
	job.HistoryMux.Lock()
	defer job.HistoryMux.Unlock()
	if ji, ok := job.HistoryLookup[invocationID]; ok {
		return ji
	}
	return nil
}

// Execute is the job body.
func (job *Job) Execute(ctx context.Context) (err error) {
	invocationOutput := NewJobInvocationOutput()
	ctx = WithJobInvocationOutput(ctx, invocationOutput)
	if err = job.Job.Execute(ctx); err != nil {
		return
	}
	job.AddHistory(NewJobInvocation(cron.GetJobInvocation(ctx), invocationOutput))
	job.CullHistory()

	if err = job.PersistHistory(ctx); err != nil {
		return
	}
	return
}

//
// exported utility methods
//

// Debugf logs a debug message if the logger is set.
func (job Job) Debugf(ctx context.Context, format string, args ...interface{}) {
	if job.Log != nil {
		job.Log.WithContext(ctx).Debugf(format, args...)
	}
}

// Error logs an error if the logger i set.
func (job Job) Error(ctx context.Context, err error) error {
	if job.Log != nil && err != nil {
		job.Log.WithContext(ctx).Error(err)
	}
	return err
}

//
// private utility methods
//

func (job Job) sendStats(ctx context.Context, flag string) {
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
	ji := cron.GetJobInvocation(ctx)
	jio := GetJobInvocationOutput(ctx)
	invocation := NewJobInvocation(ji, jio)

	if job.SlackClient != nil {
		if ji != nil {
			message := NewSlackMessage(flag, job.SlackDefaults, invocation)
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
			message, err := NewEmailMessage(flag, job.EmailDefaults, invocation)
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

	job.HistoryMux.Lock()
	defer job.HistoryMux.Unlock()
	var err error
	if job.History, err = job.HistoryProvider.RestoreHistory(ctx); err != nil {
		return job.Error(ctx, err)
	}
	return nil
}

// PersistHistory calls the persist handler if it's set.
func (job *Job) PersistHistory(ctx context.Context) error {
	if !job.JobConfig.HistoryDisabledOrDefault() {
		return nil
	}
	if !job.JobConfig.HistoryPersistenceDisabledOrDefault() {
		return nil
	}
	if job.HistoryProvider == nil {
		return nil
	}

	job.HistoryMux.Lock()
	defer job.HistoryMux.Unlock()

	historyCopy := make([]*JobInvocation, len(job.History))
	copy(historyCopy, job.History)
	if err := job.HistoryProvider.PersistHistory(ctx, historyCopy); err != nil {
		return job.Error(ctx, err)
	}
	return nil
}

// AddHistory adds an item to history.
func (job *Job) AddHistory(ji *JobInvocation) {
	job.HistoryMux.Lock()
	job.History = append(job.History, ji)
	job.HistoryLookup[ji.JobInvocation.ID] = ji
	job.HistoryMux.Unlock()
}

// CullHistory culls history after the job completes, but before we persist history.
func (job *Job) CullHistory() {
	job.HistoryMux.Lock()
	defer job.HistoryMux.Unlock()

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
		switch ji.JobInvocation.Status {
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
