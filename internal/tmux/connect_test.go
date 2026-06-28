package tmux

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	resolver "github.com/jmcampanini/cmdk/internal/session"
)

type scriptedTmuxCall struct {
	args   []string
	output string
	err    error
}

type scriptedTmuxRunner struct {
	t     *testing.T
	calls []scriptedTmuxCall
}

func newScriptedTmuxRunner(t *testing.T, calls ...scriptedTmuxCall) *scriptedTmuxRunner {
	t.Helper()
	return &scriptedTmuxRunner{t: t, calls: calls}
}

func (r *scriptedTmuxRunner) Output(_ context.Context, args ...string) ([]byte, error) {
	r.t.Helper()
	if len(r.calls) == 0 {
		r.t.Fatalf("unexpected tmux call: %q", args)
	}
	call := r.calls[0]
	r.calls = r.calls[1:]
	if !slices.Equal(args, call.args) {
		r.t.Fatalf("tmux args mismatch\ngot:  %q\nwant: %q", args, call.args)
	}
	return []byte(call.output), call.err
}

func (r *scriptedTmuxRunner) done() {
	r.t.Helper()
	if len(r.calls) != 0 {
		r.t.Fatalf("%d expected tmux calls were not made; next: %q", len(r.calls), r.calls[0].args)
	}
}

func repoConnectPlan() resolver.Plan {
	return resolver.Plan{
		SessionKind:            resolver.KindRepo,
		SessionKey:             "/Users/me/Code/github.com/me/dotfiles",
		SessionDisplay:         "~/Code/github.com/me/dotfiles",
		LaunchPath:             "/Users/me/Code/github.com/me/dotfiles/main",
		PlannedTmuxSessionName: "Users/me/Code/github_com/me/dotfiles",
		PlannedTmuxWindowName:  "main",
	}
}

func directoryConnectPlan() resolver.Plan {
	return resolver.Plan{
		SessionKind:            resolver.KindDirectory,
		SessionKey:             "/Users/me/Downloads/scratch",
		SessionDisplay:         "~/Downloads/scratch",
		LaunchPath:             "/Users/me/Downloads/scratch",
		PlannedTmuxSessionName: "Users/me/Downloads/scratch",
		PlannedTmuxWindowName:  "scratch",
	}
}

func connectWithRunner(t *testing.T, runner *scriptedTmuxRunner, plan resolver.Plan) error {
	t.Helper()
	err := connector{runner: runner}.connect(context.Background(), plan)
	runner.done()
	return err
}

func TestConnectCreatesMissingRepoSession(t *testing.T) {
	plan := repoConnectPlan()
	runner := newScriptedTmuxRunner(t,
		scriptedTmuxCall{args: []string{"list-sessions", "-F", cmdkSessionKeyListFormat}, output: "$9\t/other\n"},
		scriptedTmuxCall{args: []string{"new-session", "-d", "-P", "-F", newSessionIDsFormat, "-s", plan.PlannedTmuxSessionName, "-n", plan.PlannedTmuxWindowName, "-c", plan.LaunchPath}, output: "$1\t@1\n"},
		scriptedTmuxCall{args: []string{"set-option", "-t", "$1", cmdkSessionKindOption, plan.SessionKind}},
		scriptedTmuxCall{args: []string{"set-option", "-t", "$1", cmdkSessionKeyOption, plan.SessionKey}},
		scriptedTmuxCall{args: []string{"set-option", "-t", "$1", cmdkSessionDisplayOption, plan.SessionDisplay}},
		scriptedTmuxCall{args: []string{"switch-client", "-t", "$1:@1"}},
	)

	if err := connectWithRunner(t, runner, plan); err != nil {
		t.Fatal(err)
	}
}

func TestConnectCreatesMissingDirectorySession(t *testing.T) {
	plan := directoryConnectPlan()
	runner := newScriptedTmuxRunner(t,
		scriptedTmuxCall{args: []string{"list-sessions", "-F", cmdkSessionKeyListFormat}},
		scriptedTmuxCall{args: []string{"new-session", "-d", "-P", "-F", newSessionIDsFormat, "-s", plan.PlannedTmuxSessionName, "-n", "scratch", "-c", plan.LaunchPath}, output: "$4\t@8\n"},
		scriptedTmuxCall{args: []string{"set-option", "-t", "$4", cmdkSessionKindOption, resolver.KindDirectory}},
		scriptedTmuxCall{args: []string{"set-option", "-t", "$4", cmdkSessionKeyOption, plan.SessionKey}},
		scriptedTmuxCall{args: []string{"set-option", "-t", "$4", cmdkSessionDisplayOption, plan.SessionDisplay}},
		scriptedTmuxCall{args: []string{"switch-client", "-t", "$4:@8"}},
	)

	if err := connectWithRunner(t, runner, plan); err != nil {
		t.Fatal(err)
	}
}

