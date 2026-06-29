package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/jmcampanini/cmdk/internal/config"
	"github.com/jmcampanini/cmdk/internal/pathfmt"
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
	display, err := sessionDisplayOptions(cfg)
	if err != nil {
		return resolver.Plan{}, err
	}

	ctx, cancel := sessionResolveContext(cmd, cfg)
	defer cancel()

	plan, err := resolver.Resolve(ctx, path, display)
	if err != nil {
		return resolver.Plan{}, err
	}
	return plan, nil
}

func sessionDisplayOptions(cfg config.Config) (resolver.DisplayOptions, error) {
	home, err := os.UserHomeDir()
	if err != nil && cfg.Display.ShortenHome != "" {
		return resolver.DisplayOptions{}, fmt.Errorf("cannot shorten home prefix: %w", err)
	}
	if cfg.Display.ShortenHome != "" && home != "" {
		resolvedHome, err := filepath.EvalSymlinks(home)
		if err != nil {
			return resolver.DisplayOptions{}, fmt.Errorf("cannot resolve home prefix: %w", err)
		}
		home = filepath.Clean(resolvedHome)
	}

	return resolver.DisplayOptions{
		Home:        home,
		ShortenHome: cfg.Display.ShortenHome,
		Rules:       pathfmt.CompileRules(cfg.Display.Rules),
		Truncation: pathfmt.Truncation{
			Length: cfg.Display.TruncationLength,
			Symbol: cfg.Display.TruncationSymbol,
		},
	}, nil
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
