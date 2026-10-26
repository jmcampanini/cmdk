package cli

import (
	"errors"

	"github.com/spf13/cobra"

	resolver "github.com/jmcampanini/cmdk/internal/session"
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

func newSessionWindowCommand(root *rootOptions) *cobra.Command {
	options := sessionWindowOptions{}
	var request sessionWindowRequest
	cmd := &cobra.Command{
		Use:   "window <path> [flags] (--new | -- <command> [args...])",
		Short: "Create a tmux window in the cmdk-managed session for a path",
		Long: `Create a fresh tmux window in the cmdk-managed session for a path.

The path must be an existing directory. cmdk resolves it like "cmdk session
resolve", finds the managed session by its @cmdk_session_key option or creates
it, and creates one new window there with the path as its working directory.
Exactly one mode is required: --new opens an interactive shell, and
-- <command> [args...] runs the command with each argument shell-quoted so
metacharacters are literal (invoke a shell explicitly for pipes or
expansion). When the command exits, tmux closes the window; cmdk does not set
remain-on-exit or fall back to a shell.

The window is named after the path's base name unless --name sets it; a name
longer than behavior.window_name_max_length is shortened to that many
characters ending in …. The window stays in the background unless --switch
switches the current client to it; --switch needs a current client, while
background creation needs only a running server. cmdk never attaches from
outside tmux and never starts a server.

--switch and --name must appear before --; everything after -- is the
command. A path that starts with - must follow its own leading --. Each
invocation creates a new window and never reuses one by name.

Stdout: nothing. Stderr: diagnostics; a failure exits 1, and when --switch
fails after the window was created, the window exists in the background. The
command runs git to classify the path and tmux to find or create the session
and window, and never prompts.

` + tmuxPrerequisiteHelp + `

` + configLoadingHelpWithKeys,
		Example: `  cmdk session window . --switch --new
  cmdk session window . --switch -- make test
  cmdk session window . --name tests -- sh -lc 'npm test | tee test.log'
  cmdk session window -- -dir -- make test`,
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
			return runSessionWindowCommand(root, cmd, request, options)
		},
	}
	options.bindFlags(cmd)
	return cmd
}

// bindFlags registers the session window flags on cmd so parsing fills o.
func (o *sessionWindowOptions) bindFlags(cmd *cobra.Command) {
	cmd.Flags().BoolVar(&o.newShell, "new", false, "open an interactive shell in the window")
	cmd.Flags().StringVar(&o.name, "name", "", "window name; must not be empty")
	cmd.Flags().BoolVar(&o.switchWindow, "switch", false, "switch the current client to the window")
}

// parseSessionWindowGrammar validates the complete positional grammar before
// any command work: exactly one path before -- and exactly one of --new or a
// nonempty command after --. Cobra runs it as the Args validator, after flag
// parsing and before the tmux prerequisite. Flag values are checked by the
// runner, so a bad value is a command failure rather than a usage error.
func parseSessionWindowGrammar(cmd *cobra.Command, args []string, options sessionWindowOptions) (sessionWindowRequest, error) {
	if len(args) == 0 {
		return sessionWindowRequest{}, errors.New("path is required")
	}
	nameSet := cmd.Flags().Changed("name")
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

func runSessionWindowCommand(root *rootOptions, cmd *cobra.Command, request sessionWindowRequest, options sessionWindowOptions) error {
	if request.nameSet && options.name == "" {
		return errors.New("--name cannot be empty")
	}
	launchPath, err := resolver.ValidateLaunchDirectory(request.path)
	if err != nil {
		return err
	}
	loaded, err := root.loadConfig(cmd)
	if err != nil {
		return err
	}
	cfg := loaded.config
	plan, err := resolveSessionPlanWithConfig(cmd, launchPath, cfg)
	if err != nil {
		return err
	}
	windowName := options.name
	if !request.nameSet {
		windowName = resolver.DefaultWindowNameForLaunchPath(launchPath)
	}

	_, err = createResolvedSessionWindow(commandContext(cmd), plan, launchPath, tmux.SessionWindowOptions{
		Name:          windowName,
		NewShell:      options.newShell,
		Command:       request.commandArgs,
		Switch:        options.switchWindow,
		MaxNameLength: cfg.Behavior.WindowNameMaxLength,
		Timeouts:      tmux.TimeoutsFromConfig(cfg),
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
