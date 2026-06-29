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
	run    bool
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
	call := r.nextCall("Output", args)
	if call.run {
		r.t.Fatalf("tmux call %q used Output, want Run", args)
	}
	return []byte(call.output), call.err
}

func (r *scriptedTmuxRunner) Run(_ context.Context, args ...string) error {
	r.t.Helper()
	call := r.nextCall("Run", args)
	if !call.run {
		r.t.Fatalf("tmux call %q used Run, want Output", args)
	}
	return call.err
}

func (r *scriptedTmuxRunner) nextCall(method string, args []string) scriptedTmuxCall {
	r.t.Helper()
	if len(r.calls) == 0 {
		r.t.Fatalf("unexpected tmux %s call: %q", method, args)
	}
	call := r.calls[0]
	r.calls = r.calls[1:]
	if !slices.Equal(args, call.args) {
		r.t.Fatalf("tmux args mismatch\ngot:  %q\nwant: %q", args, call.args)
	}
	return call
}

func (r *scriptedTmuxRunner) done() {
	r.t.Helper()
	if len(r.calls) != 0 {
		r.t.Fatalf("%d expected tmux calls were not made; next: %q", len(r.calls), r.calls[0].args)
	}
}

func repoSessionWindowPlan() resolver.Plan {
	return resolver.Plan{
		SessionKind:            resolver.KindRepo,
		SessionKey:             "/Users/me/Code/github.com/me/dotfiles",
		SessionDisplay:         "~/Code/github.com/me/dotfiles",
		LaunchPath:             "/Users/me/Code/github.com/me/dotfiles/main",
		PlannedTmuxSessionName: "Users/me/Code/github_com/me/dotfiles",
		PlannedTmuxWindowName:  "main",
	}
}

func directorySessionWindowPlan() resolver.Plan {
	return resolver.Plan{
		SessionKind:            resolver.KindDirectory,
		SessionKey:             "/Users/me/Downloads/scratch",
		SessionDisplay:         "~/Downloads/scratch",
		LaunchPath:             "/Users/me/Downloads/scratch",
		PlannedTmuxSessionName: "Users/me/Downloads/scratch",
		PlannedTmuxWindowName:  "scratch",
	}
}

func createWindowWithRunner(t *testing.T, runner *scriptedTmuxRunner, plan resolver.Plan, opts SessionWindowOptions) error {
	t.Helper()
	err := sessionWindowManager{runner: runner}.createResolvedWindow(context.Background(), plan, opts)
	runner.done()
	return err
}

func attachWithRunner(t *testing.T, runner *scriptedTmuxRunner, plan resolver.Plan) error {
	t.Helper()
	err := sessionWindowManager{runner: runner}.attachResolvedSession(context.Background(), plan)
	runner.done()
	return err
}

func TestAttachResolvedSessionAttachesExistingManagedSession(t *testing.T) {
	plan := repoSessionWindowPlan()
	runner := newScriptedTmuxRunner(t,
		scriptedTmuxCall{args: []string{"list-sessions", "-F", cmdkSessionKeyListFormat}, output: "$2\t" + plan.SessionKey + "\n$3\t/other\n"},
		scriptedTmuxCall{args: []string{"attach-session", "-t", "$2"}, run: true},
	)

	if err := attachWithRunner(t, runner, plan); err != nil {
		t.Fatal(err)
	}
}

func TestAttachResolvedSessionCreatesMissingSessionThenAttaches(t *testing.T) {
	plan := repoSessionWindowPlan()
	runner := newScriptedTmuxRunner(t,
		scriptedTmuxCall{args: []string{"list-sessions", "-F", cmdkSessionKeyListFormat}, output: "$9\t/other\n"},
		scriptedTmuxCall{args: []string{"new-session", "-d", "-P", "-F", newSessionIDsFormat, "-s", plan.PlannedTmuxSessionName, "-n", plan.PlannedTmuxWindowName, "-c", plan.LaunchPath}, output: "$1\t@1\n"},
		scriptedTmuxCall{args: []string{"set-option", "-t", "$1", cmdkSessionKindOption, plan.SessionKind}},
		scriptedTmuxCall{args: []string{"set-option", "-t", "$1", cmdkSessionKeyOption, plan.SessionKey}},
		scriptedTmuxCall{args: []string{"set-option", "-t", "$1", cmdkSessionDisplayOption, plan.SessionDisplay}},
		scriptedTmuxCall{args: []string{"attach-session", "-t", "$1"}, run: true},
	)

	if err := attachWithRunner(t, runner, plan); err != nil {
		t.Fatal(err)
	}
}

