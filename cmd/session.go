package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/jmcampanini/cmdk/internal/config"
	"github.com/jmcampanini/cmdk/internal/pathfmt"
	resolver "github.com/jmcampanini/cmdk/internal/session"
	"github.com/jmcampanini/cmdk/internal/tmux"
)

type sessionResolveOptions struct {
	json bool
}

type resolvedSessionPlan struct {
	plan   resolver.Plan
	ctx    context.Context
	cancel context.CancelFunc
}

var sessionCmd = &cobra.Command{
	Use:   "session",
	Short: "Resolve and connect cmdk sessions",
	Long: `Resolve and connect cmdk sessions.

Session commands turn existing directories into cmdk session plans and can
create or reuse tmux sessions for those plans.`,
}

var (
	sessionResolveCmd = newSessionResolveCommand()
	sessionConnectCmd = newSessionConnectCommand()
)

func newSessionResolveCommand() *cobra.Command {
	options := sessionResolveOptions{}
	cmd := &cobra.Command{
		Use:   "resolve <path>",
		Short: "Resolve a path to a cmdk session plan",
		Long: `Resolve a filesystem path to the cmdk session plan.

The path must exist and be a directory. The resolver classifies the path as a
repo session or directory session, computes cmdk's logical session_key, and
shows the tmux-safe names cmdk passes to tmux when connecting. This command does
not create tmux sessions.

Fields:
  session_kind                 repo or directory
  session_key                  cmdk grouping key; not tmux #{session_id}
  session_display              display path derived from cmdk path formatting
  launch_path                  filesystem path for the initial/current window cwd
  planned_tmux_session_name    tmux-safe name cmdk passes to tmux -s on create
  planned_tmux_window_name     tmux window name cmdk passes to tmux -n on create

Repo sessions use a path-based session_key so sibling Grove-style worktrees
(main, develop, master) share one cmdk session. Directory sessions use absolute
path identity and the directory basename as their planned tmux window name.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSessionResolveCommand(cmd, args[0], options)
		},
	}
	cmd.Flags().BoolVar(&options.json, "json", false, "output the session plan as JSON")
	return cmd
}

func newSessionConnectCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "connect <path>",
		Short: "Create or switch to a cmdk-managed tmux session for a path",
		Long: `Create or switch to a cmdk-managed tmux session for an existing directory.

The path is required. Repo worktrees share one cmdk-managed tmux session per
repo/container and use one window per worktree. Non-repo directories get one
session whose default window uses the directory basename.

cmdk recognizes managed sessions by @cmdk_session_key metadata, not by tmux
session name. When cmdk creates a session it sets @cmdk_session_kind,
@cmdk_session_key, and @cmdk_session_display. Existing managed sessions are
reused without refreshing metadata.

The final switch uses tmux switch-client and requires a current tmux client;
Phase 3 does not attach from outside tmux.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSessionConnectCommand(cmd, args[0])
		},
	}
}

func runSessionResolveCommand(cmd *cobra.Command, path string, options sessionResolveOptions) error {
	resolved, err := resolveSessionPlanForCommand(cmd, path)
	if err != nil {
		return err
	}
	defer resolved.cancel()

	if options.json {
		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetIndent("", "  ")
		return enc.Encode(resolved.plan)
	}
	return writeSessionPlan(cmd.OutOrStdout(), resolved.plan)
}

func runSessionConnectCommand(cmd *cobra.Command, path string) error {
	resolved, err := resolveSessionPlanForCommand(cmd, path)
	if err != nil {
		return err
	}
	defer resolved.cancel()

	return tmux.ConnectResolvedSession(resolved.ctx, resolved.plan)
}

func resolveSessionPlanForCommand(cmd *cobra.Command, path string) (resolvedSessionPlan, error) {
	cfgPath, err := resolveConfigPath()
	if err != nil {
		return resolvedSessionPlan{}, err
	}
	cfg, err := config.Load(cfgPath)
	if err != nil {
		return resolvedSessionPlan{}, fmt.Errorf("loading config: %w", err)
	}

	display, err := sessionDisplayOptions(cfg)
	if err != nil {
		return resolvedSessionPlan{}, err
	}

	ctx, cancel := sessionResolveContext(cmd, cfg)
	plan, err := resolver.Resolve(ctx, path, display)
	if err != nil {
		cancel()
		return resolvedSessionPlan{}, err
	}
	return resolvedSessionPlan{plan: plan, ctx: ctx, cancel: cancel}, nil
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

func writeSessionPlan(out io.Writer, plan resolver.Plan) error {
	rows := []struct {
		label string
		value string
	}{
		{"session_kind:", plan.SessionKind},
		{"session_key:", plan.SessionKey},
		{"session_display:", plan.SessionDisplay},
		{"launch_path:", plan.LaunchPath},
		{"planned_tmux_session_name:", plan.PlannedTmuxSessionName},
		{"planned_tmux_window_name:", plan.PlannedTmuxWindowName},
	}
	for _, row := range rows {
		if _, err := fmt.Fprintf(out, "%-27s %s\n", row.label, row.value); err != nil {
			return err
		}
	}
	return nil
}

func init() {
	sessionCmd.AddCommand(sessionResolveCmd, sessionConnectCmd)
	rootCmd.AddCommand(sessionCmd)
}
