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

// ShellActionOption mutates a ShellActionOptions object.
type ShellActionOption func(*ShellActionOptions)

// OptShellActionDiscardOutput sets the `Discard` field on the options.
func OptShellActionDiscardOutput(discard bool) ShellActionOption {
	return func(opts *ShellActionOptions) { opts.DiscardOutput = discard }
}

// OptShellActionSkipExpandEnv sets if ShellAction should skip expanding env values.
func OptShellActionSkipExpandEnv(SkipExpandEnv bool) ShellActionOption {
	return func(opts *ShellActionOptions) { opts.SkipExpandEnv = SkipExpandEnv }
}

// ShellActionOptions captures options for a shell action.
type ShellActionOptions struct {
	SkipExpandEnv bool `yaml:"skipExpandEnv"`
	DiscardOutput bool `yaml:"discardOutput"`
}

// ShellAction creates a new shell action.
func ShellAction(exec []string, opts ...ShellActionOption) func(context.Context) error {
	var options ShellActionOptions
	for _, opt := range opts {
		opt(&options)
	}

	return func(ctx context.Context) error {
		ji := cron.GetJobInvocation(ctx)
		if ji == nil || ji.Output == nil {
			return fmt.Errorf("shell action; invocation meta required with the output set")
		}

		localExec := make([]string, len(exec))
		copy(localExec, exec)

		if !options.SkipExpandEnv {
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
		if !options.DiscardOutput {
			cmd.Stdout = io.MultiWriter(ji.Output, os.Stdout)
			cmd.Stderr = io.MultiWriter(ji.Output, os.Stderr)
		}
		return ex.New(cmd.Run())
	}
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
