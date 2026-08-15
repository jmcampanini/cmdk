package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"

	"github.com/jmcampanini/cmdk/internal/config"
	"github.com/jmcampanini/cmdk/internal/execute"
	"github.com/jmcampanini/cmdk/internal/item"
	"github.com/jmcampanini/cmdk/internal/tmux"
)

func writeActionRunConfig(t *testing.T, content string) {
	t.Helper()
	xdg := useTempConfigHome(t)
	dir := filepath.Join(xdg, "cmdk")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "config.toml"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func stubActionRunCurrentClient(t *testing.T, resolve func(context.Context, time.Duration) (tmux.ClientTarget, error)) {
	t.Helper()
	old := currentActionRunClient
	currentActionRunClient = resolve
	t.Cleanup(func() { currentActionRunClient = old })
}

func stubActionRunServerCheck(t *testing.T, check func(context.Context, time.Duration) error) {
	t.Helper()
	old := requireActionRunServer
	requireActionRunServer = check
	t.Cleanup(func() { requireActionRunServer = old })
}

func stubActionRunTmux(t *testing.T, paneID string) {
	t.Helper()
	stubTmuxPrerequisite(t, func(context.Context) error { return nil })
	stubActionRunCurrentClient(t, func(context.Context, time.Duration) (tmux.ClientTarget, error) {
		return tmux.ClientTarget{Name: "/dev/pts/4", PaneID: paneID}, nil
	})
}

func stubPreparedActionLaunch(t *testing.T, executeLaunch func(execute.Launch) (execute.LaunchResult, error)) {
	t.Helper()
	oldResolve := resolveConfiguredActionLaunch
	oldExecute := executeConfiguredActionLaunch
	t.Cleanup(func() {
		resolveConfiguredActionLaunch = oldResolve
		executeConfiguredActionLaunch = oldExecute
	})
	resolveConfiguredActionLaunch = func([]item.Item, item.Item, string, config.Config) (execute.Launch, map[string]string, error) {
		return execute.Launch{}, nil, nil
	}
	executeConfiguredActionLaunch = executeLaunch
}

func TestActionRunHelpDocumentsContract(t *testing.T) {
	cmd := newActionRunCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	if err := cmd.Help(); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"run <exact-name>",
		"--path",
		"--input key=value",
		"--no-switch",
		"running tmux server",
		"does not start a server",
		"CMDK_PANE_ID",
		"before launch_path_cmd runs",
		"case-sensitive",
		"session-window",
		"Picker inputs",
		`"launchPath"`,
		`"paneId"`,
		"switching the client fails",
		"already running",
		"do not",
		"stderr",
	} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("help missing %q\n%s", want, out.String())
		}
	}
}

func TestActionRunRejectsShellModeBeforeTmuxPrerequisite(t *testing.T) {
	writeActionRunConfig(t, `
[[actions]]
name = "shell action"
matches = "root"
cmd = "true"
`)
	called := false
	stubTmuxPrerequisite(t, func(context.Context) error {
		called = true
		return nil
	})

	cmd := newRootCommand()
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	cmd.SetArgs([]string{"action", "run", "shell action", "--input", "bad"})
	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "effective launch_mode") {
		t.Fatalf("error = %v, want unsupported effective launch mode", err)
	}
	if called {
		t.Fatal("tmux prerequisite ran before launch-mode rejection")
	}
}

func TestActionRunRejectsSessionMatchBeforeTmuxPrerequisite(t *testing.T) {
	writeActionRunConfig(t, `
[[actions]]
name = "session action"
matches = "session"
launch_mode = "session-window"
launch_path = "/tmp"
cmd = "true"
`)
	called := false
	stubTmuxPrerequisite(t, func(context.Context) error {
		called = true
		return nil
	})

	cmd := newRootCommand()
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	cmd.SetArgs([]string{"action", "run", "session action"})
	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "session actions are not supported") {
		t.Fatalf("error = %v, want unsupported session action", err)
	}
	if called {
		t.Fatal("tmux prerequisite ran before session-scope rejection")
	}
}

