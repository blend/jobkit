package jobkit

import (
	"github.com/blend/go-sdk/cron"
)

// JobConfig is something you can use to give your jobs some knobs to turn
// from configuration.
// You can use this job config by embedding it into your larger job config struct.
type JobConfig struct {
	cron.JobConfig `yaml:",inline"`
	// Schedule is the job schedule in cron string form.
	Schedule string `yaml:"schedule"`
	// Exec is a job body that shells out for its action.
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
