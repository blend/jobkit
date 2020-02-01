package jobkit

import (
	"context"
	"fmt"

	"github.com/blend/go-sdk/cron"
	"github.com/blend/go-sdk/email"
	"github.com/blend/go-sdk/ex"
	"github.com/blend/go-sdk/logger"
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
		Job: wrapped,
	}
	if typed, ok := wrapped.(cron.ScheduleProvider); ok {
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

// OptJobLog sets the job logger.
func OptJobLog(log logger.Log) JobOption {
	return func(job *Job) error {
		job.Log = log
		return nil
	}
}

// OptJobHistory sets the job history provider.
func OptJobHistory(provider HistoryProvider) JobOption {
	return func(job *Job) error {
		job.HistoryProvider = provider
		return nil
	}
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
// One specific consideration is it merges the inner job's config parameters with the config parameters
// found on the wrapping jobkit job config.
func (job *Job) Config() cron.JobConfig {
	var cfg cron.JobConfig
	if typed, ok := job.Job.(cron.ConfigProvider); ok {
		cfg = typed.Config()
	}
	cfg.ParameterValues = cron.MergeJobParameterValues(cfg.ParameterValues, DefaultParameterValues(job.JobConfig.Parameters...))
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
	retryOptions := []RetryQueueOption{
		OptRetryQueueMaxAttempts(job.JobConfig.Notifications.MaxRetriesOrDefault()),
		OptRetryQueueRetryWait(job.JobConfig.Notifications.RetryWaitOrDefault()),
	}

	job.NotificationsQueueEmail = NewRetryQueue(job.notifyEmail, retryOptions...)
	job.NotificationsQueueEmail.Log = job.Log
	go job.NotificationsQueueEmail.Start()
	<-job.NotificationsQueueEmail.NotifyStarted()

	job.NotificationsQueueSlack = NewRetryQueue(job.notifySlack, retryOptions...)
	job.NotificationsQueueSlack.Log = job.Log
	go job.NotificationsQueueSlack.Start()
	<-job.NotificationsQueueSlack.NotifyStarted()

	job.NotificationsQueueWebhook = NewRetryQueue(job.notifyWebhook, retryOptions...)
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
	if err := job.AddHistoryResult(ctx, NewJobInvocation(cron.GetJobInvocation(ctx))); err != nil {
		job.Error(ctx, err)
	}
	if err := job.CullHistory(ctx); err != nil {
		job.Error(ctx, err)
	}
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

// AddHistoryResult adds an item to history and culls old items.
func (job *Job) AddHistoryResult(ctx context.Context, ji *JobInvocation) error {
	if job.JobConfig.HistoryDisabledOrDefault() ||
		job.HistoryProvider == nil {
		return nil
	}
	return job.HistoryProvider.Add(ctx, ji)
}

// CullHistory triggers the history provider cull.
func (job *Job) CullHistory(ctx context.Context) error {
	if job.JobConfig.HistoryDisabledOrDefault() ||
		job.HistoryProvider == nil {
		return nil
	}
	logger.MaybeDebugfContext(ctx, job.Log, "culling history: %d items %v age", job.JobConfig.HistoryMaxCountOrDefault(), job.JobConfig.HistoryMaxAgeOrDefault())
	return job.HistoryProvider.Cull(ctx, job.Name(), job.JobConfig.HistoryMaxCountOrDefault(), job.JobConfig.HistoryMaxAgeOrDefault())
}
