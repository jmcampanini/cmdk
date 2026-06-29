package cmd

import (
	"context"
	"encoding/json"
	"errors"
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

type sessionWindowOptions struct {
	newShell      bool
	name          string
	nameSet       bool
	dashSeen      bool
	argsLenAtDash int
}

var createResolvedSessionWindow = tmux.CreateResolvedSessionWindow

var sessionCmd = &cobra.Command{
	Use:   "session",
	Short: "Resolve and manage cmdk sessions",
	Long: `Resolve and manage cmdk sessions.

Session commands turn existing directories into cmdk session plans and can
create fresh tmux windows inside cmdk-managed sessions for those plans.`,
}

var (
	sessionResolveCmd = newSessionResolveCommand()
	sessionWindowCmd  = newSessionWindowCommand()
)

func newSessionResolveCommand() *cobra.Command {
	options := sessionResolveOptions{}
	cmd := &cobra.Command{
		Use:   "resolve <path>",
		Short: "Resolve a path to a cmdk session plan",
		Long: `Resolve a filesystem path to the cmdk session plan.

The path must exist and be a directory. The resolver classifies the path as a
repo session or directory session, computes cmdk's logical session_key, and
shows the tmux-safe names cmdk passes to tmux when creating managed session
windows. This command does not create tmux sessions or windows.

Fields:
  session_kind                 repo or directory
  session_key                  cmdk grouping key; not tmux #{session_id}
  session_display              display path derived from cmdk path formatting
  launch_path                  filesystem path for new managed windows' cwd
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

The new window's cwd is the resolved launch_path. The default window name is the
resolved planned_tmux_window_name; --name overrides it for either mode and must
not be empty.

Command args after -- are treated as argv-style input and are shell-quoted before
being passed to tmux as its shell-command string. Shell metacharacters are
literal by default; invoke a shell explicitly for shell features, for example:

  cmdk session window . --name tests -- sh -lc 'npm test | tee test.log'

cmdk creates a fresh window every time, tracks it by the returned tmux window_id,
and switches the current tmux client to <session_id>:<window_id>.`,
		Args: cobra.MinimumNArgs(1),
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

func runSessionResolveCommand(cmd *cobra.Command, path string, options sessionResolveOptions) error {
	plan, err := resolveSessionPlanForCommand(cmd, path)
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

	plan, err := resolveSessionPlanForCommand(cmd, path)
	if err != nil {
		return err
	}

	return createResolvedSessionWindow(sessionMutationContext(cmd), plan, tmux.SessionWindowOptions{
		Name:     options.name,
		NewShell: options.newShell,
		Command:  commandArgs,
		Switch:   true,
	})
}

func splitSessionWindowArgs(args []string, options sessionWindowOptions) (path string, commandArgs []string, commandDelimiter bool, err error) {
	if len(args) == 0 {
		return "", nil, false, errors.New("path is required")
	}

	if options.dashSeen {
		switch options.argsLenAtDash {
		case 0:
			path = args[0]
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
	sessionCmd.AddCommand(sessionResolveCmd, sessionWindowCmd)
	rootCmd.AddCommand(sessionCmd)
}
