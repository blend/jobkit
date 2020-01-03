package jobkit

import (
	"context"
	"fmt"
	"time"

	"github.com/blend/go-sdk/cron"
	"github.com/blend/go-sdk/email"
	"github.com/blend/go-sdk/ex"
	"github.com/blend/go-sdk/logger"
	"github.com/blend/go-sdk/sentry"
	"github.com/blend/go-sdk/slack"
	"github.com/blend/go-sdk/stats"
	"github.com/blend/go-sdk/stringutil"
)

var (
	_ cron.Job                   = (*Job)(nil)
	_ cron.ScheduleProvider      = (*Job)(nil)
	_ cron.OnLoadHandler         = (*Job)(nil)
	_ cron.OnUnloadHandler       = (*Job)(nil)
	_ cron.LabelsProvider        = (*Job)(nil)
	_ cron.JobConfigProvider     = (*Job)(nil)
	_ cron.ScheduleProvider      = (*Job)(nil)
	_ cron.OnBeginHandler        = (*Job)(nil)
	_ cron.OnCompleteHandler     = (*Job)(nil)
	_ cron.OnFailureHandler      = (*Job)(nil)
	_ cron.OnCancellationHandler = (*Job)(nil)
	_ cron.OnBrokenHandler       = (*Job)(nil)
	_ cron.OnFixedHandler        = (*Job)(nil)
	_ cron.OnDisabledHandler     = (*Job)(nil)
	_ cron.OnEnabledHandler      = (*Job)(nil)
	_ cron.HistoryProvider       = (*Job)(nil)
)

// NewJob returns a new job.
func NewJob(cfg JobConfig, action func(context.Context) error, options ...JobOption) (*Job, error) {
	retryQueueOptions := []RetryQueueOption{
		OptRetryQueueMaxAttempts(128),
		OptRetryQueueRetryWait(5 * time.Second),
	}

	var job Job
	job.HistoryProvider = HistoryJSON{cfg}
	job.NotificationsEmail = NewRetryQueue(job.notifyEmail, retryQueueOptions...)
	job.NotificationsSlack = NewRetryQueue(job.notifySlack, retryQueueOptions...)
	job.NotificationsWebhook = NewRetryQueue(job.notifyWebhook, retryQueueOptions...)

	options = append([]JobOption{
		OptConfig(cfg),
		OptAction(action),
		OptParsedSchedule(cfg.ScheduleOrDefault()),
	}, options...)

	var err error
	for _, opt := range options {
		if err = opt(&job); err != nil {
			return nil, err
		}
	}
	return &job, nil
}

// WrapJob wraps a cron job with the jobkit notifications.
func WrapJob(job cron.Job) *Job {
	var j Job
	j.Config.Name = job.Name()
	j.Action = job.Execute
	j.NotificationsEmail = NewRetryQueue(j.notifyEmail)
	j.NotificationsSlack = NewRetryQueue(j.notifySlack)
	j.NotificationsWebhook = NewRetryQueue(j.notifyWebhook)

	if typed, ok := job.(cron.JobConfigProvider); ok {
		j.Config.JobConfig = typed.JobConfig()
	}
	if typed, ok := job.(cron.ScheduleProvider); ok {
		j.CompiledSchedule = typed.Schedule()
	}
	return &j
}

// OptAction sets the job action.
func OptAction(action func(context.Context) error) JobOption {
	return func(job *Job) error {
		job.Action = action
		return nil
	}
}

// OptConfig sets the job config.
func OptConfig(cfg JobConfig) JobOption {
	return func(job *Job) error {
		job.Config = cfg
		return nil
	}
}

// OptParsedSchedule sets the job's compiled schedule from a schedule string.
func OptParsedSchedule(schedule string) JobOption {
	return func(job *Job) error {
		schedule, err := cron.ParseString(schedule)
		if err != nil {
			return err
		}
		job.CompiledSchedule = schedule
		return nil
	}
}

// JobOption is an option for jobs.
type JobOption func(*Job) error

