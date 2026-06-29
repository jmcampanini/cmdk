package cmd

import "github.com/spf13/cobra"

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
