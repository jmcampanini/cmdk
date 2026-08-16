package cmd

import (
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

// rejectUnknownOperands is the Args validator for commands whose operands can
// only be subcommand names. Operands are quoted with %q so adversarial input
// cannot inject terminal control sequences into the diagnostic.
func rejectUnknownOperands(cmd *cobra.Command, args []string) error {
	if len(args) == 0 {
		return nil
	}
	if cmd.SuggestionsMinimumDistance <= 0 {
		cmd.SuggestionsMinimumDistance = 2
	}
	message := fmt.Sprintf("unknown command %q for %q", args[0], cmd.CommandPath())
	if suggestions := cmd.SuggestionsFor(args[0]); len(suggestions) > 0 {
		message += "\n\nDid you mean this?\n\t" + strings.Join(suggestions, "\n\t")
	}
	return errors.New(message)
}

// runHelp makes command groups and help topics runnable so cobra validates
// their Args before showing help; non-runnable commands skip Args validation
// entirely and would silently accept unknown operands.
func runHelp(cmd *cobra.Command, _ []string) error {
	return cmd.Help()
}
