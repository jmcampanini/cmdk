package cmd

import (
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// TestEveryApplicationCommandDeclaresGrammar checks the two declarations
// cobra needs to reject stray operands before any command work: an Args
// validator on every command but the root, whose operands cobra itself
// resolves to subcommands, and a RunE on every group, because cobra prints
// help for a non-runnable command before it validates operands.
func TestEveryApplicationCommandDeclaresGrammar(t *testing.T) {
	commands := map[string]*cobra.Command{}
	collectApplicationCommands(newRootCommand(), commands)

	for path, command := range commands {
		if command.HasParent() && command.Args == nil {
			t.Errorf("%s has no Args validator", path)
		}
		if command.HasSubCommands() && command.RunE == nil {
			t.Errorf("%s has subcommands but no RunE", path)
		}
	}
}

// collectApplicationCommands walks a command tree into path-keyed entries,
// skipping the help and completion commands that cobra owns.
func collectApplicationCommands(command *cobra.Command, into map[string]*cobra.Command) {
	name := command.Name()
	if command.HasParent() && (name == "help" || name == "completion" || strings.HasPrefix(name, "__complete")) {
		return
	}
	into[command.CommandPath()] = command
	for _, child := range command.Commands() {
		collectApplicationCommands(child, into)
	}
}