func TestActionRunRejectsInvalidInputsBeforeLaunchPathCommand(t *testing.T) {
	path := t.TempDir()
	marker := filepath.Join(t.TempDir(), "launch-path-command-ran")
	writeActionRunConfig(t, `
[[actions]]
name = "staged dir"
matches = "dir"
launch_path_cmd = "touch '`+marker+`'; printf '%s\\n' {{sq .path}}"
cmd = "true"
stages = [
  { type = "prompt", text = "Value:", key = "value" },
]
`)
	stubActionRunTmux(t, "%17")

	cmd := newRootCommand()
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	cmd.SetArgs([]string{"action", "run", "staged dir", "--path", path, "--input", "unknown=value"})
	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "unknown input key") {
		t.Fatalf("error = %v, want unknown input rejection", err)
	}
	if _, statErr := os.Stat(marker); !os.IsNotExist(statErr) {
		t.Fatalf("launch_path_cmd side effect exists; stat error = %v", statErr)
	}
}

func TestActionRunRequiresCurrentClientBeforeLaunchPathCommand(t *testing.T) {
	marker := filepath.Join(t.TempDir(), "launch-path-command-ran")
	writeActionRunConfig(t, `
[[actions]]
name = "needs client"
matches = "root"
launch_path_cmd = "touch '`+marker+`'; pwd"
cmd = "true"
`)
	stubTmuxPrerequisite(t, func(context.Context) error { return nil })
	stubActionRunCurrentClient(t, func(context.Context, time.Duration) (tmux.ClientTarget, error) {
		return tmux.ClientTarget{}, errors.New("no current tmux client")
	})

	cmd := newRootCommand()
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	cmd.SetArgs([]string{"action", "run", "needs client"})
	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "no current tmux client") {
		t.Fatalf("error = %v, want current-client rejection", err)
	}
	if _, statErr := os.Stat(marker); !os.IsNotExist(statErr) {
		t.Fatalf("launch_path_cmd side effect exists; stat error = %v", statErr)
	}
}