// Job is the main job body.
type Job struct {
	Config JobConfig

	CompiledSchedule cron.Schedule
	Action           func(context.Context) error
	HistoryProvider  cron.HistoryProvider

	Log         logger.Log
	StatsClient stats.Collector

	EmailDefaults   email.Message
	WebhookDefaults Webhook

	SlackClient  slack.Sender
	SentryClient sentry.Sender
	EmailClient  email.Sender

	NotificationsEmail   *RetryQueue
	NotificationsSlack   *RetryQueue
	NotificationsWebhook *RetryQueue
}

// Name returns the job name.
func (job Job) Name() string {
	return job.Config.Name
}

// Labels returns the job labels.
func (job Job) Labels() map[string]string {
	return job.Config.Labels
}

// Schedule returns the job schedule.
func (job Job) Schedule() cron.Schedule {
	return job.CompiledSchedule
}

// JobConfig implements job config provider.
func (job Job) JobConfig() cron.JobConfig {
	return job.Config.JobConfig
}

// OnLoad implements job on load handler.
func (job Job) OnLoad() error {
	job.NotificationsEmail.Log = job.Log
	go job.NotificationsEmail.Start()
	<-job.NotificationsEmail.NotifyStarted()

	job.NotificationsSlack.Log = job.Log
	go job.NotificationsSlack.Start()
	<-job.NotificationsSlack.NotifyStarted()

	job.NotificationsWebhook.Log = job.Log
	go job.NotificationsWebhook.Start()
	<-job.NotificationsWebhook.NotifyStarted()

	return nil
}

// OnUnload implements job on unload handler.
func (job Job) OnUnload() error {
	job.NotificationsEmail.Stop()
	job.NotificationsSlack.Stop()
	job.NotificationsWebhook.Stop()
	return nil
}

// OnBegin is a lifecycle event handler.
func (job Job) OnBegin(ctx context.Context) {
	job.stats(ctx, cron.FlagBegin)
	if job.Config.Notifications.OnBeginOrDefault() {
		job.notify(ctx, cron.FlagBegin)
	}
}

// OnComplete is a lifecycle event handler.
func (job Job) OnComplete(ctx context.Context) {
	job.stats(ctx, cron.FlagComplete)
	if job.Config.Notifications.OnSuccessOrDefault() {
		job.notify(ctx, cron.FlagComplete)
	}
}

// OnFailure is a lifecycle event handler.
func (job Job) OnFailure(ctx context.Context) {
	job.stats(ctx, cron.FlagFailed)
	if job.Config.Notifications.OnFailureOrDefault() {
		job.notify(ctx, cron.FlagFailed)
	}
}

// OnBroken is a lifecycle event handler.
func (job Job) OnBroken(ctx context.Context) {
	job.stats(ctx, cron.FlagBroken)
	if job.Config.Notifications.OnBrokenOrDefault() {
		job.notify(ctx, cron.FlagBroken)
	}
}

// OnFixed is a lifecycle event handler.
func (job Job) OnFixed(ctx context.Context) {
	job.stats(ctx, cron.FlagFixed)
	if job.Config.Notifications.OnFixedOrDefault() {
		job.notify(ctx, cron.FlagFixed)
	}
}

// OnCancellation is a lifecycle event handler.
func (job Job) OnCancellation(ctx context.Context) {
	job.stats(ctx, cron.FlagCancelled)
	if job.Config.Notifications.OnCancellationOrDefault() {
		job.notify(ctx, cron.FlagCancelled)
	}
}

// OnEnabled is a lifecycle event handler.
func (job Job) OnEnabled(ctx context.Context) {
	if job.Config.Notifications.OnEnabledOrDefault() {
		job.notify(ctx, cron.FlagEnabled)
	}
}

// OnDisabled is a lifecycle event handler.
func (job Job) OnDisabled(ctx context.Context) {
	if job.Config.Notifications.OnDisabledOrDefault() {
		job.notify(ctx, cron.FlagDisabled)
	}
}

// PersistHistory writes the history to disk.
// It does so completely, overwriting any existing history file on disk with the complete history.
func (job Job) PersistHistory(ctx context.Context, log []cron.JobInvocation) error {
	if job.HistoryProvider != nil {
		return job.HistoryProvider.PersistHistory(ctx, log)
	}
	return nil
}

