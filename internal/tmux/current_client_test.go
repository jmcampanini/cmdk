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

func TestCurrentClient(t *testing.T) {
	setCurrentClientEnv(t, "%17")
	runner := newScriptedTmuxRunner(t,
		scriptedTmuxCall{
			args:   []string{"display-message", "-p", currentClientFormat},
			output: "/dev/pts/4\t%17\n",
		},
		scriptedTmuxCall{
			args:   []string{"list-clients", "-F", currentClientFormat},
			output: "/dev/pts/4\t%17\n",
		},
	)

	target, err := currentClient(context.Background(), testTmuxTimeouts.Query, runner)
	if err != nil {
		t.Fatal(err)
	}
	runner.done()
	want := ClientTarget{Name: "/dev/pts/4", PaneID: "%17"}
	if target != want {
		t.Fatalf("target = %#v, want %#v", target, want)
	}
}

func TestCurrentClientRejectsMissingEnvironment(t *testing.T) {
	t.Setenv("TMUX", "")
	t.Setenv("TMUX_PANE", "")
	runner := newScriptedTmuxRunner(t)

	_, err := currentClient(context.Background(), testTmuxTimeouts.Query, runner)
	runner.done()
	if err == nil || !strings.Contains(err.Error(), "TMUX is not set") {
		t.Fatalf("error = %v, want missing TMUX environment", err)
	}
}

func TestCurrentClientRejectsMissingClient(t *testing.T) {
	setCurrentClientEnv(t, "%17")
	runner := newScriptedTmuxRunner(t, scriptedTmuxCall{
		args:   []string{"display-message", "-p", currentClientFormat},
		output: "\t%17\n",
	})

	_, err := currentClient(context.Background(), testTmuxTimeouts.Query, runner)
	runner.done()
	if err == nil || !strings.Contains(err.Error(), "no current tmux client") {
		t.Fatalf("error = %v, want missing-client rejection", err)
	}
}

func TestCurrentClientRejectsInvalidPane(t *testing.T) {
	setCurrentClientEnv(t, "%17")
	runner := newScriptedTmuxRunner(t, scriptedTmuxCall{
		args:   []string{"display-message", "-p", currentClientFormat},
		output: "/dev/pts/4\tnot-pane\n",
	})

	_, err := currentClient(context.Background(), testTmuxTimeouts.Query, runner)
	runner.done()
	if err == nil || !strings.Contains(err.Error(), "pane_id") {
		t.Fatalf("error = %v, want pane validation", err)
	}
}

func TestCurrentClientRejectsEnvironmentMismatch(t *testing.T) {
	setCurrentClientEnv(t, "%18")
	runner := newScriptedTmuxRunner(t, scriptedTmuxCall{
		args:   []string{"display-message", "-p", currentClientFormat},
		output: "/dev/pts/4\t%17\n",
	})

	_, err := currentClient(context.Background(), testTmuxTimeouts.Query, runner)
	runner.done()
	if err == nil || !strings.Contains(err.Error(), "does not match TMUX_PANE") {
		t.Fatalf("error = %v, want pane environment mismatch", err)
	}
}

func TestCurrentClientRejectsClientAttachedToDifferentPane(t *testing.T) {
	setCurrentClientEnv(t, "%17")
	runner := newScriptedTmuxRunner(t,
		scriptedTmuxCall{
			args:   []string{"display-message", "-p", currentClientFormat},
			output: "/dev/pts/4\t%17\n",
		},
		scriptedTmuxCall{
			args:   []string{"list-clients", "-F", currentClientFormat},
			output: "/dev/pts/4\t%99\n",
		},
	)

	_, err := currentClient(context.Background(), testTmuxTimeouts.Query, runner)
	runner.done()
	if err == nil || !strings.Contains(err.Error(), "not invoking pane") {
		t.Fatalf("error = %v, want attached-pane mismatch", err)
	}
}

func TestCurrentClientWrapsQueryFailure(t *testing.T) {
	setCurrentClientEnv(t, "%17")
	queryErr := errors.New("query failed")
	runner := newScriptedTmuxRunner(t, scriptedTmuxCall{
		args: []string{"display-message", "-p", currentClientFormat},
		err:  queryErr,
	})

	_, err := currentClient(context.Background(), testTmuxTimeouts.Query, runner)
	runner.done()
	if !errors.Is(err, queryErr) || !strings.Contains(err.Error(), "current tmux client") {
		t.Fatalf("error = %v, want wrapped query failure", err)
	}
}
