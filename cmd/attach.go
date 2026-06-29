package cmd

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/jmcampanini/cmdk/internal/config"
	"github.com/jmcampanini/cmdk/internal/tmux"
)

var (
	attachResolvedSession = tmux.AttachResolvedSession
	isInsideTmux          = func() bool { return os.Getenv("TMUX") != "" }
)

var attachCmd = newAttachCommand()

func newAttachCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "attach [path]",
		Short: "Attach to a cmdk-managed tmux session from outside tmux",
		Long: `Attach to a cmdk-managed tmux session from outside tmux.

With [path], cmdk resolves that directory using the same session resolver as
"cmdk session resolve". Without [path], [startup].path in the config file is
required. If the managed session already exists, cmdk attaches to it. Otherwise
cmdk creates the managed session and then attaches.

This command refuses to run inside tmux because it is intended as the outer entry
point into tmux, not as a nested tmux command.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAttachCommand(cmd, args)
		},
	}
	return cmd
}

func runAttachCommand(cmd *cobra.Command, args []string) error {
	if len(args) > 1 {
		return errors.New("expected at most one path")
	}
	if isInsideTmux() {
		return errors.New("cmdk attach cannot run inside tmux; it is intended to enter a cmdk-managed tmux session from outside tmux")
	}

	cfgPath, err := resolveConfigPath()
	if err != nil {
		return err
	}
	cfg, err := config.Load(cfgPath)
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	startupPath, err := attachPath(args, cfg, cfgPath)
	if err != nil {
		return err
	}
	startupPath, err = expandAttachPath(startupPath)
	if err != nil {
		return err
	}

	plan, err := resolveSessionPlanWithConfig(cmd, startupPath, cfg)
	if err != nil {
		return err
	}
	return attachResolvedSession(sessionMutationContext(cmd), plan)
}

func attachPath(args []string, cfg config.Config, cfgPath string) (string, error) {
	if len(args) > 0 {
		return args[0], nil
	}
	if cfg.Startup.Path != "" {
		return cfg.Startup.Path, nil
	}
	return "", fmt.Errorf("startup path is not configured; set [startup].path in %s or pass a path: cmdk attach <path>", cfgPath)
}

func expandAttachPath(path string) (string, error) {
	if path == "~" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("expanding ~ in startup path: %w", err)
		}
		return home, nil
	}
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("expanding ~ in startup path: %w", err)
		}
		return filepath.Join(home, path[2:]), nil
	}
	return path, nil
}

func init() {
	rootCmd.AddCommand(attachCmd)
}
