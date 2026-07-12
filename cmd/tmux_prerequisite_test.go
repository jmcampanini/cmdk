package cmd

import (
	"context"
	"errors"
	"io"
	"testing"

	"github.com/spf13/cobra"
)

func windowLeafCommand(t *testing.T, name string) *cobra.Command {
	t.Helper()
	parent := newWindowCommand()
	for _, child := range parent.Commands() {
		if child.Name() == name {
			return child
		}
	}
	t.Fatalf("window command %q not found", name)
	return nil
}

func TestTmuxBackedCommandsCheckPrerequisite(t *testing.T) {
	oldCheck := checkTmuxPrerequisite
	calls := 0
	checkTmuxPrerequisite = func(context.Context) error {
		calls++
		return nil
	}
	t.Cleanup(func() { checkTmuxPrerequisite = oldCheck })

	commands := []*cobra.Command{
		newRootCommand(),
		newAttachCommand(),
		newSessionWindowCommand(),
		windowLeafCommand(t, "next"),
		windowLeafCommand(t, "previous"),
	}
	for _, command := range commands {
		if command.PreRunE == nil {
			t.Fatalf("%s has no tmux prerequisite check", command.CommandPath())
		}
		if err := command.PreRunE(command, nil); err != nil {
			t.Fatalf("%s prerequisite: %v", command.CommandPath(), err)
		}
	}
	if calls != len(commands) {
		t.Fatalf("prerequisite calls = %d, want %d", calls, len(commands))
	}
}

func TestTmuxFreeCommandDoesNotCheckPrerequisite(t *testing.T) {
	useTempConfigHome(t)
	oldCheck := checkTmuxPrerequisite
	checkTmuxPrerequisite = func(context.Context) error {
		return errors.New("unexpected tmux prerequisite check")
	}
	t.Cleanup(func() { checkTmuxPrerequisite = oldCheck })

	command := newRootCommand()
	command.SetOut(io.Discard)
	command.SetErr(io.Discard)
	command.SetArgs([]string{"shorten", "/usr/local/bin"})
	if err := command.Execute(); err != nil {
		t.Fatal(err)
	}
}

func TestHelpDoesNotCheckTmuxPrerequisite(t *testing.T) {
	oldCheck := checkTmuxPrerequisite
	checkTmuxPrerequisite = func(context.Context) error {
		return errors.New("unexpected tmux prerequisite check")
	}
	t.Cleanup(func() { checkTmuxPrerequisite = oldCheck })

	command := newRootCommand()
	command.SetOut(io.Discard)
	command.SetErr(io.Discard)
	command.SetArgs([]string{"--help"})
	if err := command.Execute(); err != nil {
		t.Fatal(err)
	}
}

func TestVersionDoesNotCheckTmuxPrerequisite(t *testing.T) {
	oldCheck := checkTmuxPrerequisite
	checkTmuxPrerequisite = func(context.Context) error {
		return errors.New("unexpected tmux prerequisite check")
	}
	t.Cleanup(func() { checkTmuxPrerequisite = oldCheck })

	command := newRootCommand()
	command.SetOut(io.Discard)
	command.SetErr(io.Discard)
	command.SetArgs([]string{"--version"})
	if err := command.Execute(); err != nil {
		t.Fatal(err)
	}
}

func TestArgumentValidationRunsBeforeTmuxPrerequisite(t *testing.T) {
	oldCheck := checkTmuxPrerequisite
	called := false
	checkTmuxPrerequisite = func(context.Context) error {
		called = true
		return nil
	}
	t.Cleanup(func() { checkTmuxPrerequisite = oldCheck })

	command := newWindowCommand()
	command.SetOut(io.Discard)
	command.SetErr(io.Discard)
	command.SetArgs([]string{"next", "extra"})
	if err := command.Execute(); err == nil {
		t.Fatal("expected argument validation error")
	}
	if called {
		t.Fatal("tmux prerequisite ran before argument validation")
	}
}
