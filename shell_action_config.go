package jobkit

// ShellActionConfig is a config for shell actions.
type ShellActionConfig struct {
	// Exec is a job body that shells out for its action.
	Exec []string `yaml:"exec"`
	// SkipExpandEnv skips expanding environment variables in the exec segments.
	SkipExpandEnv *bool `yaml:"skipExpandEnv"`
	// DiscardOutput skips setting up output buffers for job invocations.
	DiscardOutput *bool `yaml:"discardOutput"`
	// HideOutput skips writing job output to standard output and standard error.
	HideOutput *bool `yaml:"hideOutput"`
}

// SkipExpandEnvOrDefault returns a value or a default.
func (se ShellActionConfig) SkipExpandEnvOrDefault() bool {
	if se.SkipExpandEnv != nil {
		return *se.SkipExpandEnv
	}
	return DefaultSkipExpandEnv
}

// DiscardOutputOrDefault returns a value or a default.
func (se ShellActionConfig) DiscardOutputOrDefault() bool {
	if se.DiscardOutput != nil {
		return *se.DiscardOutput
	}
	return DefaultDiscardOutput
}

// HideOutputOrDefault returns a value or a default.
func (se ShellActionConfig) HideOutputOrDefault() bool {
	if se.HideOutput != nil {
		return *se.HideOutput
	}
	return DefaultHideOutput
}
