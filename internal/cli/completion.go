package cli

import "github.com/spf13/cobra"

const completionOutputHelp = `Stdout: a shell script, including completion descriptions unless
--no-descriptions is set. Stderr: diagnostics. The command never prompts,
needs no terminal, reads no configuration or other files, and writes no files.
The example loads completions into the current shell session.`

func newCompletionCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "completion",
		Short: "Generate shell completion scripts",
		Long: `Generate a shell completion script for cmdk.

Choose a shell below and read that command's --help for how to load its
script. A bare "cmdk completion" prints this help and exits 0.

` + helpOutputContract,
		Args:              cobra.NoArgs,
		RunE:              runHelp,
		ValidArgsFunction: cobra.NoFileCompletions,
	}
	cmd.AddCommand(newCompletionBashCommand(), newCompletionZshCommand(), newCompletionFishCommand(), newCompletionPowerShellCommand())
	return cmd
}
