package cmd

import (
	"context"
	"errors"
	"io"
	"testing"
	"time"

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

func stubTmuxPrerequisite(t *testing.T, check func(context.Context) error) {
	t.Helper()
	oldCheck := checkTmuxPrerequisite
	checkTmuxPrerequisite = check
	t.Cleanup(func() { checkTmuxPrerequisite = oldCheck })
}

func forbidTmuxPrerequisite(t *testing.T) {
	t.Helper()
	stubTmuxPrerequisite(t, func(context.Context) error {
		return errors.New("unexpected tmux prerequisite check")
	})
}

func TestTmuxBackedCommandsCheckPrerequisite(t *testing.T) {
	dir := t.TempDir()
	writeActionRunConfig(t, `
[[actions]]
name = "tmux prerequisite"
matches = "root"
launch_path = "`+dir+`"
cmd = "true"
`)

	calls := 0
	stubTmuxPrerequisite(t, func(context.Context) error {
		calls++
		return nil
	})
	stubActionRunCurrentPane(t, func(context.Context, time.Duration) (string, error) {
		return "%17", nil
	})

	commands := []struct {
		command *cobra.Command
		args    []string
	}{
		{command: newRootCommand()},
		{command: newActionRunCommand(), args: []string{"tmux prerequisite"}},
		{command: newAttachCommand()},
		{command: newSessionWindowCommand()},
		{command: windowLeafCommand(t, "next")},
		{command: windowLeafCommand(t, "previous")},
	}
	for _, test := range commands {
		if test.command.PreRunE == nil {
			t.Fatalf("%s has no tmux prerequisite check", test.command.CommandPath())
		}
		if err := test.command.PreRunE(test.command, test.args); err != nil {
			t.Fatalf("%s prerequisite: %v", test.command.CommandPath(), err)
		}
	}
	if calls != len(commands) {
		t.Fatalf("prerequisite calls = %d, want %d", calls, len(commands))
	}
}

func TestTmuxFreeCommandDoesNotCheckPrerequisite(t *testing.T) {
	useTempConfigHome(t)
	forbidTmuxPrerequisite(t)

	command := newRootCommand()
	command.SetOut(io.Discard)
	command.SetErr(io.Discard)
	command.SetArgs([]string{"shorten", "/usr/local/bin"})
	if err := command.Execute(); err != nil {
		t.Fatal(err)
	}
}

func TestHelpDoesNotCheckTmuxPrerequisite(t *testing.T) {
	forbidTmuxPrerequisite(t)

	command := newRootCommand()
	command.SetOut(io.Discard)
	command.SetErr(io.Discard)
	command.SetArgs([]string{"--help"})
	if err := command.Execute(); err != nil {
		t.Fatal(err)
	}
}

func TestVersionDoesNotCheckTmuxPrerequisite(t *testing.T) {
	forbidTmuxPrerequisite(t)

	command := newRootCommand()
	command.SetOut(io.Discard)
	command.SetErr(io.Discard)
	command.SetArgs([]string{"--version"})
	if err := command.Execute(); err != nil {
		t.Fatal(err)
	}
}

func TestArgumentValidationRunsBeforeTmuxPrerequisite(t *testing.T) {
	called := false
	stubTmuxPrerequisite(t, func(context.Context) error {
		called = true
		return nil
	})

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
