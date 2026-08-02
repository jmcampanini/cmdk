package tmux

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/jmcampanini/cmdk/internal/cmdrun"
)

func TestRequireRunningServer(t *testing.T) {
	runner := newScriptedTmuxRunner(t, scriptedTmuxCall{
		args:   []string{"list-sessions", "-F", "#{session_id}"},
		output: "$0\n",
	})

	if err := requireRunningServer(context.Background(), testTmuxTimeouts.Query, runner); err != nil {
		t.Fatal(err)
	}
	runner.done()
}

func TestRequireRunningServerReportsMissingServer(t *testing.T) {
	runner := newScriptedTmuxRunner(t, scriptedTmuxCall{
		args: []string{"list-sessions", "-F", "#{session_id}"},
		err: &cmdrun.CommandError{
			Op:       "tmux list-sessions",
			Kind:     cmdrun.KindExit,
			ExitCode: 1,
			Err:      errors.New("exit status 1"),
		},
	})

	err := requireRunningServer(context.Background(), testTmuxTimeouts.Query, runner)
	runner.done()
	if err == nil || !strings.Contains(err.Error(), "no running tmux server") {
		t.Fatalf("error = %v, want no running tmux server", err)
	}
}

func TestRequireRunningServerPreservesUnexpectedFailures(t *testing.T) {
	runner := newScriptedTmuxRunner(t, scriptedTmuxCall{
		args: []string{"list-sessions", "-F", "#{session_id}"},
		err: &cmdrun.CommandError{
			Op:   "tmux list-sessions",
			Kind: cmdrun.KindTimeout,
			Err:  errors.New("deadline exceeded"),
		},
	})

	err := requireRunningServer(context.Background(), testTmuxTimeouts.Query, runner)
	runner.done()
	if err == nil || strings.Contains(err.Error(), "no running tmux server") {
		t.Fatalf("error = %v, want the original failure, not a missing-server verdict", err)
	}
	var cmdErr *cmdrun.CommandError
	if !errors.As(err, &cmdErr) || cmdErr.Kind != cmdrun.KindTimeout {
		t.Fatalf("error = %v, want the original command error preserved", err)
	}
}
