package jobkit

// Constants and Defaults
const (
	DefaultMaxLogBytes   = 10 * (1 << 10)
	DefaultHistoryPath   = "_history"
	DefaultSkipExpandEnv = false
	DefaultDiscardOutput = false
	DefaultHideOutput    = false

	DefaultHistoryMaxCount = 256
	DefaultHistoryMaxAge   = 0
)