func TestConnectReusesManagedSessionAndExistingWindow(t *testing.T) {
	plan := repoConnectPlan()
	runner := newScriptedTmuxRunner(t,
		scriptedTmuxCall{args: []string{"list-sessions", "-F", cmdkSessionKeyListFormat}, output: "$2\t" + plan.SessionKey + "\n$3\t/other\n"},
		scriptedTmuxCall{args: []string{"list-windows", "-t", "$2", "-F", connectWindowListFormat}, output: "1\t@7\tmain\n"},
		scriptedTmuxCall{args: []string{"switch-client", "-t", "$2:@7"}},
	)

	if err := connectWithRunner(t, runner, plan); err != nil {
		t.Fatal(err)
	}
}

func TestConnectCreatesMissingWorktreeWindowInExistingRepoSession(t *testing.T) {
	plan := repoConnectPlan()
	plan.LaunchPath = "/Users/me/Code/github.com/me/dotfiles/wt-feature"
	plan.PlannedTmuxWindowName = "wt-feature"
	runner := newScriptedTmuxRunner(t,
		scriptedTmuxCall{args: []string{"list-sessions", "-F", cmdkSessionKeyListFormat}, output: "$2\t" + plan.SessionKey + "\n"},
		scriptedTmuxCall{args: []string{"list-windows", "-t", "$2", "-F", connectWindowListFormat}, output: "1\t@1\tmain\n"},
		scriptedTmuxCall{args: []string{"new-window", "-P", "-F", "#{window_id}", "-t", "$2:", "-n", "wt-feature", "-c", plan.LaunchPath}, output: "@5\n"},
		scriptedTmuxCall{args: []string{"switch-client", "-t", "$2:@5"}},
	)

	if err := connectWithRunner(t, runner, plan); err != nil {
		t.Fatal(err)
	}
}

func TestConnectDuplicateWindowNamesChooseFirstNumericIndex(t *testing.T) {
	plan := repoConnectPlan()
	runner := newScriptedTmuxRunner(t,
		scriptedTmuxCall{args: []string{"list-sessions", "-F", cmdkSessionKeyListFormat}, output: "$2\t" + plan.SessionKey + "\n"},
		scriptedTmuxCall{args: []string{"list-windows", "-t", "$2", "-F", connectWindowListFormat}, output: "3\t@3\tmain\n1\t@1\tmain\n2\t@2\tother\n"},
		scriptedTmuxCall{args: []string{"switch-client", "-t", "$2:@1"}},
	)

	if err := connectWithRunner(t, runner, plan); err != nil {
		t.Fatal(err)
	}
}

func TestConnectWindowMatchingCollapsesTmuxDoubledBackslashes(t *testing.T) {
	plan := repoConnectPlan()
	plan.PlannedTmuxWindowName = `feature\name`
	runner := newScriptedTmuxRunner(t,
		scriptedTmuxCall{args: []string{"list-sessions", "-F", cmdkSessionKeyListFormat}, output: "$2\t" + plan.SessionKey + "\n"},
		scriptedTmuxCall{args: []string{"list-windows", "-t", "$2", "-F", connectWindowListFormat}, output: "1\t@9\tfeature\\\\name\n"},
		scriptedTmuxCall{args: []string{"switch-client", "-t", "$2:@9"}},
	)

	if err := connectWithRunner(t, runner, plan); err != nil {
		t.Fatal(err)
	}
}

func TestConnectDuplicateManagedSessionsFail(t *testing.T) {
	plan := repoConnectPlan()
	runner := newScriptedTmuxRunner(t,
		scriptedTmuxCall{args: []string{"list-sessions", "-F", cmdkSessionKeyListFormat}, output: "$2\t" + plan.SessionKey + "\n$3\t" + plan.SessionKey + "\n"},
	)

	err := connectWithRunner(t, runner, plan)
	if err == nil {
		t.Fatal("expected duplicate session error")
	}
	if !strings.Contains(err.Error(), "multiple tmux sessions") {
		t.Errorf("error = %q, want duplicate session context", err.Error())
	}
}

