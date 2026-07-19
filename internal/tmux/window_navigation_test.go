package tmux

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func noTmuxPaneEnv(string) (string, bool) { return "", false }

func TestSwitchRelativeWindowNextUsesPaneAndSessionIDWindowIndexOrder(t *testing.T) {
	runner := newScriptedTmuxRunner(t,
		scriptedTmuxCall{args: []string{"display-message", "-p", "-t", "%5", currentWindowFormat}, output: "$2\t@4\n"},
		scriptedTmuxCall{args: []string{"list-windows", "-a", "-F", windowNavigationListFormat}, output: "$10\t1\t@10\n$2\t2\t@4\n$1\t5\t@1\n$2\t1\t@3\n"},
		scriptedTmuxCall{args: []string{"switch-client", "-t", "$10:@10"}},
	)

	err := windowNavigator{runner: runner, lookupEnv: noTmuxPaneEnv, timeouts: testTmuxTimeouts}.switchRelativeWindow(context.Background(), WindowNext, WindowSwitchOptions{PaneID: "%5"})
	runner.done()
	if err != nil {
		t.Fatal(err)
	}
}

func TestSwitchRelativeWindowPreviousMovesWithinSession(t *testing.T) {
	runner := newScriptedTmuxRunner(t,
		scriptedTmuxCall{args: []string{"display-message", "-p", "-t", "%5", currentWindowFormat}, output: "$2\t@4\n"},
		scriptedTmuxCall{args: []string{"list-windows", "-a", "-F", windowNavigationListFormat}, output: "$2\t2\t@4\n$2\t1\t@3\n$3\t1\t@9\n"},
		scriptedTmuxCall{args: []string{"switch-client", "-t", "$2:@3"}},
	)

	err := windowNavigator{runner: runner, lookupEnv: noTmuxPaneEnv, timeouts: testTmuxTimeouts}.switchRelativeWindow(context.Background(), WindowPrevious, WindowSwitchOptions{PaneID: "%5"})
	runner.done()
	if err != nil {
		t.Fatal(err)
	}
}

func TestSwitchRelativeWindowPreviousWrapsToLastWindow(t *testing.T) {
	runner := newScriptedTmuxRunner(t,
		scriptedTmuxCall{args: []string{"display-message", "-p", "-t", "%1", currentWindowFormat}, output: "$1\t@1\n"},
		scriptedTmuxCall{args: []string{"list-windows", "-a", "-F", windowNavigationListFormat}, output: "$10\t1\t@10\n$1\t1\t@1\n$2\t1\t@2\n"},
		scriptedTmuxCall{args: []string{"switch-client", "-t", "$10:@10"}},
	)

	err := windowNavigator{runner: runner, lookupEnv: noTmuxPaneEnv, timeouts: testTmuxTimeouts}.switchRelativeWindow(context.Background(), WindowPrevious, WindowSwitchOptions{PaneID: "%1"})
	runner.done()
	if err != nil {
		t.Fatal(err)
	}
}

func TestSwitchRelativeWindowNextWrapsToFirstWindow(t *testing.T) {
	runner := newScriptedTmuxRunner(t,
		scriptedTmuxCall{args: []string{"display-message", "-p", "-t", "%10", currentWindowFormat}, output: "$10\t@10\n"},
		scriptedTmuxCall{args: []string{"list-windows", "-a", "-F", windowNavigationListFormat}, output: "$10\t1\t@10\n$1\t1\t@1\n$2\t1\t@2\n"},
		scriptedTmuxCall{args: []string{"switch-client", "-t", "$1:@1"}},
	)

	err := windowNavigator{runner: runner, lookupEnv: noTmuxPaneEnv, timeouts: testTmuxTimeouts}.switchRelativeWindow(context.Background(), WindowNext, WindowSwitchOptions{PaneID: "%10"})
	runner.done()
	if err != nil {
		t.Fatal(err)
	}
}

func TestSwitchRelativeWindowSingleWindowSwitchesToItself(t *testing.T) {
	runner := newScriptedTmuxRunner(t,
		scriptedTmuxCall{args: []string{"display-message", "-p", "-t", "%1", currentWindowFormat}, output: "$1\t@1\n"},
		scriptedTmuxCall{args: []string{"list-windows", "-a", "-F", windowNavigationListFormat}, output: "$1\t1\t@1\n"},
		scriptedTmuxCall{args: []string{"switch-client", "-t", "$1:@1"}},
	)

	err := windowNavigator{runner: runner, lookupEnv: noTmuxPaneEnv, timeouts: testTmuxTimeouts}.switchRelativeWindow(context.Background(), WindowNext, WindowSwitchOptions{PaneID: "%1"})
	runner.done()
	if err != nil {
		t.Fatal(err)
	}
}

