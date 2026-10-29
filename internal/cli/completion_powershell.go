package cli

import "github.com/spf13/cobra"

func newCompletionPowerShellCommand() *cobra.Command {
	var noDescriptions bool
	cmd := &cobra.Command{
		Use:   "powershell",
		Short: "Generate the PowerShell completion script",
		Long: `Generate the cmdk completion script for PowerShell.

Load the script into the current PowerShell session with Invoke-Expression.

` + completionOutputHelp,
		Example:           "  cmdk completion powershell | Out-String | Invoke-Expression",
		Args:              cobra.NoArgs,
		ValidArgsFunction: cobra.NoFileCompletions,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if noDescriptions {
				return cmd.Root().GenPowerShellCompletion(cmd.OutOrStdout())
			}
			return cmd.Root().GenPowerShellCompletionWithDesc(cmd.OutOrStdout())
		},
	}
	cmd.Flags().BoolVar(&noDescriptions, "no-descriptions", false, "omit completion descriptions")
	return cmd
}