func TestAttachResolvedSessionTreatsNoServerAsMissing(t *testing.T) {
	plan := directorySessionWindowPlan()
	runner := newScriptedTmuxRunner(t,
		scriptedTmuxCall{args: []string{"list-sessions", "-F", cmdkSessionKeyListFormat}, err: errors.New("tmux list-sessions: exit status 1: no server running on /tmp/tmux-501/default")},
		scriptedTmuxCall{args: []string{"new-session", "-d", "-P", "-F", newSessionIDsFormat, "-s", plan.PlannedTmuxSessionName, "-n", plan.PlannedTmuxWindowName, "-c", plan.LaunchPath}, output: "$4\t@8\n"},
		scriptedTmuxCall{args: []string{"set-option", "-t", "$4", cmdkSessionKindOption, plan.SessionKind}},
		scriptedTmuxCall{args: []string{"set-option", "-t", "$4", cmdkSessionKeyOption, plan.SessionKey}},
		scriptedTmuxCall{args: []string{"set-option", "-t", "$4", cmdkSessionDisplayOption, plan.SessionDisplay}},
		scriptedTmuxCall{args: []string{"attach-session", "-t", "$4"}, run: true},
	)

	if err := attachWithRunner(t, runner, plan); err != nil {
		t.Fatal(err)
	}
}

func TestCreateResolvedSessionWindowCreatesMissingSessionWithRequestedWindow(t *testing.T) {
	plan := repoSessionWindowPlan()
	runner := newScriptedTmuxRunner(t,
		scriptedTmuxCall{args: []string{"list-sessions", "-F", cmdkSessionKeyListFormat}, output: "$9\t/other\n"},
		scriptedTmuxCall{args: []string{"new-session", "-d", "-P", "-F", newSessionIDsFormat, "-s", plan.PlannedTmuxSessionName, "-n", plan.PlannedTmuxWindowName, "-c", plan.LaunchPath}, output: "$1\t@1\n"},
		scriptedTmuxCall{args: []string{"set-option", "-t", "$1", cmdkSessionKindOption, plan.SessionKind}},
		scriptedTmuxCall{args: []string{"set-option", "-t", "$1", cmdkSessionKeyOption, plan.SessionKey}},
		scriptedTmuxCall{args: []string{"set-option", "-t", "$1", cmdkSessionDisplayOption, plan.SessionDisplay}},
		scriptedTmuxCall{args: []string{"switch-client", "-t", "$1:@1"}},
	)

	err := createWindowWithRunner(t, runner, plan, SessionWindowOptions{NewShell: true, Switch: true})
	if err != nil {
		t.Fatal(err)
	}
}

func TestCreateResolvedSessionWindowCreatesMissingDirectorySession(t *testing.T) {
	plan := directorySessionWindowPlan()
	runner := newScriptedTmuxRunner(t,
		scriptedTmuxCall{args: []string{"list-sessions", "-F", cmdkSessionKeyListFormat}},
		scriptedTmuxCall{args: []string{"new-session", "-d", "-P", "-F", newSessionIDsFormat, "-s", plan.PlannedTmuxSessionName, "-n", "scratch", "-c", plan.LaunchPath}, output: "$4\t@8\n"},
		scriptedTmuxCall{args: []string{"set-option", "-t", "$4", cmdkSessionKindOption, resolver.KindDirectory}},
		scriptedTmuxCall{args: []string{"set-option", "-t", "$4", cmdkSessionKeyOption, plan.SessionKey}},
		scriptedTmuxCall{args: []string{"set-option", "-t", "$4", cmdkSessionDisplayOption, plan.SessionDisplay}},
		scriptedTmuxCall{args: []string{"switch-client", "-t", "$4:@8"}},
	)

	err := createWindowWithRunner(t, runner, plan, SessionWindowOptions{NewShell: true, Switch: true})
	if err != nil {
		t.Fatal(err)
	}
}

func TestCreateResolvedSessionWindowExistingSessionCreatesFreshNewWindow(t *testing.T) {
	plan := repoSessionWindowPlan()
	runner := newScriptedTmuxRunner(t,
		scriptedTmuxCall{args: []string{"list-sessions", "-F", cmdkSessionKeyListFormat}, output: "$2\t" + plan.SessionKey + "\n$3\t/other\n"},
		scriptedTmuxCall{args: []string{"new-window", "-P", "-F", "#{window_id}", "-t", "$2:", "-n", "main", "-c", plan.LaunchPath}, output: "@5\n"},
		scriptedTmuxCall{args: []string{"switch-client", "-t", "$2:@5"}},
	)

	err := createWindowWithRunner(t, runner, plan, SessionWindowOptions{NewShell: true, Switch: true})
	if err != nil {
		t.Fatal(err)
	}
}

