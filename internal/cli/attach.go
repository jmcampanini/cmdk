package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/jmcampanini/cmdk/internal/config"
	resolver "github.com/jmcampanini/cmdk/internal/session"
	"github.com/jmcampanini/cmdk/internal/tmux"
)

var (
	attachResolvedSession = tmux.AttachResolvedSession
	isInsideTmux          = func() bool { return os.Getenv("TMUX") != "" }
)

func newAttachCommand(root *rootOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "attach [path]",
		Short: "Attach to a cmdk-managed tmux session from outside tmux",
		Long: `Attach to the cmdk-managed tmux session for a directory from outside tmux.

With [path], cmdk resolves that directory with the same resolver as "cmdk
session resolve"; a leading ~/ expands to your home directory. Without it,
[startup].path from the configuration is required. The path must be an
existing directory. When the managed session exists, cmdk attaches to it.
Otherwise cmdk creates the session with a window named after the directory,
sets its @cmdk_session_kind and @cmdk_session_key options, and attaches.

cmdk then hands the terminal to "tmux attach-session", an external program,
so stdin, stdout, and stderr belong to tmux until the client detaches; the
attach never times out. cmdk itself writes only diagnostics to stderr and
never prompts. The command refuses to run when TMUX is set, because it is
the outer entry point into tmux, not a nested tmux command. It runs git to
classify the path and tmux to find or create the session.

` + tmuxPrerequisiteHelp + `

` + configLoadingHelpWithKeys,
		Example: `  cmdk attach
  cmdk attach ~/Code/github.com/me/project`,
		Args:    cobra.MaximumNArgs(1),
		PreRunE: requireTmux,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAttachCommand(root, cmd, args)
		},
	}
	return cmd
}

func runAttachCommand(root *rootOptions, cmd *cobra.Command, args []string) error {
	if isInsideTmux() {
		return errors.New("cmdk attach cannot run inside tmux; it is intended to enter a cmdk-managed tmux session from outside tmux")
	}

	loaded, err := root.loadConfig(cmd)
	if err != nil {
		return err
	}
	cfg := loaded.config

	path, err := attachPath(args, cfg, loaded.path)
	if err != nil {
		return err
	}
	path, err = expandAttachPath(path)
	if err != nil {
		return err
	}

	launchPath, err := resolver.ValidateLaunchDirectory(path)
	if err != nil {
		return err
	}
	plan, err := resolveSessionPlanWithConfig(cmd, launchPath, cfg)
	if err != nil {
		return err
	}
	return attachResolvedSession(commandContext(cmd), plan, launchPath, tmux.AttachOptions{
		WindowName:    resolver.DefaultWindowNameForLaunchPath(launchPath),
		MaxNameLength: cfg.Behavior.WindowNameMaxLength,
		Timeouts:      tmux.TimeoutsFromConfig(cfg),
		Terminal:      tmux.TerminalIO{Stdin: os.Stdin, Stdout: os.Stdout, Stderr: os.Stderr},
	})
}

func attachPath(args []string, cfg config.Config, cfgPath string) (string, error) {
	switch {
	case len(args) > 0:
		return args[0], nil
	case cfg.Startup.Path != "":
		return cfg.Startup.Path, nil
	default:
		return "", fmt.Errorf("startup path is not configured; set [startup].path in %s or pass a path: cmdk attach <path>", cfgPath)
	}
}

func expandAttachPath(path string) (string, error) {
	if path != "~" && !strings.HasPrefix(path, "~/") {
		return path, nil
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("expanding ~ in startup path: %w", err)
	}
	if path == "~" {
		return home, nil
	}
	return filepath.Join(home, path[2:]), nil
}
