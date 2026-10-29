package cli

import "github.com/spf13/cobra"

func newCompletionZshCommand() *cobra.Command {
	var noDescriptions bool
	cmd := &cobra.Command{
		Use:   "zsh",
		Short: "Generate the zsh completion script",
		Long: `Generate the cmdk completion script for zsh.

Enable zsh completion with compinit before loading the script.

` + completionOutputHelp,
		Example:           "  autoload -U compinit; compinit\n  source <(cmdk completion zsh)",
		Args:              cobra.NoArgs,
		ValidArgsFunction: cobra.NoFileCompletions,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if noDescriptions {
				return cmd.Root().GenZshCompletionNoDesc(cmd.OutOrStdout())
			}
			return cmd.Root().GenZshCompletion(cmd.OutOrStdout())
		},
	}
	cmd.Flags().BoolVar(&noDescriptions, "no-descriptions", false, "omit completion descriptions")
	return cmd
}