func TestCreateResolvedSessionWindowDoesNotSearchOrReuseExistingWindows(t *testing.T) {
	plan := repoSessionWindowPlan()
	runner := newScriptedTmuxRunner(t,
		scriptedTmuxCall{args: []string{"list-sessions", "-F", cmdkSessionKeyListFormat}, output: "$2\t" + plan.SessionKey + "\n"},
		scriptedTmuxCall{args: []string{"new-window", "-P", "-F", "#{window_id}", "-t", "$2:", "-n", "main", "-c", plan.LaunchPath}, output: "@6\n"},
		scriptedTmuxCall{args: []string{"switch-client", "-t", "$2:@6"}},
	)

	err := createWindowWithRunner(t, runner, plan, SessionWindowOptions{NewShell: true, Switch: true})
	if err != nil {
		t.Fatal(err)
	}
}

func TestCreateResolvedSessionWindowCommandModeShellQuotesArgv(t *testing.T) {
	plan := repoSessionWindowPlan()
	command := []string{"echo", "hello $HOME", "it's ok"}
	wantCommand := `'echo' 'hello $HOME' 'it'\''s ok'`
	runner := newScriptedTmuxRunner(t,
		scriptedTmuxCall{args: []string{"list-sessions", "-F", cmdkSessionKeyListFormat}, output: "$2\t" + plan.SessionKey + "\n"},
		scriptedTmuxCall{args: []string{"new-window", "-P", "-F", "#{window_id}", "-t", "$2:", "-n", "main", "-c", plan.LaunchPath, wantCommand}, output: "@7\n"},
		scriptedTmuxCall{args: []string{"switch-client", "-t", "$2:@7"}},
	)

	err := createWindowWithRunner(t, runner, plan, SessionWindowOptions{Command: command, Switch: true})
	if err != nil {
		t.Fatal(err)
	}
}

func TestCreateResolvedSessionWindowCommandModeMetacharactersAreLiteral(t *testing.T) {
	plan := repoSessionWindowPlan()
	command := []string{"printf", "$HOME | tee out; rm -rf /", ">file"}
	wantCommand := `'printf' '$HOME | tee out; rm -rf /' '>file'`
	runner := newScriptedTmuxRunner(t,
		scriptedTmuxCall{args: []string{"list-sessions", "-F", cmdkSessionKeyListFormat}, output: "$2\t" + plan.SessionKey + "\n"},
		scriptedTmuxCall{args: []string{"new-window", "-P", "-F", "#{window_id}", "-t", "$2:", "-n", "main", "-c", plan.LaunchPath, wantCommand}, output: "@8\n"},
		scriptedTmuxCall{args: []string{"switch-client", "-t", "$2:@8"}},
	)

	err := createWindowWithRunner(t, runner, plan, SessionWindowOptions{Command: command, Switch: true})
	if err != nil {
		t.Fatal(err)
	}
}

func TestCreateResolvedSessionWindowCommandModeExplicitShellRemainsAvailable(t *testing.T) {
	plan := repoSessionWindowPlan()
	command := []string{"sh", "-lc", "echo hi | tee x"}
	wantCommand := `'sh' '-lc' 'echo hi | tee x'`
	runner := newScriptedTmuxRunner(t,
		scriptedTmuxCall{args: []string{"list-sessions", "-F", cmdkSessionKeyListFormat}, output: "$2\t" + plan.SessionKey + "\n"},
		scriptedTmuxCall{args: []string{"new-window", "-P", "-F", "#{window_id}", "-t", "$2:", "-n", "main", "-c", plan.LaunchPath, wantCommand}, output: "@9\n"},
		scriptedTmuxCall{args: []string{"switch-client", "-t", "$2:@9"}},
	)

	err := createWindowWithRunner(t, runner, plan, SessionWindowOptions{Command: command, Switch: true})
	if err != nil {
		t.Fatal(err)
	}
}

