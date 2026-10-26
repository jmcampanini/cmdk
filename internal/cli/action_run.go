package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/jmcampanini/cmdk/internal/actionrun"
	"github.com/jmcampanini/cmdk/internal/cmdrun"
	"github.com/jmcampanini/cmdk/internal/config"
	"github.com/jmcampanini/cmdk/internal/execute"
	"github.com/jmcampanini/cmdk/internal/tmux"
)

type actionRunOptions struct {
	path     string
	inputs   []string
	noSwitch bool
}

type actionRunInvocation struct {
	actionName string
	client     tmux.ClientTarget
	config     config.Config
	prepared   actionrun.Prepared
	noSwitch   bool
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
	requireActionRunServer        = tmux.RequireRunningServer
	resolveConfiguredActionLaunch = execute.ResolveLaunch
	executeConfiguredActionLaunch = func(launch execute.Launch) (execute.LaunchResult, error) {
		return launch.ExecuteWithResult(cmdrun.Replace)
	}
)

func newActionRunCommand(root *rootOptions) *cobra.Command {
	options := actionRunOptions{}
	var invocation *actionRunInvocation

	cmd := &cobra.Command{
		Use:   "run <exact-name>",
		Short: "Run a configured session-window action",
		Long: `Run one configured action by its exact, case-sensitive name.

The name must match exactly one configured action; no match, or more than
one, is an error. Only root and dir actions whose effective launch mode is
session-window are supported. Session actions and shell-mode actions are
rejected before action inputs are resolved or external commands run.

By default cmdk must run inside an attached tmux client; it creates the
window and switches that exact client to it. With --no-switch cmdk requires a
running tmux server but not an attached client, creates the window in the
background, and never runs switch-client. cmdk does not start a server, and a
missing server fails before inputs are resolved or external commands run.
--no-switch provides no invoking pane: {{.pane_id}} is unavailable and
CMDK_PANE_ID is unset, so a template that references pane_id fails before
launch_path_cmd runs or tmux is changed.

A dir action requires --path, its selected directory. A root action rejects
--path and keeps its configured launch path behavior, including the
current-directory fallback of an explicit session-window action that has
neither launch_path nor launch_path_cmd.

Supply each action input with its own --input key=value. Keys are exact and
case-sensitive; the value is everything after the first = and stays literal,
including commas, dots, brackets, spaces, and an empty string. A prompt input
may be omitted when its stage has a default; key= supplies an empty value,
which only a stage with allow_empty accepts. A picker input must be supplied
and is used as the stage's value without running its source. Missing,
unknown, duplicate, and disallowed-empty inputs fail before launch_path_cmd
runs or tmux is changed, and the message lists the accepted keys.

Stdout: on success, one JSON object with the keys action, launchPath,
sessionId, sessionKey, windowId, windowName, and paneId, for example:

  {
    "action": "pi worktree",
    "launchPath": "/absolute/path/to/worktree",
    "sessionId": "$5",
    "sessionKey": "/absolute/path/to/repository",
    "windowId": "@18",
    "windowName": "wt-hyperion-notific…",
    "paneId": "%51"
  }

Stderr: every diagnostic; a failure writes no JSON and exits 1. If the window
and its command were created but switching the client fails, the action is
already running: the diagnostic names the created session, window, and pane
and how to switch there by hand. Do not rerun it; automation that does not
need the switch should pass --no-switch. cmdk sends no keys to the launched
command and never prompts. The command runs launch_path_cmd when the action
has one, and runs tmux to find the client and create the window.

` + tmuxPrerequisiteHelp + `

` + configLoadingHelpWithKeys,
		Example: `  cmdk action run "Deploy" --path . --input environment=production
  cmdk action run "Dotfiles pi" --no-switch`,
		Args: cobra.ExactArgs(1),
		PreRunE: func(cmd *cobra.Command, args []string) error {
			prepared, err := prepareActionRunInvocation(root, cmd, args[0], options)
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
	cmd.Flags().StringVar(&options.path, "path", "", "selected directory (required for dir actions)")
	cmd.Flags().StringArrayVar(&options.inputs, "input", nil, "action input as key=value (repeatable)")
	cmd.Flags().BoolVar(&options.noSwitch, "no-switch", false, "create the window without switching a client")
	return cmd
}

func prepareActionRunInvocation(root *rootOptions, cmd *cobra.Command, name string, options actionRunOptions) (actionRunInvocation, error) {
	loaded, err := root.loadConfig(cmd)
	if err != nil {
		return actionRunInvocation{}, err
	}
	cfg := loaded.config

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
	var client tmux.ClientTarget
	if options.noSwitch {
		if err := requireActionRunServer(commandContext(cmd), cfg.Timeout.Fetch); err != nil {
			return actionRunInvocation{}, err
		}
	} else {
		client, err = currentActionRunClient(commandContext(cmd), cfg.Timeout.Fetch)
		if err != nil {
			return actionRunInvocation{}, err
		}
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
		noSwitch:   options.noSwitch,
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
	if invocation.noSwitch {
		launch = launch.WithoutSwitch()
	} else {
		launch = launch.ForClient(invocation.client)
	}
	result, err := executeConfiguredActionLaunch(launch)
	if err != nil {
		var switchErr *tmux.SwitchClientError
		if errors.As(err, &switchErr) && completeActionLaunchResult(result) {
			return actionRunSwitchFailureError(invocation.actionName, result, switchErr)
		}
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

func completeActionLaunchResult(result execute.LaunchResult) bool {
	return result.LaunchPath != "" &&
		result.SessionID != "" &&
		result.SessionKey != "" &&
		result.WindowID != "" &&
		result.WindowName != "" &&
		result.PaneID != ""
}

func actionRunSwitchFailureError(actionName string, result execute.LaunchResult, switchErr *tmux.SwitchClientError) error {
	switchFailure := strings.ReplaceAll(strings.TrimRight(switchErr.Error(), "\n"), "\n", "\n  ")
	message := fmt.Sprintf(`action %q launched, but switching the client failed.

The action's side effects already happened: the launch path exists and the
window is running its command. Do not rerun this action - rerunning launches
it a second time.

Created tmux state:
  session:     %s  (%s)
  window:      %s  %s
  pane:        %s
  launch path: %s

Switch failure: %s

To go there manually: tmux switch-client -t '%s:%s'
Automation that does not need the switch should pass --no-switch`,
		actionName,
		result.SessionID,
		result.SessionKey,
		result.WindowID,
		result.WindowName,
		result.PaneID,
		result.LaunchPath,
		switchFailure,
		result.SessionID,
		result.WindowID,
	)
	return &actionRunSwitchFailureDiagnostic{message: message, cause: switchErr}
}

type actionRunSwitchFailureDiagnostic struct {
	message string
	cause   error
}

func (e *actionRunSwitchFailureDiagnostic) Error() string {
	return e.message
}

func (e *actionRunSwitchFailureDiagnostic) Unwrap() error {
	return e.cause
}

func writeActionRunResult(cmd *cobra.Command, result actionRunResult) error {
	enc := json.NewEncoder(cmd.OutOrStdout())
	enc.SetIndent("", "  ")
	if err := enc.Encode(result); err != nil {
		return fmt.Errorf("writing action run result: %w", err)
	}
	return nil
}