func TestConnectRejectsControlCharactersBeforeTmuxOperations(t *testing.T) {
	plan := repoConnectPlan()
	plan.SessionKey = "/tmp/bad\nkey"
	runner := newScriptedTmuxRunner(t)

	err := connectWithRunner(t, runner, plan)
	if err == nil {
		t.Fatal("expected validation error")
	}
	if !strings.Contains(err.Error(), "session_key") || !strings.Contains(err.Error(), "control") {
		t.Errorf("error = %q, want session_key control character context", err.Error())
	}
}

func TestConnectMalformedIdentityOutputFailsBeforeCreate(t *testing.T) {
	plan := repoConnectPlan()
	runner := newScriptedTmuxRunner(t,
		scriptedTmuxCall{args: []string{"list-sessions", "-F", cmdkSessionKeyListFormat}, output: "$1\ttoo\tmany\n"},
	)

	err := connectWithRunner(t, runner, plan)
	if err == nil {
		t.Fatal("expected parse error")
	}
	if !strings.Contains(err.Error(), "list-sessions row 1") {
		t.Errorf("error = %q, want list-sessions parse context", err.Error())
	}
}

func TestConnectParsesNewSessionIDsForFollowupTargets(t *testing.T) {
	plan := repoConnectPlan()
	runner := newScriptedTmuxRunner(t,
		scriptedTmuxCall{args: []string{"list-sessions", "-F", cmdkSessionKeyListFormat}},
		scriptedTmuxCall{args: []string{"new-session", "-d", "-P", "-F", newSessionIDsFormat, "-s", plan.PlannedTmuxSessionName, "-n", plan.PlannedTmuxWindowName, "-c", plan.LaunchPath}, output: "$42\t@99\n"},
		scriptedTmuxCall{args: []string{"set-option", "-t", "$42", cmdkSessionKindOption, plan.SessionKind}},
		scriptedTmuxCall{args: []string{"set-option", "-t", "$42", cmdkSessionKeyOption, plan.SessionKey}},
		scriptedTmuxCall{args: []string{"set-option", "-t", "$42", cmdkSessionDisplayOption, plan.SessionDisplay}},
		scriptedTmuxCall{args: []string{"switch-client", "-t", "$42:@99"}},
	)

	if err := connectWithRunner(t, runner, plan); err != nil {
		t.Fatal(err)
	}
}

func TestConnectRejectsMalformedNewWindowOutput(t *testing.T) {
	plan := repoConnectPlan()
	plan.PlannedTmuxWindowName = "feature"
	runner := newScriptedTmuxRunner(t,
		scriptedTmuxCall{args: []string{"list-sessions", "-F", cmdkSessionKeyListFormat}, output: "$2\t" + plan.SessionKey + "\n"},
		scriptedTmuxCall{args: []string{"list-windows", "-t", "$2", "-F", connectWindowListFormat}, output: "1\t@1\tmain\n"},
		scriptedTmuxCall{args: []string{"new-window", "-P", "-F", "#{window_id}", "-t", "$2:", "-n", "feature", "-c", plan.LaunchPath}, output: "not-a-window-id\n"},
	)

	err := connectWithRunner(t, runner, plan)
	if err == nil {
		t.Fatal("expected parse error")
	}
	if !strings.Contains(err.Error(), "new-window output") {
		t.Errorf("error = %q, want new-window parse context", err.Error())
	}
}

func TestTmuxOutputIncludesStderrOnNonTimeoutErrors(t *testing.T) {
	bin := t.TempDir()
	tmuxPath := filepath.Join(bin, "tmux")
	if err := os.WriteFile(tmuxPath, []byte("#!/bin/sh\necho 'boom stderr' >&2\nexit 42\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin)

	_, err := tmuxOutput(context.Background(), "new-session")
	if err == nil {
		t.Fatal("expected tmux error")
	}
	if !strings.Contains(err.Error(), "boom stderr") {
		t.Errorf("error = %q, want stderr", err.Error())
	}
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("error = %T %[1]v, want wrapped *exec.ExitError", err)
	}
}
