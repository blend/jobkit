package jobkit

import (
	"context"
	"fmt"

	"github.com/blend/go-sdk/cron"
	"github.com/blend/go-sdk/email"
	"github.com/blend/go-sdk/logger"
	"github.com/blend/go-sdk/ref"
	"github.com/blend/go-sdk/sentry"
	"github.com/blend/go-sdk/slack"
	"github.com/blend/go-sdk/stats"
	"github.com/blend/go-sdk/stringutil"
)

var (
	_ cron.Job                    = (*Job)(nil)
	_ cron.LabelsProvider         = (*Job)(nil)
	_ cron.JobConfigProvider      = (*Job)(nil)
	_ cron.ScheduleProvider       = (*Job)(nil)
	_ cron.OnStartReceiver        = (*Job)(nil)
	_ cron.OnCompleteReceiver     = (*Job)(nil)
	_ cron.OnFailureReceiver      = (*Job)(nil)
	_ cron.OnCancellationReceiver = (*Job)(nil)
	_ cron.OnBrokenReceiver       = (*Job)(nil)
	_ cron.OnFixedReceiver        = (*Job)(nil)
	_ cron.OnDisabledReceiver     = (*Job)(nil)
	_ cron.OnEnabledReceiver      = (*Job)(nil)
	_ cron.HistoryProvider        = (*Job)(nil)
)

// NewJob returns a new job.
func NewJob(cfg JobConfig, action func(context.Context) error, options ...JobOption) (*Job, error) {
	options = append([]JobOption{
		OptConfig(cfg),
		OptAction(action),
		OptParsedSchedule(cfg.ScheduleOrDefault()),
	}, options...)

	var job Job
	var err error
	for _, opt := range options {
		if err = opt(&job); err != nil {
			return nil, err
		}
	}
	job.History = HistoryJSON{cfg}
	return &job, nil
}

