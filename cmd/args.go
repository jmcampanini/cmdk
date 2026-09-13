package cmd

import "github.com/spf13/cobra"

// runHelp makes command groups and help topics runnable so cobra validates
// their Args before showing help; non-runnable commands skip Args validation
// entirely and would silently accept unknown operands.
func runHelp(cmd *cobra.Command, _ []string) error {
	return cmd.Help()
}
