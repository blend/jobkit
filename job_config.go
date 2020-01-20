package jobkit

// JobConfig is something you can use to give your jobs some knobs to turn
// from configuration.
// You can use this job config by embedding it into your larger job config struct.
type JobConfig struct {
	Name string `yaml:"name"`
	// HistoryPath is the base path we should write job history to.
	// The files for each job will always be $HISTORY_PATH/$NAME.json
	HistoryPath string `yaml:"historyPath"`
	// Parameters are optional inputs for jobs.
	Parameters []Parameter `yaml:"parameters"`
	// Notifications hold options for notifications.
	Notifications JobNotificationsConfig `yaml:"notifications"`
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
