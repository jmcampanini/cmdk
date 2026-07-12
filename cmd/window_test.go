package cmd

import (
	"context"
	"errors"
	"testing"

	"github.com/jmcampanini/cmdk/internal/tmux"
)

func stubWindowSwitcher(t *testing.T, fn func(context.Context, tmux.WindowDirection, tmux.WindowSwitchOptions) error) {
	t.Helper()
	oldSwitch := switchRelativeWindow
	oldPaneID := paneID
	oldCheck := checkTmuxPrerequisite
	switchRelativeWindow = fn
	paneID = ""
	checkTmuxPrerequisite = func(context.Context) error { return nil }
	t.Cleanup(func() {
		switchRelativeWindow = oldSwitch
		paneID = oldPaneID
		checkTmuxPrerequisite = oldCheck
	})
}

func TestWindowNextCommandCallsSwitcher(t *testing.T) {
	called := false
	stubWindowSwitcher(t, func(ctx context.Context, direction tmux.WindowDirection, opts tmux.WindowSwitchOptions) error {
		called = true
		if _, ok := ctx.Deadline(); ok {
			t.Error("window command context should not have an implicit deadline")
		}
		if direction != tmux.WindowNext {
			t.Errorf("direction = %v, want WindowNext", direction)
		}
		if opts.PaneID != "%5" {
			t.Errorf("PaneID = %q, want %%5", opts.PaneID)
		}
		return nil
	})

	cmd := newWindowCommand()
	cmd.SetArgs([]string{"next", "--pane-id", "%5"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !called {
		t.Fatal("switchRelativeWindow was not called")
	}
}

func TestWindowPreviousAliasCallsSwitcher(t *testing.T) {
	called := false
	stubWindowSwitcher(t, func(_ context.Context, direction tmux.WindowDirection, opts tmux.WindowSwitchOptions) error {
		called = true
		if direction != tmux.WindowPrevious {
			t.Errorf("direction = %v, want WindowPrevious", direction)
		}
		if opts.PaneID != "" {
			t.Errorf("PaneID = %q, want empty fallback", opts.PaneID)
		}
		return nil
	})

	cmd := newWindowCommand()
	cmd.SetArgs([]string{"prev"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !called {
		t.Fatal("switchRelativeWindow was not called")
	}
}

func TestWindowCommandPersistentPaneIDBeforeSubcommand(t *testing.T) {
	stubWindowSwitcher(t, func(_ context.Context, _ tmux.WindowDirection, opts tmux.WindowSwitchOptions) error {
		if opts.PaneID != "%7" {
			t.Errorf("PaneID = %q, want %%7", opts.PaneID)
		}
		return nil
	})

	cmd := newWindowCommand()
	cmd.SetArgs([]string{"--pane-id", "%7", "next"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
}

func TestWindowCommandRejectsArgsWithoutSwitching(t *testing.T) {
	stubWindowSwitcher(t, func(context.Context, tmux.WindowDirection, tmux.WindowSwitchOptions) error {
		t.Fatal("switchRelativeWindow should not be called")
		return nil
	})

	cmd := newWindowCommand()
	cmd.SetArgs([]string{"next", "extra"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected arg error")
	}
}

func TestWindowCommandPropagatesSwitcherError(t *testing.T) {
	wantErr := errors.New("tmux failed")
	stubWindowSwitcher(t, func(context.Context, tmux.WindowDirection, tmux.WindowSwitchOptions) error {
		return wantErr
	})

	cmd := newWindowCommand()
	cmd.SetArgs([]string{"next"})
	err := cmd.Execute()
	if !errors.Is(err, wantErr) {
		t.Fatalf("error = %v, want %v", err, wantErr)
	}
}
