package jobkit

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/blend/go-sdk/cron"
	"github.com/blend/go-sdk/ex"
	"github.com/blend/go-sdk/sh"
)

// ShellAction captures options for a shell action.
type ShellAction struct {
	// Name is the name of the job.
	Name string `yaml:"name"`
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
}

// SkipExpandEnvOrDefault returns a value or a default.
func (se ShellAction) SkipExpandEnvOrDefault() bool {
	if se.SkipExpandEnv != nil {
		return *se.SkipExpandEnv
	}
	return DefaultSkipExpandEnv
}

// DiscardOutputOrDefault returns a value or a default.
func (se ShellAction) DiscardOutputOrDefault() bool {
	if se.DiscardOutput != nil {
		return *se.DiscardOutput
	}
	return DefaultDiscardOutput
}

// HideOutputOrDefault returns a value or a default.
func (se ShellAction) HideOutputOrDefault() bool {
	if se.HideOutput != nil {
		return *se.HideOutput
	}
	return DefaultHideOutput
}

// Execute is the job body.
func (se ShellAction) Execute(ctx context.Context) error {
	ji := cron.GetJobInvocation(ctx)
	jio := GetJobInvocationOutput(ctx)

	if ji == nil || jio == nil {
		return fmt.Errorf("shell action; invocation meta required with the output set")
	}

	localExec := make([]string, len(se.Exec))
	copy(localExec, se.Exec)

	if !se.SkipExpandEnvOrDefault() {
		for index, arg := range localExec {
			if index == 0 {
				continue
			}
			localExec[index] = os.Expand(arg, CreateParameterExpand(ji))
		}
	}

	cmd, err := sh.CmdContext(ctx, localExec[0], localExec[1:]...)
	if err != nil {
		return err
	}
	cmd.Env = append(os.Environ(), ParameterValuesAsEnviron(ji.Parameters)...)
	if !se.DiscardOutputOrDefault() {
		if !se.HideOutputOrDefault() {
			cmd.Stdout = io.MultiWriter(jio.Output, os.Stdout)
			cmd.Stderr = io.MultiWriter(jio.Output, os.Stderr)
		} else {
			cmd.Stdout = jio.Output
			cmd.Stderr = jio.Output
		}
	} else if !se.HideOutputOrDefault() {
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	}
	return ex.New(cmd.Run())
}

// CreateParameterExpand returns a new parameter expander for a given job invocation.
func CreateParameterExpand(ji *cron.JobInvocation) func(string) string {
	return func(name string) string {
		if ji.Parameters != nil {
			if value, ok := ji.Parameters[name]; ok {
				return value
			}
		}
		return os.Getenv(name)
	}
}
