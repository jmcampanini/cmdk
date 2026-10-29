package cli

import (
	"testing"

	"github.com/spf13/cobra"
)

// TestEveryApplicationCommandDeclaresGrammar checks the declarations cobra
// needs to reject stray operands before any command work: an Args validator
// on every command but the root, whose operands cobra itself resolves to
// subcommands, and a RunE on every group and help topic, because cobra
// prints help for a non-runnable command before it validates operands.
func TestEveryApplicationCommandDeclaresGrammar(t *testing.T) {
	root := newRootCommand()
	if !root.DisableSuggestions {
		t.Error("root must set DisableSuggestions so an unknown command fails the same way at every level")
	}
	if root.Args != nil {
		t.Error("root must leave Args unset so cobra rejects an unknown subcommand during discovery")
	}
	if root.Version != "" {
		t.Error("root must leave Cobra's Version unset and own --version")
	}
	if root.Flags().Lookup("version") == nil || root.Flags().Lookup("help") == nil {
		t.Error("root must register --help and --version during construction")
	}

	commands := map[string]*cobra.Command{}
	collectApplicationCommands(root, commands)
	for path, command := range commands {
		if command.HasParent() && command.Args == nil {
			t.Errorf("%s has no Args validator", path)
		}
		if command.HasSubCommands() && command.RunE == nil {
			t.Errorf("%s has subcommands but no RunE", path)
		}
		if command.GroupID == topicGroupID && command.RunE == nil {
			t.Errorf("%s is a help topic but is not runnable", path)
		}
	}
}

// collectApplicationCommands walks a fresh command tree into path-keyed entries.
func collectApplicationCommands(command *cobra.Command, into map[string]*cobra.Command) {
	into[command.CommandPath()] = command
	for _, child := range command.Commands() {
		collectApplicationCommands(child, into)
	}
}
