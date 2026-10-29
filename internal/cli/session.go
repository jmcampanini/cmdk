package cli

import "github.com/spf13/cobra"

func newSessionCommand(root *rootOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "session",
		Short: "Resolve paths and open windows in managed sessions",
		Long: `Resolve a directory to its cmdk-managed session and open windows there.

A managed session is one cmdk created for a repository or a directory; see
"cmdk help launching" for how paths map to sessions. A bare "cmdk session"
prints this help and exits 0.

` + helpOutputContract,
		Args: cobra.NoArgs,
		RunE: runHelp,
	}
	cmd.AddCommand(newSessionResolveCommand(root), newSessionWindowCommand(root))
	return cmd
}
