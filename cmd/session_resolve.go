package cmd

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

func newSessionResolveCommand() *cobra.Command {
	options := sessionResolveOptions{}
	cmd := &cobra.Command{
		Use:   "resolve <path>",
		Short: "Resolve a path to a cmdk session plan",
		Long: `Resolve a filesystem path to the cmdk session plan.

The path must exist and be a directory. The resolver classifies the path as a
repo session or directory session and computes cmdk's logical session_key. This
command does not create tmux sessions or windows.

Fields:
  session_kind                 repo or directory
  session_key                  cmdk grouping key; not tmux #{session_id}

Repo sessions use a path-based session_key so sibling Grove-style worktrees
(main, develop, master) share one cmdk session. Directory sessions use absolute
path identity.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSessionResolveCommand(cmd, args[0], options)
		},
	}
	cmd.Flags().BoolVar(&options.json, "json", false, "output the session plan as JSON")
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
