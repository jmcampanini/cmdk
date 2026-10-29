package cli

import "github.com/spf13/cobra"

// Command groups in the root help. Every application-owned command belongs
// to one so cobra lists topics apart from commands that do work.
const (
	commandGroupID = "commands"
	topicGroupID   = "topics"
)

const helpOutputContract = `Stdout: plain-text help. Stderr: diagnostics. No machine-readable format is
provided. The command never prompts, needs no terminal, and reads or writes
no files.`

// runHelp makes command groups and help topics runnable so cobra validates
// their Args before showing help; non-runnable commands skip Args validation
// entirely and would silently accept unknown operands.
func runHelp(cmd *cobra.Command, _ []string) error {
	return cmd.Help()
}

// newHelpTopic builds a help topic: an application-owned command that does no
// work and prints the same text whether run directly or through "cmdk help".
func newHelpTopic(use string, short string, long string) *cobra.Command {
	return &cobra.Command{
		Use:     use,
		Short:   short,
		Long:    long + "\n\n" + helpOutputContract,
		GroupID: topicGroupID,
		Args:    cobra.NoArgs,
		RunE:    runHelp,
	}
}