func TestSwitchRelativeWindowFallsBackToTMUXPane(t *testing.T) {
	runner := newScriptedTmuxRunner(t,
		scriptedTmuxCall{args: []string{"display-message", "-p", "-t", "%9", currentWindowFormat}, output: "$1\t@1\n"},
		scriptedTmuxCall{args: []string{"list-windows", "-a", "-F", windowNavigationListFormat}, output: "$1\t1\t@1\n$1\t2\t@2\n"},
		scriptedTmuxCall{args: []string{"switch-client", "-t", "$1:@2"}},
	)
	lookupEnv := func(name string) (string, bool) {
		if name == "TMUX_PANE" {
			return "%9", true
		}
		return "", false
	}

	err := windowNavigator{runner: runner, lookupEnv: lookupEnv, timeouts: testTmuxTimeouts}.switchRelativeWindow(context.Background(), WindowNext, WindowSwitchOptions{})
	runner.done()
	if err != nil {
		t.Fatal(err)
	}
}

func TestSwitchRelativeWindowFallsBackToDefaultTmuxContext(t *testing.T) {
	runner := newScriptedTmuxRunner(t,
		scriptedTmuxCall{args: []string{"display-message", "-p", currentWindowFormat}, output: "$1\t@1\n"},
		scriptedTmuxCall{args: []string{"list-windows", "-a", "-F", windowNavigationListFormat}, output: "$1\t1\t@1\n$1\t2\t@2\n"},
		scriptedTmuxCall{args: []string{"switch-client", "-t", "$1:@2"}},
	)

	err := windowNavigator{runner: runner, lookupEnv: noTmuxPaneEnv, timeouts: testTmuxTimeouts}.switchRelativeWindow(context.Background(), WindowNext, WindowSwitchOptions{})
	runner.done()
	if err != nil {
		t.Fatal(err)
	}
}

func TestSwitchRelativeWindowRejectsInvalidExplicitPaneID(t *testing.T) {
	runner := newScriptedTmuxRunner(t)

	err := windowNavigator{runner: runner, lookupEnv: noTmuxPaneEnv, timeouts: testTmuxTimeouts}.switchRelativeWindow(context.Background(), WindowNext, WindowSwitchOptions{PaneID: "not-a-pane"})
	runner.done()
	if err == nil || !strings.Contains(err.Error(), "--pane-id") {
		t.Fatalf("error = %v, want --pane-id validation", err)
	}
}

func TestSwitchRelativeWindowRejectsInvalidTMUXPane(t *testing.T) {
	runner := newScriptedTmuxRunner(t)
	lookupEnv := func(name string) (string, bool) {
		if name == "TMUX_PANE" {
			return "not-a-pane", true
		}
		return "", false
	}

	err := windowNavigator{runner: runner, lookupEnv: lookupEnv, timeouts: testTmuxTimeouts}.switchRelativeWindow(context.Background(), WindowNext, WindowSwitchOptions{})
	runner.done()
	if err == nil || !strings.Contains(err.Error(), "TMUX_PANE") {
		t.Fatalf("error = %v, want TMUX_PANE validation", err)
	}
}

func TestSwitchRelativeWindowMalformedListWindowsRowErrors(t *testing.T) {
	runner := newScriptedTmuxRunner(t,
		scriptedTmuxCall{args: []string{"display-message", "-p", "-t", "%1", currentWindowFormat}, output: "$1\t@1\n"},
		scriptedTmuxCall{args: []string{"list-windows", "-a", "-F", windowNavigationListFormat}, output: "bad\n"},
	)

	err := windowNavigator{runner: runner, lookupEnv: noTmuxPaneEnv, timeouts: testTmuxTimeouts}.switchRelativeWindow(context.Background(), WindowNext, WindowSwitchOptions{PaneID: "%1"})
	runner.done()
	if err == nil || !strings.Contains(err.Error(), "list-windows row 1") {
		t.Fatalf("error = %v, want row parse error", err)
	}
}

func TestSwitchRelativeWindowCurrentWindowMissingErrors(t *testing.T) {
	runner := newScriptedTmuxRunner(t,
		scriptedTmuxCall{args: []string{"display-message", "-p", "-t", "%1", currentWindowFormat}, output: "$1\t@9\n"},
		scriptedTmuxCall{args: []string{"list-windows", "-a", "-F", windowNavigationListFormat}, output: "$1\t1\t@1\n"},
	)

	err := windowNavigator{runner: runner, lookupEnv: noTmuxPaneEnv, timeouts: testTmuxTimeouts}.switchRelativeWindow(context.Background(), WindowNext, WindowSwitchOptions{PaneID: "%1"})
	runner.done()
	if !errors.Is(err, errCurrentWindowNotInWindow) {
		t.Fatalf("error = %v, want current-window-missing error", err)
	}
}
