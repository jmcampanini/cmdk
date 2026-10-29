package cli

import "github.com/spf13/cobra"

func newCompletionFishCommand() *cobra.Command {
	var noDescriptions bool
	cmd := &cobra.Command{
		Use:   "fish",
		Short: "Generate the fish completion script",
		Long: `Generate the cmdk completion script for fish.

Load the script into the current fish session with source.

` + completionOutputHelp,
		Example:           "  cmdk completion fish | source",
		Args:              cobra.NoArgs,
		ValidArgsFunction: cobra.NoFileCompletions,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Root().GenFishCompletion(cmd.OutOrStdout(), !noDescriptions)
		},
	}
	cmd.Flags().BoolVar(&noDescriptions, "no-descriptions", false, "omit completion descriptions")
	return cmd
}
