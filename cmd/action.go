package cmd

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/jmcampanini/cmdk/internal/actionrun"
	"github.com/jmcampanini/cmdk/internal/cmdrun"
	"github.com/jmcampanini/cmdk/internal/config"
	"github.com/jmcampanini/cmdk/internal/execute"
	"github.com/jmcampanini/cmdk/internal/safetext"
	"github.com/jmcampanini/cmdk/internal/tmux"
)

type actionRunOptions struct {
	path   string
	inputs []string
}

type actionRunInvocation struct {
	actionName string
	client     tmux.ClientTarget
	config     config.Config
	prepared   actionrun.Prepared
}

type actionRunResult struct {
	Action     string `json:"action"`
	LaunchPath string `json:"launchPath"`
	SessionID  string `json:"sessionId"`
	SessionKey string `json:"sessionKey"`
	WindowID   string `json:"windowId"`
	WindowName string `json:"windowName"`
	PaneID     string `json:"paneId"`
}

var (
	currentActionRunClient        = tmux.CurrentClient
	resolveConfiguredActionLaunch = execute.ResolveLaunch
	executeConfiguredActionLaunch = func(launch execute.Launch) (execute.LaunchResult, error) {
		return launch.ExecuteWithResult(cmdrun.Replace)
	}
)

func newActionCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "action",
		Short: "Run configured actions noninteractively",
		Long: `Run configured actions without opening the interactive picker.

Only configured root and directory actions whose effective launch mode is
session-window are supported.`,
	}
	cmd.SetFlagErrorFunc(func(_ *cobra.Command, err error) error {
		return terminalSafeActionRunError(err)
	})
	cmd.AddCommand(newActionRunCommand())
	return cmd
}

func newActionRunCommand() *cobra.Command {
	options := actionRunOptions{}
	var invocation *actionRunInvocation

	cmd := &cobra.Command{
		Use:   "run <exact-name>",
		Short: "Run a configured session-window action",
		Long: `Run one configured action by its exact, case-sensitive name.

Only actions matching root or dir whose effective launch_mode is session-window
are supported. Session-matching and shell-mode actions fail before action inputs
are resolved or external commands run. The command must run from inside an
attached tmux client because successful actions switch that client.

Directory actions require --path. Root actions reject --path and retain their
configured launch path behavior, including the current-directory fallback for
an explicit session-window action without launch_path or launch_path_cmd.

Supply each action input with a separate --input key=value flag. Input keys are
exact and case-sensitive. Values are split at the first = and otherwise remain
literal, including commas, dots, brackets, spaces, and empty strings. Prompt
inputs may use their rendered defaults when omitted. Picker inputs must be
supplied and are accepted directly without running the picker source.

Missing, unknown, duplicate, and disallowed-empty inputs fail before
launch_path_cmd or tmux mutation. On success, cmdk creates and switches to the
new window and writes one JSON object to stdout:

  {
    "action": "pi worktree",
    "launchPath": "/absolute/path/to/worktree",
    "sessionId": "$5",
    "sessionKey": "/absolute/path/to/repository",
    "windowId": "@18",
    "windowName": "wt-hyperion-notific…",
    "paneId": "%51"
  }

Diagnostics are written to stderr and return exit status 1. No --json flag is
needed. cmdk does not send text or keys to the launched application.`,
		Args: cobra.ExactArgs(1),
		PreRunE: func(cmd *cobra.Command, args []string) error {
			prepared, err := prepareActionRunInvocation(cmd, args[0], options)
			if err != nil {
				return terminalSafeActionRunError(err)
			}
			invocation = &prepared
			return nil
		},
		RunE: func(cmd *cobra.Command, _ []string) error {
			if invocation == nil {
				return errors.New("internal error: action run was not prepared")
			}
			return terminalSafeActionRunError(runPreparedAction(cmd, *invocation))
		},
	}
	cmd.Flags().StringVar(&options.path, "path", "", "selected directory context (required for dir actions)")
	cmd.Flags().StringArrayVar(&options.inputs, "input", nil, "action input as exact key=value (repeatable)")
	return cmd
}

func prepareActionRunInvocation(cmd *cobra.Command, name string, options actionRunOptions) (actionRunInvocation, error) {
	cfgPath, err := resolveConfigPath()
	if err != nil {
		return actionRunInvocation{}, err
	}
	cfg, err := config.Load(cfgPath)
	if err != nil {
		return actionRunInvocation{}, fmt.Errorf("loading config: %w", err)
	}

	action, err := actionrun.ValidateAction(cfg, name)
	if err != nil {
		return actionRunInvocation{}, err
	}
	pathSet := cmd.Flags().Changed("path")
	switch action.Matches {
	case "root":
		if pathSet {
			return actionRunInvocation{}, fmt.Errorf("action %q: --path is not valid for an action matching root", name)
		}
	case "dir":
		if !pathSet || options.path == "" {
			return actionRunInvocation{}, fmt.Errorf("action %q: --path is required for an action matching dir", name)
		}
	}

	if err := requireTmux(cmd, nil); err != nil {
		return actionRunInvocation{}, err
	}
	client, err := currentActionRunClient(commandContext(cmd), cfg.Timeout.EffectiveFetch())
	if err != nil {
		return actionRunInvocation{}, err
	}

	prepared, err := actionrun.Prepare(cfg, name, options.path, client.PaneID, options.inputs)
	if err != nil {
		return actionRunInvocation{}, err
	}
	return actionRunInvocation{
		actionName: action.Name,
		client:     client,
		config:     cfg,
		prepared:   prepared,
	}, nil
}

func runPreparedAction(cmd *cobra.Command, invocation actionRunInvocation) error {
	launch, _, err := resolveConfiguredActionLaunch(
		invocation.prepared.Accumulated,
		invocation.prepared.Selected,
		invocation.client.PaneID,
		invocation.config,
	)
	if err != nil {
		return err
	}
	result, err := executeConfiguredActionLaunch(launch.ForClient(invocation.client))
	if err != nil {
		return err
	}

	return writeActionRunResult(cmd, actionRunResult{
		Action:     invocation.actionName,
		LaunchPath: result.LaunchPath,
		SessionID:  result.SessionID,
		SessionKey: result.SessionKey,
		WindowID:   result.WindowID,
		WindowName: result.WindowName,
		PaneID:     result.PaneID,
	})
}

type actionRunError struct {
	cause error
}

func (e *actionRunError) Error() string {
	return safetext.EscapeTerminalControls(e.cause.Error())
}

func (e *actionRunError) Unwrap() error {
	return e.cause
}

func terminalSafeActionRunError(err error) error {
	if err == nil {
		return nil
	}
	return &actionRunError{cause: err}
}

func writeActionRunResult(cmd *cobra.Command, result actionRunResult) error {
	enc := json.NewEncoder(cmd.OutOrStdout())
	enc.SetIndent("", "  ")
	if err := enc.Encode(result); err != nil {
		return fmt.Errorf("writing action run result: %w", err)
	}
	return nil
}
