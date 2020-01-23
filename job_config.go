package jobkit

import "time"

// JobConfig is something you can use to give your jobs some knobs to turn
// from configuration.
// You can use this job config by embedding it into your larger job config struct.
type JobConfig struct {
	Name     string `yaml:"name"`
	Schedule string `yaml:"schedule"`

	HistoryDisabled            *bool                  `yaml:"historyDisabled"`
	HistoryPersistenceDisabled *bool                  `yaml:"historyDisabled"`
	HistoryPath                string                 `yaml:"historyPath"`
	HistoryMaxAge              *time.Duration         `yaml:"historyMaxAge"`
	HistoryMaxCount            *int                   `yaml:"historyMaxCount"`
	Parameters                 []Parameter            `yaml:"parameters"`
	Notifications              JobNotificationsConfig `yaml:"notifications"`
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

// HistoryPathOrDefault returns a value or a default.
func (jc JobConfig) HistoryPathOrDefault() string {
	if jc.HistoryPath != "" {
		return jc.HistoryPath
	}
	return DefaultHistoryPath
}

// HistoryMaxCountOrDefault returns a value or a default.
func (jc JobConfig) HistoryMaxCountOrDefault() int {
	if jc.HistoryMaxCount != nil {
		return *jc.HistoryMaxCount
	}
	return DefaultHistoryMaxCount
}

// HistoryMaxAgeOrDefault returns a value or a default.
func (jc JobConfig) HistoryMaxAgeOrDefault() time.Duration {
	if jc.HistoryMaxAge != nil {
		return *jc.HistoryMaxAge
	}
	return DefaultHistoryMaxAge
}
