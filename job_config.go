package jobkit

import "time"
import "github.com/blend/go-sdk/cron"

// JobConfig is something you can use to give your jobs some knobs to turn
// from configuration.
// You can use this job config by embedding it into your larger job config struct.
type JobConfig struct {
	cron.JobConfig    `yaml:",inline"`
	ShellActionConfig `yaml:",inline"`

	Name                       string                 `yaml:"name"`
	Schedule                   string                 `yaml:"schedule"`
	HistoryPath                string                 `yaml:"historyPath"`
	HistoryDisabled            *bool                  `yaml:"historyDisabled"`
	HistoryPersistenceDisabled *bool                  `yaml:"historyPersistenceDisabled"`
	HistoryMaxAge              *time.Duration         `yaml:"historyMaxAge"`
	HistoryMaxCount            *int                   `yaml:"historyMaxCount"`
	Parameters                 []Parameter            `yaml:"parameters"`
	Notifications              JobNotificationsConfig `yaml:"notifications"`
}

// ScheduleOrDefault returns a value or a default.
func (jc JobConfig) ScheduleOrDefault() string {
	if jc.Schedule != "" {
		return jc.Schedule
	}
	return DefaultSchedule
}

// HistoryPathOrDefault returns a value or a default.
func (jc JobConfig) HistoryPathOrDefault() string {
	if jc.HistoryPath != "" {
		return jc.HistoryPath
	}
	return DefaultHistoryPath
}

// HistoryDisabledOrDefault returns a value or a default.
func (jc JobConfig) HistoryDisabledOrDefault() bool {
	if jc.HistoryDisabled != nil {
		return *jc.HistoryDisabled
	}
	return false
}

// HistoryPersistenceDisabledOrDefault returns a value or a default.
func (jc JobConfig) HistoryPersistenceDisabledOrDefault() bool {
	if jc.HistoryPersistenceDisabled != nil {
		return *jc.HistoryPersistenceDisabled
	}
	return false
}

// HistoryMaxAgeOrDefault returns a value or a default.
func (jc JobConfig) HistoryMaxAgeOrDefault() time.Duration {
	if jc.HistoryMaxAge != nil {
		return *jc.HistoryMaxAge
	}
	return DefaultHistoryMaxAge
}

// HistoryMaxCountOrDefault returns a value or a default.
func (jc JobConfig) HistoryMaxCountOrDefault() int {
	if jc.HistoryMaxCount != nil {
		return *jc.HistoryMaxCount
	}
	return DefaultHistoryMaxCount
}