func TestActionRunNoSwitchSkipsCurrentClientAndOmitsPaneContext(t *testing.T) {
	dir := t.TempDir()
	writeActionRunConfig(t, `
[[actions]]
name = "headless action"
matches = "root"
launch_path = "`+dir+`"
cmd = "true"
`)
	stubTmuxPrerequisite(t, func(context.Context) error { return nil })
	stubActionRunServerCheck(t, func(context.Context, time.Duration) error { return nil })
	clientCalled := false
	stubActionRunCurrentClient(t, func(context.Context, time.Duration) (tmux.ClientTarget, error) {
		clientCalled = true
		return tmux.ClientTarget{}, errors.New("current client should not be resolved")
	})

	oldResolve := resolveConfiguredActionLaunch
	oldExecute := executeConfiguredActionLaunch
	t.Cleanup(func() {
		resolveConfiguredActionLaunch = oldResolve
		executeConfiguredActionLaunch = oldExecute
	})
	resolved := false
	resolveConfiguredActionLaunch = func(_ []item.Item, _ item.Item, paneID string, _ config.Config) (execute.Launch, map[string]string, error) {
		resolved = true
		if paneID != "" {
			t.Errorf("paneID = %q, want empty", paneID)
		}
		return execute.Launch{}, nil, nil
	}
	executeConfiguredActionLaunch = func(execute.Launch) (execute.LaunchResult, error) {
		return execute.LaunchResult{
			LaunchPath: dir,
			SessionID:  "$5",
			SessionKey: dir,
			WindowID:   "@18",
			WindowName: "headless",
			PaneID:     "%51",
		}, nil
	}

	cmd := newRootCommand()
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	cmd.SetArgs([]string{"action", "run", "headless action", "--no-switch"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if clientCalled {
		t.Fatal("current client was resolved in no-switch mode")
	}
	if !resolved {
		t.Fatal("launch was not resolved")
	}
}

func TestActionRunNoSwitchFailsWithoutServerBeforeResolvingLaunch(t *testing.T) {
	dir := t.TempDir()
	writeActionRunConfig(t, `
[[actions]]
name = "headless action"
matches = "root"
launch_path = "`+dir+`"
cmd = "true"
`)
	stubTmuxPrerequisite(t, func(context.Context) error { return nil })
	stubActionRunServerCheck(t, func(context.Context, time.Duration) error {
		return errors.New("no running tmux server; cmdk does not start one")
	})
	stubActionRunCurrentClient(t, func(context.Context, time.Duration) (tmux.ClientTarget, error) {
		t.Error("current client should not be resolved in no-switch mode")
		return tmux.ClientTarget{}, errors.New("unexpected")
	})

	oldResolve := resolveConfiguredActionLaunch
	t.Cleanup(func() { resolveConfiguredActionLaunch = oldResolve })
	resolveConfiguredActionLaunch = func([]item.Item, item.Item, string, config.Config) (execute.Launch, map[string]string, error) {
		t.Error("launch was resolved despite the missing server")
		return execute.Launch{}, nil, nil
	}

	cmd := newRootCommand()
	cmd.SetOut(io.Discard)
	var errOut bytes.Buffer
	cmd.SetErr(&errOut)
	cmd.SetArgs([]string{"action", "run", "headless action", "--no-switch"})
	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "no running tmux server") {
		t.Fatalf("error = %v, want missing tmux server", err)
	}
}

func TestActionRunRejectsExplicitEmptyPathForRoot(t *testing.T) {
	dir := t.TempDir()
	writeActionRunConfig(t, `
[[actions]]
name = "root action"
matches = "root"
launch_path = "`+dir+`"
cmd = "true"
`)
	stubActionRunTmux(t, "%17")

	cmd := newRootCommand()
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	cmd.SetArgs([]string{"action", "run", "root action", "--path="})
	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "--path is not valid") {
		t.Fatalf("error = %v, want root --path rejection", err)
	}
}

func TestActionRunWritesLaunchJSON(t *testing.T) {
	path := t.TempDir()
	writeActionRunConfig(t, `
[[actions]]
name = "Deploy exact"
matches = "dir"
cmd = "printf '%s' {{sq .message}}"
window_name = "deploy-{{.message}}"
stages = [
  { type = "prompt", text = "Message:", key = "message" },
  { type = "prompt", text = "Origin:", key = "origin", default = "{{.pane_id}}" },
]
`)
	stubActionRunTmux(t, "%17")

	oldResolve := resolveConfiguredActionLaunch
	oldExecute := executeConfiguredActionLaunch
	t.Cleanup(func() {
		resolveConfiguredActionLaunch = oldResolve
		executeConfiguredActionLaunch = oldExecute
	})

	resolveConfiguredActionLaunch = func(accumulated []item.Item, selected item.Item, paneID string, cfg config.Config) (execute.Launch, map[string]string, error) {
		data := execute.FlattenData(accumulated)
		if data["path"] != filepath.Clean(path) {
			t.Errorf("path data = %q, want %q", data["path"], filepath.Clean(path))
		}
		if data["message"] != "release,a=b" {
			t.Errorf("message data = %q, want literal value", data["message"])
		}
		if data["origin"] != "%17" {
			t.Errorf("origin data = %q, want invoking pane", data["origin"])
		}
		if selected.Display != "Deploy exact" {
			t.Errorf("selected display = %q", selected.Display)
		}
		if paneID != "%17" {
			t.Errorf("paneID = %q, want %%17", paneID)
		}
		return execute.Launch{}, data, nil
	}
	executeConfiguredActionLaunch = func(execute.Launch) (execute.LaunchResult, error) {
		return execute.LaunchResult{
			LaunchPath: path,
			SessionID:  "$5",
			SessionKey: "/repo/key",
			WindowID:   "@18",
			WindowName: "deploy-release",
			PaneID:     "%51",
		}, nil
	}

	cmd := newRootCommand()
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(io.Discard)
	cmd.SetArgs([]string{
		"action", "run", "Deploy exact",
		"--path", path,
		"--input", "message=release,a=b",
	})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}

	var result actionRunResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("invalid JSON %q: %v", stdout.String(), err)
	}
	want := actionRunResult{
		Action:     "Deploy exact",
		LaunchPath: path,
		SessionID:  "$5",
		SessionKey: "/repo/key",
		WindowID:   "@18",
		WindowName: "deploy-release",
		PaneID:     "%51",
	}
	if result != want {
		t.Errorf("result = %#v, want %#v", result, want)
	}
	var fields map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &fields); err != nil {
		t.Fatal(err)
	}
	if len(fields) != 7 {
		t.Errorf("JSON fields = %v, want exactly 7", fields)
	}
}

