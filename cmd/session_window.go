package cmd

import (
	"context"
	"errors"

	"github.com/spf13/cobra"

	"github.com/jmcampanini/cmdk/internal/tmux"
)

type sessionWindowOptions struct {
	newShell      bool
	name          string
	nameSet       bool
	dashSeen      bool
	argsLenAtDash int
}

var createResolvedSessionWindow = tmux.CreateResolvedSessionWindow

func newSessionWindowCommand() *cobra.Command {
	options := sessionWindowOptions{}
	cmd := &cobra.Command{
		Use:   "window <path> [--name <name>] (--new | -- <command> [args...])",
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

Command args after -- are treated as argv-style input and are shell-quoted before
being passed to tmux as its shell-command string. Shell metacharacters are
literal by default; invoke a shell explicitly for shell features, for example:

  cmdk session window . --name tests -- sh -lc 'npm test | tee test.log'

cmdk creates a fresh window every time, tracks it by the returned tmux window_id,
and switches the current tmux client to <session_id>:<window_id>.`,
		Args:    cobra.MinimumNArgs(1),
		PreRunE: requireTmux,
		RunE: func(cmd *cobra.Command, args []string) error {
			options.nameSet = cmd.Flags().Changed("name")
			options.argsLenAtDash = cmd.Flags().ArgsLenAtDash()
			options.dashSeen = options.argsLenAtDash >= 0
			return runSessionWindowCommand(cmd, args, options)
		},
	}
	cmd.Flags().BoolVar(&options.newShell, "new", false, "create a fresh interactive shell window")
	cmd.Flags().StringVar(&options.name, "name", "", "override the tmux window name")
	return cmd
}

func runSessionWindowCommand(cmd *cobra.Command, args []string, options sessionWindowOptions) error {
	if len(args) == 0 {
		return errors.New("path is required")
	}
	if options.nameSet && options.name == "" {
		return errors.New("--name cannot be empty")
	}
	path, commandArgs, commandDelimiter, err := splitSessionWindowArgs(args, options)
	if err != nil {
		return err
	}
	haveCommand := len(commandArgs) > 0
	if haveCommand && !commandDelimiter {
		return errors.New("command args must follow --")
	}
	if options.newShell && haveCommand {
		return errors.New("--new cannot be used with command args")
	}
	if !options.newShell && !haveCommand {
		return errors.New("session window requires --new or command args after --")
	}

	launchPath, err := validateLaunchDirectory(path)
	if err != nil {
		return err
	}
	plan, err := resolveSessionPlanForCommand(cmd, launchPath)
	if err != nil {
		return err
	}
	windowName := options.name
	if !options.nameSet {
		windowName = defaultWindowNameForLaunchPath(launchPath)
	}

	return createResolvedSessionWindow(sessionMutationContext(cmd), plan, launchPath, tmux.SessionWindowOptions{
		Name:     windowName,
		NewShell: options.newShell,
		Command:  commandArgs,
		Switch:   true,
	})
}

func splitSessionWindowArgs(args []string, options sessionWindowOptions) (string, []string, bool, error) {
	if len(args) == 0 {
		return "", nil, false, errors.New("path is required")
	}

	if options.dashSeen {
		switch options.argsLenAtDash {
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

func sessionMutationContext(cmd *cobra.Command) context.Context {
	// Do not reuse sessionResolveContext here. tmux mutations must not inherit the
	// [timeout].fetch deadline that may already have been mostly consumed by
	// git/config resolution.
	ctx := cmd.Context()
	if ctx == nil {
		return context.Background()
	}
	return ctx
}