func TestCreateResolvedSessionWindowNameOverrideAppliesToNewShell(t *testing.T) {
	plan := repoSessionWindowPlan()
	runner := newScriptedTmuxRunner(t,
		scriptedTmuxCall{args: []string{"list-sessions", "-F", cmdkSessionKeyListFormat}, output: "$2\t" + plan.SessionKey + "\n"},
		scriptedTmuxCall{args: []string{"new-window", "-P", "-F", "#{window_id}", "-t", "$2:", "-n", "tests", "-c", plan.LaunchPath}, output: "@10\n"},
		scriptedTmuxCall{args: []string{"switch-client", "-t", "$2:@10"}},
	)

	err := createWindowWithRunner(t, runner, plan, SessionWindowOptions{Name: "tests", NewShell: true, Switch: true})
	if err != nil {
		t.Fatal(err)
	}
}

func TestCreateResolvedSessionWindowNameOverrideAppliesToCommandMode(t *testing.T) {
	plan := repoSessionWindowPlan()
	runner := newScriptedTmuxRunner(t,
		scriptedTmuxCall{args: []string{"list-sessions", "-F", cmdkSessionKeyListFormat}, output: "$2\t" + plan.SessionKey + "\n"},
		scriptedTmuxCall{args: []string{"new-window", "-P", "-F", "#{window_id}", "-t", "$2:", "-n", "claude", "-c", plan.LaunchPath, "'claude'"}, output: "@11\n"},
		scriptedTmuxCall{args: []string{"switch-client", "-t", "$2:@11"}},
	)

	err := createWindowWithRunner(t, runner, plan, SessionWindowOptions{Name: "claude", Command: []string{"claude"}, Switch: true})
	if err != nil {
		t.Fatal(err)
	}
}

func TestCreateResolvedSessionWindowCanSkipSwitch(t *testing.T) {
	plan := repoSessionWindowPlan()
	runner := newScriptedTmuxRunner(t,
		scriptedTmuxCall{args: []string{"list-sessions", "-F", cmdkSessionKeyListFormat}, output: "$2\t" + plan.SessionKey + "\n"},
		scriptedTmuxCall{args: []string{"new-window", "-P", "-F", "#{window_id}", "-t", "$2:", "-n", "main", "-c", plan.LaunchPath}, output: "@12\n"},
	)

	err := createWindowWithRunner(t, runner, plan, SessionWindowOptions{NewShell: true})
	if err != nil {
		t.Fatal(err)
	}
}

func TestCreateResolvedSessionWindowRejectsMissingMode(t *testing.T) {
	plan := repoSessionWindowPlan()
	runner := newScriptedTmuxRunner(t)

	err := createWindowWithRunner(t, runner, plan, SessionWindowOptions{})
	if err == nil {
		t.Fatal("expected mode error")
	}
	if !strings.Contains(err.Error(), "exactly one mode") {
		t.Errorf("error = %q, want mode context", err.Error())
	}
}

func TestCreateResolvedSessionWindowRejectsBothModes(t *testing.T) {
	plan := repoSessionWindowPlan()
	runner := newScriptedTmuxRunner(t)

	err := createWindowWithRunner(t, runner, plan, SessionWindowOptions{NewShell: true, Command: []string{"echo"}})
	if err == nil {
		t.Fatal("expected mode error")
	}
	if !strings.Contains(err.Error(), "exactly one mode") {
		t.Errorf("error = %q, want mode context", err.Error())
	}
}

func TestCreateResolvedSessionWindowDuplicateManagedSessionsFail(t *testing.T) {
	plan := repoSessionWindowPlan()
	runner := newScriptedTmuxRunner(t,
		scriptedTmuxCall{args: []string{"list-sessions", "-F", cmdkSessionKeyListFormat}, output: "$2\t" + plan.SessionKey + "\n$3\t" + plan.SessionKey + "\n"},
	)

	err := createWindowWithRunner(t, runner, plan, SessionWindowOptions{NewShell: true})
	if err == nil {
		t.Fatal("expected duplicate session error")
	}
	if !strings.Contains(err.Error(), "multiple tmux sessions") {
		t.Errorf("error = %q, want duplicate session context", err.Error())
	}
}

func TestCreateResolvedSessionWindowRejectsControlCharactersBeforeTmuxOperations(t *testing.T) {
	plan := repoSessionWindowPlan()
	plan.SessionKey = "/tmp/bad\nkey"
	runner := newScriptedTmuxRunner(t)

	err := createWindowWithRunner(t, runner, plan, SessionWindowOptions{NewShell: true})
	if err == nil {
		t.Fatal("expected validation error")
	}
	if !strings.Contains(err.Error(), "session_key") || !strings.Contains(err.Error(), "control") {
		t.Errorf("error = %q, want session_key control character context", err.Error())
	}
}