func TestActionFlagErrorsEscapeTerminalControls(t *testing.T) {
	tests := []struct {
		name string
		flag string
	}{
		{name: "long", flag: "--unknown\x1b]52;c;payload\a"},
		{name: "shorthand", flag: "-\x1b"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cmd := newRootCommand()
			cmd.SetOut(io.Discard)
			cmd.SetErr(io.Discard)
			cmd.SetArgs([]string{"action", "run", "unused", test.flag})

			err := cmd.Execute()
			if err == nil {
				t.Fatal("expected flag parsing error")
			}
			if strings.ContainsRune(err.Error(), '\x1b') || strings.ContainsRune(err.Error(), '\a') {
				t.Fatalf("unsafe controls remain in %q", err.Error())
			}
			if !strings.Contains(err.Error(), `\x1b`) {
				t.Errorf("safe error %q missing escaped control", err.Error())
			}
		})
	}
}

func TestTerminalSafeActionRunErrorEscapesControlsAndPreservesCause(t *testing.T) {
	cause := errors.New("bad\x1b[31m\toutput\xff\nnext")
	err := terminalSafeActionRunError(cause)
	if !errors.Is(err, cause) {
		t.Fatal("safe error does not preserve its cause")
	}
	if strings.ContainsRune(err.Error(), '\x1b') || strings.ContainsRune(err.Error(), '\t') || strings.Contains(err.Error(), string([]byte{0xff})) {
		t.Fatalf("unsafe controls remain in %q", err.Error())
	}
	for _, want := range []string{`\x1b`, `\t`, `\xff`, "\nnext"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("safe error %q missing %q", err.Error(), want)
		}
	}
}

func TestRunPreparedActionReportsCreatedStateOnSwitchFailure(t *testing.T) {
	switchCause := errors.New("tmux switch-client failed: exit status 1\nstderr: client disappeared")
	stubPreparedActionLaunch(t, func(execute.Launch) (execute.LaunchResult, error) {
		return execute.LaunchResult{
			LaunchPath: "/Users/me/Code/github.com/acme/api-wt/fix-login",
			SessionID:  "$5",
			SessionKey: "/Users/me/Code/github.com/acme/api",
			WindowID:   "@18",
			WindowName: "wt-fix-login",
			PaneID:     "%51",
		}, &tmux.SwitchClientError{Err: switchCause}
	})

	var stdout bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&stdout)
	err := runPreparedAction(cmd, actionRunInvocation{actionName: "pi worktree"})
	if err == nil {
		t.Fatal("expected switch failure")
	}
	var switchErr *tmux.SwitchClientError
	if !errors.As(err, &switchErr) || !errors.Is(err, switchCause) {
		t.Fatalf("error = %T %[1]v, want wrapped switch failure", err)
	}
	for _, want := range []string{
		`action "pi worktree" launched`,
		"Do not rerun this action",
		"Created tmux state:",
		"$5",
		"/Users/me/Code/github.com/acme/api",
		"@18",
		"wt-fix-login",
		"%51",
		"/Users/me/Code/github.com/acme/api-wt/fix-login",
		"Switch failure: tmux switch-client failed: exit status 1\n  stderr: client disappeared",
		"tmux switch-client -t '$5:@18'",
		"--no-switch",
	} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error missing %q:\n%s", want, err)
		}
	}
	if stdout.Len() != 0 {
		t.Errorf("stdout = %q, want empty", stdout.String())
	}
}