// WrapJob wraps a cron job with the jobkit notifications.
func WrapJob(job cron.Job) *Job {
	j := &Job{
		Config: JobConfig{
			JobConfig: cron.JobConfig{
				Name: job.Name(),
			},
		},
		Action: job.Execute,
	}

	if typed, ok := job.(cron.ScheduleProvider); ok {
		j.CompiledSchedule = typed.Schedule()
	}
	if typed, ok := job.(cron.DescriptionProvider); ok {
		j.Config.Description = typed.Description()
	}
	if typed, ok := job.(cron.LabelsProvider); ok {
		j.Config.Labels = typed.Labels()
	}
	if typed, ok := job.(cron.TimeoutProvider); ok {
		j.Config.Timeout = typed.Timeout()
	}
	if typed, ok := job.(cron.ShutdownGracePeriodProvider); ok {
		j.Config.ShutdownGracePeriod = typed.ShutdownGracePeriod()
	}
	if typed, ok := job.(cron.HistoryDisabledProvider); ok {
		j.Config.HistoryDisabled = ref.Bool(typed.HistoryDisabled())
	}
	if typed, ok := job.(cron.HistoryPersistenceEnabledProvider); ok {
		j.Config.HistoryPersistenceEnabled = ref.Bool(typed.HistoryPersistenceEnabled())
	}
	if typed, ok := job.(cron.HistoryMaxCountProvider); ok {
		j.Config.HistoryMaxCount = typed.HistoryMaxCount()
	}
	if typed, ok := job.(cron.HistoryMaxAgeProvider); ok {
		j.Config.HistoryMaxAge = typed.HistoryMaxAge()
	}
	if typed, ok := job.(cron.ShouldSkipLoggerListenersProvider); ok {
		j.Config.ShouldSkipLoggerListeners = ref.Bool(typed.ShouldSkipLoggerListeners())
	}
	if typed, ok := job.(cron.ShouldSkipLoggerOutputProvider); ok {
		j.Config.ShouldSkipLoggerOutput = ref.Bool(typed.ShouldSkipLoggerOutput())
	}
	if typed, ok := job.(cron.HistoryProvider); ok {
		j.History = typed
	}
	return j
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
		job.EmailDefaults = cfg.EmailDefaults
		job.WebhookDefaults = cfg.WebhookDefaults
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

	EmailDefaults   email.Message
	WebhookDefaults Webhook

	Log          logger.Log
	StatsClient  stats.Collector
	SlackClient  slack.Sender
	SentryClient sentry.Sender
	EmailClient  email.Sender

	History cron.HistoryProvider
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

// JobConfig returns the underlying job config.
func (job Job) JobConfig() cron.JobConfig {
	return job.Config.JobConfig
}

// OnStart is a lifecycle event handler.
func (job Job) OnStart(ctx context.Context) {
	job.stats(ctx, cron.FlagStarted)
	if job.Config.NotifyOnStartOrDefault() {
		job.notify(ctx, cron.FlagStarted)
	}
}

// OnComplete is a lifecycle event handler.
func (job Job) OnComplete(ctx context.Context) {
	job.stats(ctx, cron.FlagComplete)
	if job.Config.NotifyOnSuccessOrDefault() {
		job.notify(ctx, cron.FlagComplete)
	}
}

// OnFailure is a lifecycle event handler.
func (job Job) OnFailure(ctx context.Context) {
	job.stats(ctx, cron.FlagFailed)
	if job.Config.NotifyOnFailureOrDefault() {
		job.notify(ctx, cron.FlagFailed)
	}
}

// OnBroken is a lifecycle event handler.
func (job Job) OnBroken(ctx context.Context) {
	job.stats(ctx, cron.FlagBroken)
	if job.Config.NotifyOnBrokenOrDefault() {
		job.notify(ctx, cron.FlagBroken)
	}
}

// OnFixed is a lifecycle event handler.
func (job Job) OnFixed(ctx context.Context) {
	job.stats(ctx, cron.FlagFixed)
	if job.Config.NotifyOnFixedOrDefault() {
		job.notify(ctx, cron.FlagFixed)
	}
}

// OnCancellation is a lifecycle event handler.
func (job Job) OnCancellation(ctx context.Context) {
	job.stats(ctx, cron.FlagCancelled)
	if job.Config.NotifyOnCancellationOrDefault() {
		job.notify(ctx, cron.FlagCancelled)
	}
}

// OnEnabled is a lifecycle event handler.
func (job Job) OnEnabled(ctx context.Context) {
	if job.Config.NotifyOnEnabledOrDefault() {
		job.notify(ctx, cron.FlagEnabled)
	}
}

// OnDisabled is a lifecycle event handler.
func (job Job) OnDisabled(ctx context.Context) {
	if job.Config.NotifyOnDisabledOrDefault() {
		job.notify(ctx, cron.FlagDisabled)
	}
}

// PersistHistory writes the history to disk.
// It does so completely, overwriting any existing history file on disk with the complete history.
func (job Job) PersistHistory(ctx context.Context, log []cron.JobInvocation) error {
	if job.History != nil {
		return job.History.PersistHistory(ctx, log)
	}
	return nil
}

// RestoreHistory restores history from disk.
func (job Job) RestoreHistory(ctx context.Context) (output []cron.JobInvocation, err error) {
	if job.History != nil {
		output, err = job.History.RestoreHistory(ctx)
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
			job.Debugf(ctx, "notify (email); sending email notification")
			job.Error(ctx, job.StatsClient.TimeInMilliseconds(string(flag), ji.Elapsed, fmt.Sprintf("%s:%s", stats.TagJob, job.Name())))
		}
	} else {
		job.Debugf(ctx, "stats client unset, skipping logging stats")
	}
}

func (job Job) notify(ctx context.Context, flag string) {
	if job.SlackClient != nil {
		if ji := cron.GetJobInvocation(ctx); ji != nil {
			job.Debugf(ctx, "notify (slack); sending slack notification")
			job.Error(ctx, job.SlackClient.Send(context.Background(), NewSlackMessage(flag, ji)))
		}
	} else {
		job.Debugf(ctx, "notify (slack); sender unset skipping sending slack notification")
	}

	if job.EmailClient != nil {
		if ji := cron.GetJobInvocation(ctx); ji != nil {
			message, err := NewEmailMessage(flag, job.EmailDefaults, ji)
			if err != nil {
				job.Error(ctx, err)
			}
			job.Error(ctx, job.EmailClient.Send(context.Background(), message))
			job.Debugf(ctx, "notify (email); sent email notification to %s (%s)", stringutil.CSV(message.To), message.Subject)
		} else {
			job.Debugf(ctx, "notify (email); job invocation not found on context")
		}
	} else {
		job.Debugf(ctx, "notify (email); email sender unset, skipping sending email notification")
	}

	if !job.WebhookDefaults.IsZero() {
		job.Debugf(ctx, "notify (webhook); sending webhook notification")
		_, err := job.WebhookDefaults.Request().Discard()
		if err != nil {
			job.Error(ctx, err)
		}
	}
}
