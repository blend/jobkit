package jobkit

import (
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
	// EmailDefaults holds the message defaults for email notifications.
	EmailDefaults email.Message `yaml:"emailDefaults"`
	// WebhookDefaults set a webhook target for notifications.
	WebhookDefaults Webhook `yaml:"webhookDefaults"`

	// NotifyOnStart governs if we should send notifications job start.
	NotifyOnStart *bool `yaml:"notifyOnStart"`
	// NotifyOnSuccess governs if we should send notifications on any success.
	NotifyOnSuccess *bool `yaml:"notifyOnSuccess"`
	// NotifyOnFailure governs if we should send notifications on any failure.
	NotifyOnFailure *bool `yaml:"notifyOnFailure"`
	// NotifyOnCancellation governs if we should send notifications on cancellation.
	NotifyOnCancellation *bool `yaml:"notifyOnCancellation"`
	// NotifyOnBroken governs if we should send notifications on a success => failure transition.
	NotifyOnBroken *bool `yaml:"notifyOnBroken"`
	// NotifyOnFixed governs if we should send notifications on a failure => success transition.
	NotifyOnFixed *bool `yaml:"notifyOnFixed"`
	// NotifyOnEnabled governs if we should send notifications when a job is enabled.
	NotifyOnEnabled *bool `yaml:"notifyOnEnabled"`
	// NotifyOnDisabled governs if we should send notifications when a job is disabled.
	NotifyOnDisabled *bool `yaml:"notifyOnDisabled"`
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

// NotifyOnStartOrDefault returns a value or a default.
func (jc JobConfig) NotifyOnStartOrDefault() bool {
	if jc.NotifyOnStart != nil {
		return *jc.NotifyOnStart
	}
	return false
}

// NotifyOnSuccessOrDefault returns a value or a default.
func (jc JobConfig) NotifyOnSuccessOrDefault() bool {
	if jc.NotifyOnSuccess != nil {
		return *jc.NotifyOnSuccess
	}
	return false
}

// NotifyOnFailureOrDefault returns a value or a default.
func (jc JobConfig) NotifyOnFailureOrDefault() bool {
	if jc.NotifyOnFailure != nil {
		return *jc.NotifyOnFailure
	}
	return true
}

// NotifyOnCancellationOrDefault returns a value or a default.
func (jc JobConfig) NotifyOnCancellationOrDefault() bool {
	if jc.NotifyOnCancellation != nil {
		return *jc.NotifyOnCancellation
	}
	return true
}

// NotifyOnBrokenOrDefault returns a value or a default.
func (jc JobConfig) NotifyOnBrokenOrDefault() bool {
	if jc.NotifyOnBroken != nil {
		return *jc.NotifyOnBroken
	}
	return true
}

// NotifyOnFixedOrDefault returns a value or a default.
func (jc JobConfig) NotifyOnFixedOrDefault() bool {
	if jc.NotifyOnFixed != nil {
		return *jc.NotifyOnFixed
	}
	return true
}

// NotifyOnEnabledOrDefault returns a value or a default.
func (jc JobConfig) NotifyOnEnabledOrDefault() bool {
	if jc.NotifyOnEnabled != nil {
		return *jc.NotifyOnEnabled
	}
	return false
}

// NotifyOnDisabledOrDefault returns a value or a default.
func (jc JobConfig) NotifyOnDisabledOrDefault() bool {
	if jc.NotifyOnDisabled != nil {
		return *jc.NotifyOnDisabled
	}
	return false
}