func TestRunPreparedActionFallsBackToPlainSwitchErrorForIncompleteCreatedState(t *testing.T) {
	switchCause := errors.New("switch failed")
	stubPreparedActionLaunch(t, func(execute.Launch) (execute.LaunchResult, error) {
		return execute.LaunchResult{WindowID: "@1"}, &tmux.SwitchClientError{Err: switchCause}
	})

	var stdout bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&stdout)
	err := runPreparedAction(cmd, actionRunInvocation{actionName: "test"})
	if !errors.Is(err, switchCause) || err.Error() != switchCause.Error() {
		t.Fatalf("error = %v, want plain switch failure", err)
	}
	if strings.Contains(err.Error(), "Created tmux state") {
		t.Fatalf("incomplete result produced created-state diagnostic: %v", err)
	}
	if stdout.Len() != 0 {
		t.Errorf("stdout = %q, want empty", stdout.String())
	}
}

func TestRunPreparedActionLeavesNonSwitchErrorsUnchanged(t *testing.T) {
	cause := errors.New("new-window failed")
	stubPreparedActionLaunch(t, func(execute.Launch) (execute.LaunchResult, error) {
		return execute.LaunchResult{
			LaunchPath: "/tmp/launch",
			SessionID:  "$5",
			SessionKey: "/tmp/repo",
			WindowID:   "@18",
			WindowName: "main",
			PaneID:     "%51",
		}, cause
	})

	var stdout bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&stdout)
	err := runPreparedAction(cmd, actionRunInvocation{actionName: "test"})
	if err != cause {
		t.Fatalf("error = %v, want original creation error", err)
	}
	if strings.Contains(err.Error(), "Created tmux state") {
		t.Fatalf("non-switch error produced created-state diagnostic: %v", err)
	}
	if stdout.Len() != 0 {
		t.Errorf("stdout = %q, want empty", stdout.String())
	}
}

func TestRunPreparedActionSwitchFailureDiagnosticEscapesTerminalControls(t *testing.T) {
	stubPreparedActionLaunch(t, func(execute.Launch) (execute.LaunchResult, error) {
		return execute.LaunchResult{
			LaunchPath: "/tmp/launch\x1b]52;c;payload\a",
			SessionID:  "$5\x1b",
			SessionKey: "/tmp/repo\tkey",
			WindowID:   "@18",
			WindowName: "main\bname",
			PaneID:     "%51",
		}, &tmux.SwitchClientError{Err: errors.New("switch\a failed")}
	})

	cmd := &cobra.Command{}
	cmd.SetOut(io.Discard)
	err := terminalSafeActionRunError(runPreparedAction(cmd, actionRunInvocation{actionName: "pi\x1b action"}))
	if err == nil {
		t.Fatal("expected switch failure")
	}
	for _, control := range []rune{'\x1b', '\a', '\t', '\b'} {
		if strings.ContainsRune(err.Error(), control) {
			t.Fatalf("unsafe control %U remains in %q", control, err.Error())
		}
	}
	for _, want := range []string{`pi\x1b action`, `$5\x1b`, `repo\tkey`, `main\bname`, `switch\a failed`} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("safe error missing %q:\n%s", want, err)
		}
	}
}
