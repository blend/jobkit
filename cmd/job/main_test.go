package main

import (
	"testing"

	"github.com/blend/assert/assert"
	"github.com/blend/go-sdk/logger"
	"github.com/blend/jobkit"
)

func Test_createJobFromConfig(t *testing.T) {
	assert := assert.New(t)

	log := logger.None()
	baseCfg := config{}
	cfg := jobkit.JobConfig{
		Name: "createJobFromConfig_test",
		ShellActionConfig: jobkit.ShellActionConfig{
			Exec: []string{"echo", "'hello world!'"},
		},
	}
	historyProvider := new(jobkit.HistoryMemory)

	job, err := createJobFromConfig(baseCfg, cfg, log, historyProvider)
	assert.Nil(err)
	assert.NotNil(job)
}
