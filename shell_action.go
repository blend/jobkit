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

// NewShellAction returns a new shell action.
func NewShellAction(exec []string, opts ...ShellActionOption) ShellAction {
	shellAction := ShellAction{
		Config: ShellActionConfig{
			Exec: exec,
		},
	}
	for _, opt := range opts {
		opt(&shellAction)
	}
	return shellAction
}

// OptShellActionConfig sets the shell action config.
func OptShellActionConfig(cfg ShellActionConfig) ShellActionOption {
	return func(so *ShellAction) { so.Config = cfg }
}

// ShellActionOption is a mutator for a shell action.
type ShellActionOption func(*ShellAction)

// ShellAction captures options for a shell action.
type ShellAction struct {
	Config ShellActionConfig
}

// Execute is the job body.
func (se ShellAction) Execute(ctx context.Context) error {
	ji := cron.GetJobInvocation(ctx)
	jio := GetJobInvocationOutput(ctx)

	if ji == nil || jio == nil {
		return fmt.Errorf("shell action; invocation meta required with the output set")
	}

	localExec := make([]string, len(se.Config.Exec))
	copy(localExec, se.Config.Exec)

	if !se.Config.SkipExpandEnvOrDefault() {
		for index, arg := range localExec {
			if index == 0 {
				continue
			}
			localExec[index] = os.Expand(arg, ExpandParameters(ji))
		}
	}

	cmd, err := sh.CmdContext(ctx, localExec[0], localExec[1:]...)
	if err != nil {
		return err
	}
	cmd.Env = append(os.Environ(), ParameterValuesAsEnviron(ji.Parameters)...)
	if !se.Config.DiscardOutputOrDefault() {
		if !se.Config.HideOutputOrDefault() {
			cmd.Stdout = io.MultiWriter(jio.Output, os.Stdout)
			cmd.Stderr = io.MultiWriter(jio.Output, os.Stderr)
		} else {
			cmd.Stdout = jio.Output
			cmd.Stderr = jio.Output
		}
	} else if !se.Config.HideOutputOrDefault() {
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	}
	return ex.New(cmd.Run())
}
