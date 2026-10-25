package cli

import "github.com/spf13/cobra"

func newSessionCommand(root *rootOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "session",
		Short: "Resolve paths and launch windows in managed sessions",
		Long: `Resolve and manage cmdk sessions.

Session commands turn existing directories into cmdk session plans and can
create fresh tmux windows inside cmdk-managed sessions for those plans.`,
		Args: cobra.NoArgs,
		RunE: runHelp,
	}
	cmd.AddCommand(newSessionResolveCommand(root), newSessionWindowCommand(root))
	return cmd
}