// RestoreHistory restores history from disk.
func (job Job) RestoreHistory(ctx context.Context) (output []cron.JobInvocation, err error) {
	if job.HistoryProvider != nil {
		output, err = job.HistoryProvider.RestoreHistory(ctx)
		return
	}
	return
}

// Execute is the job body.
func (job Job) Execute(ctx context.Context) error {
	return job.Action(ctx)
}

//
// exported utility methods
//

// Debugf logs a debug message if the logger is set.
func (job Job) Debugf(ctx context.Context, format string, args ...interface{}) {
	if job.Log != nil {
		job.Log.WithPath("cron", job.Name(), cron.GetJobInvocation(ctx).ID).WithContext(ctx).Debugf(format, args...)
	}
}

// Error logs an error if the logger i set.
func (job Job) Error(ctx context.Context, err error) error {
	if job.Log != nil && err != nil {
		job.Log.WithPath("cron", job.Name(), cron.GetJobInvocation(ctx).ID).WithContext(ctx).Error(err)
	}
	return err

}

//
// private utility methods
//

func (job Job) stats(ctx context.Context, flag string) {
	if job.StatsClient != nil {
		job.StatsClient.Increment(string(flag), fmt.Sprintf("%s:%s", stats.TagJob, job.Name()))
		if ji := cron.GetJobInvocation(ctx); ji != nil {
			job.Debugf(ctx, "stats; sending stats to collector")
			job.Error(ctx, job.StatsClient.TimeInMilliseconds(string(flag), ji.Elapsed, fmt.Sprintf("%s:%s", stats.TagJob, job.Name())))
		}
	} else {
		job.Debugf(ctx, "stats client unset, skipping logging stats")
	}
}

func (job Job) notifySlack(ctx context.Context, item interface{}) error {
	if ji := cron.GetJobInvocation(ctx); ji != nil {
		job.Debugf(ctx, "notify (slack); sending slack notification")
		return job.SlackClient.Send(context.Background(), item.(slack.Message))
	}
	return nil
}

func (job Job) notifyEmail(ctx context.Context, item interface{}) error {
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

func (job Job) notifyWebhook(ctx context.Context, item interface{}) error {
	job.Debugf(ctx, "notify (webhook); sending webhook notification")
	_, err := job.WebhookDefaults.Request().Discard()
	return err
}

func (job Job) notify(ctx context.Context, flag string) {
	if job.SlackClient != nil {
		if ji := cron.GetJobInvocation(ctx); ji != nil {
			job.Debugf(ctx, "notify (slack); queueing slack notification")
			message := NewSlackMessage(flag, ji)
			if job.NotificationsSlack != nil && job.NotificationsSlack.Latch.IsStarted() {
				job.NotificationsSlack.Add(context.Background(), message)
			} else {
				job.Error(ctx, job.SlackClient.Send(context.Background(), message))
			}
		}
	} else {
		job.Debugf(ctx, "notify (slack); sender unset skipping queuing slack notification")
	}

	if job.EmailClient != nil {
		if ji := cron.GetJobInvocation(ctx); ji != nil {
			job.Debugf(ctx, "notify (email); queueing email notification")
			message, err := NewEmailMessage(flag, job.EmailDefaults, ji)
			if err != nil {
				job.Error(ctx, err)
			}

			if job.NotificationsEmail != nil && job.NotificationsEmail.Latch.IsStarted() {
				job.NotificationsEmail.Add(context.Background(), message)
			} else {
				job.Error(ctx, job.EmailClient.Send(context.Background(), message))
			}
		}
	} else {
		job.Debugf(ctx, "notify (email); email sender unset, skipping sending email notification")
	}

	if !job.WebhookDefaults.IsZero() {
		job.Debugf(ctx, "notify (webhook); queueing webhook notification")
		if job.NotificationsEmail != nil && job.NotificationsEmail.Latch.IsStarted() {
			job.NotificationsWebhook.Add(context.Background(), flag)
		} else {
			job.Error(ctx, job.notifyWebhook(context.Background(), flag))
		}
	} else {
		job.Debugf(ctx, "notify (webhook); webhook sender unset, skipping sending webhook notification")
	}
}
