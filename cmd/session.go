package cmd

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/jmcampanini/cmdk/internal/config"
	resolver "github.com/jmcampanini/cmdk/internal/session"
)

func newSessionCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "session",
		Short: "Resolve and manage cmdk sessions",
		Long: `Resolve and manage cmdk sessions.

Session commands turn existing directories into cmdk session plans and can
create fresh tmux windows inside cmdk-managed sessions for those plans.`,
	}
	cmd.AddCommand(newSessionResolveCommand(), newSessionWindowCommand())
	return cmd
}

func resolveSessionPlanForCommand(cmd *cobra.Command, path string) (resolver.Plan, error) {
	cfgPath, err := resolveConfigPath()
	if err != nil {
		return resolver.Plan{}, err
	}
	cfg, err := config.Load(cfgPath)
	if err != nil {
		return resolver.Plan{}, fmt.Errorf("loading config: %w", err)
	}
	return resolveSessionPlanWithConfig(cmd, path, cfg)
}

func resolveSessionPlanWithConfig(cmd *cobra.Command, path string, cfg config.Config) (resolver.Plan, error) {
	ctx, cancel := sessionResolveContext(cmd, cfg)
	defer cancel()

	plan, err := resolver.Resolve(ctx, path)
	if err != nil {
		return resolver.Plan{}, err
	}
	return plan, nil
}

func validateLaunchDirectory(path string) (string, error) {
	if path == "" {
		return "", errors.New("path is required")
	}
	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("resolving absolute path: %w", err)
	}
	absPath = filepath.Clean(absPath)
	info, err := os.Stat(absPath)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return "", fmt.Errorf("path does not exist: %s", absPath)
		}
		return "", fmt.Errorf("path is not accessible: %w", err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("path is not a directory: %s", absPath)
	}
	return absPath, nil
}

func defaultWindowNameForLaunchPath(path string) string {
	return filepath.Base(filepath.Clean(path))
}

func sessionResolveContext(cmd *cobra.Command, cfg config.Config) (context.Context, context.CancelFunc) {
	ctx := cmd.Context()
	if ctx == nil {
		ctx = context.Background()
	}
	resolveTimeout := cfg.Timeout.Fetch
	if resolveTimeout <= 0 {
		resolveTimeout = config.DefaultConfig().Timeout.Fetch
	}
	return context.WithTimeout(ctx, resolveTimeout)
}
