package tmux

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func setCurrentClientEnv(t *testing.T, paneID string) {
	t.Helper()
	t.Setenv("TMUX", "/tmp/tmux.sock,1,0")
	t.Setenv("TMUX_PANE", paneID)
}

func TestCurrentClientPane(t *testing.T) {
	setCurrentClientEnv(t, "%17")
	runner := newScriptedTmuxRunner(t, scriptedTmuxCall{
		args:   []string{"display-message", "-p", currentClientPaneFormat},
		output: "/dev/pts/4\t%17\n",
	})

	paneID, err := currentClientPane(context.Background(), testTmuxTimeouts.Query, runner)
	runner.done()
	if err != nil {
		t.Fatal(err)
	}
	if paneID != "%17" {
		t.Errorf("paneID = %q, want %%17", paneID)
	}
}

func TestCurrentClientPaneRejectsMissingEnvironment(t *testing.T) {
	t.Setenv("TMUX", "")
	t.Setenv("TMUX_PANE", "")
	runner := newScriptedTmuxRunner(t)

	_, err := currentClientPane(context.Background(), testTmuxTimeouts.Query, runner)
	runner.done()
	if err == nil || !strings.Contains(err.Error(), "TMUX is not set") {
		t.Fatalf("error = %v, want missing TMUX environment", err)
	}
}

func TestCurrentClientPaneRejectsMissingClient(t *testing.T) {
	setCurrentClientEnv(t, "%17")
	runner := newScriptedTmuxRunner(t, scriptedTmuxCall{
		args:   []string{"display-message", "-p", currentClientPaneFormat},
		output: "\t%17\n",
	})

	_, err := currentClientPane(context.Background(), testTmuxTimeouts.Query, runner)
	runner.done()
	if err == nil || !strings.Contains(err.Error(), "no current tmux client") {
		t.Fatalf("error = %v, want missing current client", err)
	}
}

func TestCurrentClientPaneRejectsInvalidPane(t *testing.T) {
	setCurrentClientEnv(t, "%17")
	runner := newScriptedTmuxRunner(t, scriptedTmuxCall{
		args:   []string{"display-message", "-p", currentClientPaneFormat},
		output: "/dev/pts/4\tnot-a-pane\n",
	})

	_, err := currentClientPane(context.Background(), testTmuxTimeouts.Query, runner)
	runner.done()
	if err == nil || !strings.Contains(err.Error(), "pane_id") {
		t.Fatalf("error = %v, want invalid pane", err)
	}
}

func TestCurrentClientPaneRejectsEnvironmentMismatch(t *testing.T) {
	setCurrentClientEnv(t, "%18")
	runner := newScriptedTmuxRunner(t, scriptedTmuxCall{
		args:   []string{"display-message", "-p", currentClientPaneFormat},
		output: "/dev/pts/4\t%17\n",
	})

	_, err := currentClientPane(context.Background(), testTmuxTimeouts.Query, runner)
	runner.done()
	if err == nil || !strings.Contains(err.Error(), "does not match TMUX_PANE") {
		t.Fatalf("error = %v, want pane environment mismatch", err)
	}
}

func TestCurrentClientPaneWrapsQueryFailure(t *testing.T) {
	setCurrentClientEnv(t, "%17")
	queryErr := errors.New("no server")
	runner := newScriptedTmuxRunner(t, scriptedTmuxCall{
		args: []string{"display-message", "-p", currentClientPaneFormat},
		err:  queryErr,
	})

	_, err := currentClientPane(context.Background(), testTmuxTimeouts.Query, runner)
	runner.done()
	if !errors.Is(err, queryErr) {
		t.Fatalf("error = %v, want wrapped query failure", err)
	}
}
