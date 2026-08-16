package cmd

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/jmcampanini/cmdk/internal/config"
	"github.com/jmcampanini/cmdk/internal/tmux"
)

type sessionWindowOptions struct {
	newShell     bool
	name         string
	switchWindow bool
}

// sessionWindowRequest is the result of grammar validation: the positional
// path, the command argv after --, and whether --name was set explicitly.
type sessionWindowRequest struct {
	path        string
	commandArgs []string
	nameSet     bool
}

var createResolvedSessionWindow = tmux.CreateResolvedSessionWindow

func newSessionWindowCommand() *cobra.Command {
	options := sessionWindowOptions{}
	var request sessionWindowRequest
	cmd := &cobra.Command{
		Use:   "window <path> [--name <name>] [--switch] (--new | -- <command> [args...])",
		Short: "Create a new tmux window in a cmdk-managed session for a path",
		Long: `Create a fresh tmux window in the cmdk-managed session for a path.

The path is required, must exist, and must be a directory. cmdk resolves the path
using the same session resolver as "cmdk session resolve": repo/worktree paths
share a managed repo/container session, while non-repo directories get one
managed session per canonical directory.

Exactly one mode is required:
  --new                    create an interactive shell window
  -- <command> [args...]   create a command window

The new window's cwd is the validated path. The default window name is the base
name of that path; --name overrides it for either mode and must not be empty.
Names longer than [behavior].window_name_max_length (default 20) are shortened
to that many characters ending in …; set it to 0 to keep full names.
By default, cmdk creates the window in the background without changing the
current tmux window. --switch switches the current client to the new window.

Command args after -- are treated as argv-style input and are shell-quoted before
being passed to tmux as its shell-command string. Shell metacharacters are
literal by default; invoke a shell explicitly for shell features, for example:

  cmdk session window . --name tests -- sh -lc 'npm test | tee test.log'
  cmdk session window . --switch --new
  cmdk session window . --switch -- make test

--switch must appear before --. Arguments after -- are always part of the window
command. cmdk creates a fresh window every time and tracks it by the returned
tmux window_id.`,
		Args: func(cmd *cobra.Command, args []string) error {
			parsed, err := parseSessionWindowGrammar(cmd, args, options)
			if err != nil {
				return err
			}
			request = parsed
			return nil
		},
		PreRunE: requireTmux,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runSessionWindowCommand(cmd, request, options)
		},
	}
	cmd.Flags().BoolVar(&options.newShell, "new", false, "create a fresh interactive shell window")
	cmd.Flags().StringVar(&options.name, "name", "", "override the tmux window name")
	cmd.Flags().BoolVar(&options.switchWindow, "switch", false, "switch the current tmux client to the new window")
	return cmd
}

// parseSessionWindowGrammar validates the complete positional grammar before
// any command work: exactly one path before --, exactly one of --new or a
// nonempty command after --, and no empty explicit --name. Cobra runs it as
// the Args validator, after flag parsing and before the tmux prerequisite.
func parseSessionWindowGrammar(cmd *cobra.Command, args []string, options sessionWindowOptions) (sessionWindowRequest, error) {
	if len(args) == 0 {
		return sessionWindowRequest{}, errors.New("path is required")
	}
	nameSet := cmd.Flags().Changed("name")
	if nameSet && options.name == "" {
		return sessionWindowRequest{}, errors.New("--name cannot be empty")
	}
	path, commandArgs, commandDelimiter, err := splitSessionWindowArgs(args, cmd.Flags().ArgsLenAtDash())
	if err != nil {
		return sessionWindowRequest{}, err
	}
	haveCommand := len(commandArgs) > 0
	if haveCommand && !commandDelimiter {
		return sessionWindowRequest{}, errors.New("command args must follow --")
	}
	if options.newShell && haveCommand {
		return sessionWindowRequest{}, errors.New("--new cannot be used with command args")
	}
	if !options.newShell && !haveCommand {
		return sessionWindowRequest{}, errors.New("session window requires --new or command args after --")
	}
	return sessionWindowRequest{path: path, commandArgs: commandArgs, nameSet: nameSet}, nil
}

func runSessionWindowCommand(cmd *cobra.Command, request sessionWindowRequest, options sessionWindowOptions) error {
	launchPath, err := validateLaunchDirectory(request.path)
	if err != nil {
		return err
	}
	cfgPath, err := resolveConfigPath()
	if err != nil {
		return err
	}
	cfg, err := config.Load(cfgPath)
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}
	plan, err := resolveSessionPlanWithConfig(cmd, launchPath, cfg)
	if err != nil {
		return err
	}
	windowName := options.name
	if !request.nameSet {
		windowName = defaultWindowNameForLaunchPath(launchPath)
	}

	_, err = createResolvedSessionWindow(commandContext(cmd), plan, launchPath, tmux.SessionWindowOptions{
		Name:          windowName,
		NewShell:      options.newShell,
		Command:       request.commandArgs,
		Switch:        options.switchWindow,
		MaxNameLength: cfg.Behavior.WindowNameMaxLength,
		Timeouts:      tmuxTimeouts(cfg),
	})
	return err
}

func splitSessionWindowArgs(args []string, argsLenAtDash int) (string, []string, bool, error) {
	if len(args) == 0 {
		return "", nil, false, errors.New("path is required")
	}

	if argsLenAtDash >= 0 {
		switch argsLenAtDash {
		case 0:
			path := args[0]
			rest := args[1:]
			if len(rest) > 0 && rest[0] == "--" {
				return path, rest[1:], true, nil
			}
			return path, rest, false, nil
		case 1:
			return args[0], args[1:], true, nil
		default:
			return "", nil, false, errors.New("expected exactly one path before --")
		}
	}

	return args[0], args[1:], false, nil
}