func TestCreateResolvedSessionWindowRejectsControlCharactersInWindowNameOverride(t *testing.T) {
	plan := repoSessionWindowPlan()
	runner := newScriptedTmuxRunner(t)

	err := createWindowWithRunner(t, runner, plan, SessionWindowOptions{Name: "bad\nname", NewShell: true})
	if err == nil {
		t.Fatal("expected validation error")
	}
	if !strings.Contains(err.Error(), "window name") || !strings.Contains(err.Error(), "control") {
		t.Errorf("error = %q, want window name control character context", err.Error())
	}
}

func TestCreateResolvedSessionWindowMalformedIdentityOutputFailsBeforeCreate(t *testing.T) {
	plan := repoSessionWindowPlan()
	runner := newScriptedTmuxRunner(t,
		scriptedTmuxCall{args: []string{"list-sessions", "-F", cmdkSessionKeyListFormat}, output: "$1\ttoo\tmany\n"},
	)

	err := createWindowWithRunner(t, runner, plan, SessionWindowOptions{NewShell: true})
	if err == nil {
		t.Fatal("expected parse error")
	}
	if !strings.Contains(err.Error(), "list-sessions row 1") {
		t.Errorf("error = %q, want list-sessions parse context", err.Error())
	}
}

func TestCreateResolvedSessionWindowParsesNewSessionIDsForFollowupTargets(t *testing.T) {
	plan := repoSessionWindowPlan()
	runner := newScriptedTmuxRunner(t,
		scriptedTmuxCall{args: []string{"list-sessions", "-F", cmdkSessionKeyListFormat}},
		scriptedTmuxCall{args: []string{"new-session", "-d", "-P", "-F", newSessionIDsFormat, "-s", plan.PlannedTmuxSessionName, "-n", plan.PlannedTmuxWindowName, "-c", plan.LaunchPath}, output: "$42\t@99\n"},
		scriptedTmuxCall{args: []string{"set-option", "-t", "$42", cmdkSessionKindOption, plan.SessionKind}},
		scriptedTmuxCall{args: []string{"set-option", "-t", "$42", cmdkSessionKeyOption, plan.SessionKey}},
		scriptedTmuxCall{args: []string{"set-option", "-t", "$42", cmdkSessionDisplayOption, plan.SessionDisplay}},
		scriptedTmuxCall{args: []string{"switch-client", "-t", "$42:@99"}},
	)

	err := createWindowWithRunner(t, runner, plan, SessionWindowOptions{NewShell: true, Switch: true})
	if err != nil {
		t.Fatal(err)
	}
}

func TestCreateResolvedSessionWindowRejectsMalformedNewSessionOutput(t *testing.T) {
	plan := repoSessionWindowPlan()
	runner := newScriptedTmuxRunner(t,
		scriptedTmuxCall{args: []string{"list-sessions", "-F", cmdkSessionKeyListFormat}},
		scriptedTmuxCall{args: []string{"new-session", "-d", "-P", "-F", newSessionIDsFormat, "-s", plan.PlannedTmuxSessionName, "-n", plan.PlannedTmuxWindowName, "-c", plan.LaunchPath}, output: "$42\tnot-window\n"},
	)

	err := createWindowWithRunner(t, runner, plan, SessionWindowOptions{NewShell: true})
	if err == nil {
		t.Fatal("expected parse error")
	}
	if !strings.Contains(err.Error(), "new-session output") {
		t.Errorf("error = %q, want new-session parse context", err.Error())
	}
}

func TestCreateResolvedSessionWindowRejectsMalformedNewWindowOutput(t *testing.T) {
	plan := repoSessionWindowPlan()
	plan.PlannedTmuxWindowName = "feature"
	runner := newScriptedTmuxRunner(t,
		scriptedTmuxCall{args: []string{"list-sessions", "-F", cmdkSessionKeyListFormat}, output: "$2\t" + plan.SessionKey + "\n"},
		scriptedTmuxCall{args: []string{"new-window", "-P", "-F", "#{window_id}", "-t", "$2:", "-n", "feature", "-c", plan.LaunchPath}, output: "not-a-window-id\n"},
	)

	err := createWindowWithRunner(t, runner, plan, SessionWindowOptions{NewShell: true})
	if err == nil {
		t.Fatal("expected parse error")
	}
	if !strings.Contains(err.Error(), "new-window output") {
		t.Errorf("error = %q, want new-window parse context", err.Error())
	}
}

func TestShellCommandFromArgvQuotesEmptyArgs(t *testing.T) {
	got := shellCommandFromArgv([]string{"printf", ""})
	want := `'printf' ''`
	if got != want {
		t.Errorf("shellCommandFromArgv() = %q, want %q", got, want)
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
