package jobkit

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/blend/go-sdk/cron"
	"github.com/blend/go-sdk/ex"
	"github.com/blend/go-sdk/logger"
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

// OptShellActionLog sets the shell action logger.
func OptShellActionLog(log logger.Log) ShellActionOption {
	return func(so *ShellAction) { so.Log = log }
}

// ShellActionOption is a mutator for a shell action.
type ShellActionOption func(*ShellAction)

// ShellAction captures options for a shell action.
type ShellAction struct {
	Log    logger.Log
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
			if se.Log != nil {
				logOutput := logOutputStream{ctx, se.Log}
				cmd.Stdout = io.MultiWriter(jio.Output, logOutput)
				cmd.Stderr = io.MultiWriter(jio.Output, logOutput)
			} else {
				cmd.Stdout = io.MultiWriter(jio.Output, os.Stdout)
				cmd.Stderr = io.MultiWriter(jio.Output, os.Stderr)
			}
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

// Logger Constants
const (
	ShellActionLogFlag = "shell.action"
)

type logOutputStream struct {
	Context context.Context
	Log     logger.Log
}

func (los logOutputStream) Write(contents []byte) (count int, err error) {
	if los.Log == nil {
		return
	}
	los.Log.Trigger(los.Context, logger.NewMessageEvent(ShellActionLogFlag, string(contents)))
	count = len(contents)
	return
}
