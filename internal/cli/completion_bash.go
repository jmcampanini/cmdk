package cli

import "github.com/spf13/cobra"

func newCompletionBashCommand() *cobra.Command {
	var noDescriptions bool
	cmd := &cobra.Command{
		Use:   "bash",
		Short: "Generate the bash completion script",
		Long: `Generate the cmdk completion script for bash.

Install bash-completion with your package manager before loading the script.

` + completionOutputHelp,
		Example:           "  source <(cmdk completion bash)",
		Args:              cobra.NoArgs,
		ValidArgsFunction: cobra.NoFileCompletions,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Root().GenBashCompletionV2(cmd.OutOrStdout(), !noDescriptions)
		},
	}
	cmd.Flags().BoolVar(&noDescriptions, "no-descriptions", false, "omit completion descriptions")
	return cmd
}
