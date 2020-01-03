package jobkit

import (
	"time"

	"github.com/blend/go-sdk/cron"
	"github.com/blend/go-sdk/email"
)

// JobConfig is something you can use to give your jobs some knobs to turn
// from configuration.
// You can use this job config by embedding it into your larger job config struct.
type JobConfig struct {
	cron.JobConfig `yaml:",inline"`
	// Schedule is the job schedule in cron string form.
	Schedule string `yaml:"schedule"`
	// Exec is a job body that shells out for it's action.
	Exec []string `yaml:"exec"`
	// SkipExpandEnv skips expanding environment variables in the exec segments.
	SkipExpandEnv *bool `yaml:"skipExpandEnv"`
	// DiscardOutput skips setting up output buffers for job invocations.
	DiscardOutput *bool `yaml:"discardOutput"`
	// HideOutput skips writing job output to standard output and standard error.
	HideOutput *bool `yaml:"hideOutput"`
	// HistoryPath is the base path we should write job history to.
	// The files for each job will always be $HISTORY_PATH/$NAME.json
	HistoryPath string `yaml:"historyPath"`
	// Parameters are optional inputs for jobs.
	Parameters []Parameter `yaml:"parameters"`
	// Notifications hold options for notifications.
	Notifications JobNotificationsConfig `yaml:"notifications"`
}

// ScheduleOrDefault returns the schedule or a default (every 5 minutes).
func (jc JobConfig) ScheduleOrDefault() string {
	if jc.Schedule != "" {
		return jc.Schedule
	}
	return "* */5 * * * * *"
}

// HistoryPathOrDefault returns a value or a default.
func (jc JobConfig) HistoryPathOrDefault() string {
	if jc.HistoryPath != "" {
		return jc.HistoryPath
	}
	return DefaultHistoryPath
}

// SkipExpandEnvOrDefault returns a value or a default.
func (jc JobConfig) SkipExpandEnvOrDefault() bool {
	if jc.SkipExpandEnv != nil {
		return *jc.SkipExpandEnv
	}
	return DefaultSkipExpandEnv
}

// DiscardOutputOrDefault returns a value or a default.
func (jc JobConfig) DiscardOutputOrDefault() bool {
	if jc.DiscardOutput != nil {
		return *jc.DiscardOutput
	}
	return DefaultDiscardOutput
}

// HideOutputOrDefault returns a value or a default.
func (jc JobConfig) HideOutputOrDefault() bool {
	if jc.HideOutput != nil {
		return *jc.HideOutput
	}
	return DefaultHideOutput
}

// JobNotificationsConfig are the notification options for a job.
type JobNotificationsConfig struct {
	// Email holds the message defaults for email notifications.
	Email email.Message `yaml:"email"`
	// Webhook set a webhook target for notifications.
	Webhook Webhook `yaml:"webhook"`

	// MaxRetries is the maximum number of retries before we give up on a notification.
	MaxRetries int `yaml:"maxRetries"`
	// RetryWait is the time between attempts.
	RetryWait time.Duration `yaml:"retryWait"`

	// OnBegin governs if we should send notifications job start.
	OnBegin *bool `yaml:"onBegin"`
	// NotifyOnSuccess governs if we should send notifications on any success.
	OnSuccess *bool `yaml:"onSuccess"`
	// NotifyOnFailure governs if we should send notifications on any failure.
	OnFailure *bool `yaml:"onFailure"`
	// NotifyOnCancellation governs if we should send notifications on cancellation.
	OnCancellation *bool `yaml:"onCancellation"`
	// NotifyOnBroken governs if we should send notifications on a success => failure transition.
	OnBroken *bool `yaml:"onBroken"`
	// NotifyOnFixed governs if we should send notifications on a failure => success transition.
	OnFixed *bool `yaml:"onFixed"`
	// NotifyOnEnabled governs if we should send notifications when a job is enabled.
	OnEnabled *bool `yaml:"onEnabled"`
	// NotifyOnDisabled governs if we should send notifications when a job is disabled.
	OnDisabled *bool `yaml:"onDisabled"`
}

// OnBeginOrDefault returns a value or a default.
func (jnc JobNotificationsConfig) OnBeginOrDefault() bool {
	if jnc.OnBegin != nil {
		return *jnc.OnBegin
	}
	return false
}

// OnSuccessOrDefault returns a value or a default.
func (jnc JobNotificationsConfig) OnSuccessOrDefault() bool {
	if jnc.OnSuccess != nil {
		return *jnc.OnSuccess
	}
	return false
}

// OnFailureOrDefault returns a value or a default.
func (jnc JobNotificationsConfig) OnFailureOrDefault() bool {
	if jnc.OnFailure != nil {
		return *jnc.OnFailure
	}
	return true
}

// OnCancellationOrDefault returns a value or a default.
func (jnc JobNotificationsConfig) OnCancellationOrDefault() bool {
	if jnc.OnCancellation != nil {
		return *jnc.OnCancellation
	}
	return true
}

// OnBrokenOrDefault returns a value or a default.
func (jnc JobNotificationsConfig) OnBrokenOrDefault() bool {
	if jnc.OnBroken != nil {
		return *jnc.OnBroken
	}
	return true
}

// OnFixedOrDefault returns a value or a default.
func (jnc JobNotificationsConfig) OnFixedOrDefault() bool {
	if jnc.OnFixed != nil {
		return *jnc.OnFixed
	}
	return true
}

// OnEnabledOrDefault returns a value or a default.
func (jnc JobNotificationsConfig) OnEnabledOrDefault() bool {
	if jnc.OnEnabled != nil {
		return *jnc.OnEnabled
	}
	return false
}

// OnDisabledOrDefault returns a value or a default.
func (jnc JobNotificationsConfig) OnDisabledOrDefault() bool {
	if jnc.OnDisabled != nil {
		return *jnc.OnDisabled
	}
	return false
}
