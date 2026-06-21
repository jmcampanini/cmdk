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
)

type sessionResolveOptions struct {
	json bool
}

var sessionCmd = &cobra.Command{
	Use:   "session",
	Short: "Inspect cmdk session plans",
	Long: `Inspect cmdk session plans.

Session commands are debug and inspection tools. Phase 2 resolves paths to
plans only; it does not create tmux sessions.`,
}

var sessionResolveCmd = newSessionResolveCommand()

func newSessionResolveCommand() *cobra.Command {
	options := sessionResolveOptions{}
	cmd := &cobra.Command{
		Use:   "resolve <path>",
		Short: "Resolve a path to a cmdk session plan",
		Long: `Resolve a filesystem path to the cmdk session plan that would be used
when session creation is added later.

The path must exist and be a directory. The resolver classifies the path as a
repo session or directory session, computes cmdk's logical session_key, and
shows the tmux-safe names cmdk would pass to tmux later. This command does not
create tmux sessions.

Fields:
  session_kind                 repo or directory
  session_key                  cmdk grouping key; not tmux #{session_id}
  display_label                display path derived from cmdk path formatting
  launch_path                  filesystem path for the initial/current window cwd
  planned_tmux_session_name    tmux-safe name cmdk would pass to tmux -s later
  planned_tmux_window_name     tmux window name cmdk would pass to tmux -n later

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

func runSessionResolveCommand(cmd *cobra.Command, path string, options sessionResolveOptions) error {
	cfgPath, err := resolveConfigPath()
	if err != nil {
		return err
	}
	cfg, err := config.Load(cfgPath)
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	home, err := os.UserHomeDir()
	if err != nil && cfg.Display.ShortenHome != "" {
		return fmt.Errorf("cannot shorten home prefix: %w", err)
	}
	if cfg.Display.ShortenHome != "" && home != "" {
		resolvedHome, err := filepath.EvalSymlinks(home)
		if err != nil {
			return fmt.Errorf("cannot resolve home prefix: %w", err)
		}
		home = filepath.Clean(resolvedHome)
	}
	display := resolver.DisplayOptions{
		Home:        home,
		ShortenHome: cfg.Display.ShortenHome,
		Rules:       pathfmt.CompileRules(cfg.Display.Rules),
		Truncation: pathfmt.Truncation{
			Length: cfg.Display.TruncationLength,
			Symbol: cfg.Display.TruncationSymbol,
		},
	}

	ctx := cmd.Context()
	if ctx == nil {
		ctx = context.Background()
	}
	resolveTimeout := cfg.Timeout.Fetch
	if resolveTimeout <= 0 {
		resolveTimeout = config.DefaultConfig().Timeout.Fetch
	}
	ctx, cancel := context.WithTimeout(ctx, resolveTimeout)
	defer cancel()

	plan, err := resolver.Resolve(ctx, path, display)
	if err != nil {
		return err
	}

	if options.json {
		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetIndent("", "  ")
		return enc.Encode(plan)
	}
	return writeSessionPlan(cmd.OutOrStdout(), plan)
}

func writeSessionPlan(out io.Writer, plan resolver.Plan) error {
	rows := []struct {
		label string
		value string
	}{
		{"session_kind:", plan.SessionKind},
		{"session_key:", plan.SessionKey},
		{"display_label:", plan.DisplayLabel},
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
	sessionCmd.AddCommand(sessionResolveCmd)
	rootCmd.AddCommand(sessionCmd)
}
