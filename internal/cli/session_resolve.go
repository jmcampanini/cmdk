package cli

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/spf13/cobra"

	resolver "github.com/jmcampanini/cmdk/internal/session"
)

type sessionResolveOptions struct {
	json bool
}

func newSessionResolveCommand(root *rootOptions) *cobra.Command {
	options := sessionResolveOptions{}
	cmd := &cobra.Command{
		Use:   "resolve <path>",
		Short: "Resolve a path to its cmdk session plan",
		Long: `Resolve a directory to the session plan cmdk would use for it.

The path must be an existing directory. cmdk runs git under timeout.fetch to
classify it as a repo session (a repository, worktree, or worktree container,
keyed by the repository so they share one session) or a directory session
(keyed by the canonical directory). The command reads no tmux state and
creates no session or window.

Stdout: two lines, "session_kind:" (repo or directory) and "session_key:"
(cmdk's grouping key, not a tmux session ID), or with --json one object with
the fields session_kind and session_key. Stderr: diagnostics; a path that is
not a directory or a git failure exits 1. The command never prompts.

` + configLoadingHelpWithKeys,
		Example: `  cmdk session resolve ~/Code/github.com/me/project
  cmdk session resolve --json .`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSessionResolveCommand(root, cmd, args[0], options)
		},
	}
	cmd.Flags().BoolVar(&options.json, "json", false, "print the plan as JSON")
	return cmd
}

func runSessionResolveCommand(root *rootOptions, cmd *cobra.Command, path string, options sessionResolveOptions) error {
	plan, err := resolveSessionPlanForCommand(root, cmd, path)
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
	}
	for _, row := range rows {
		if _, err := fmt.Fprintf(out, "%-27s %s\n", row.label, row.value); err != nil {
			return err
		}
	}
	return nil
}

func resolveSessionPlanForCommand(root *rootOptions, cmd *cobra.Command, path string) (resolver.Plan, error) {
	loaded, err := root.loadConfig(cmd)
	if err != nil {
		return resolver.Plan{}, err
	}
	return resolveSessionPlanWithConfig(cmd, path, loaded.config)
}
