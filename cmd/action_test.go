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

	"github.com/jmcampanini/cmdk/internal/actionrun"
	"github.com/jmcampanini/cmdk/internal/config"
	"github.com/jmcampanini/cmdk/internal/execute"
	"github.com/jmcampanini/cmdk/internal/item"
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

func stubActionRunCurrentPane(t *testing.T, resolve func(context.Context, time.Duration) (string, error)) {
	t.Helper()
	old := currentActionRunPane
	currentActionRunPane = resolve
	t.Cleanup(func() { currentActionRunPane = old })
}

func stubActionRunTmux(t *testing.T, paneID string) {
	t.Helper()
	stubTmuxPrerequisite(t, func(context.Context) error { return nil })
	stubActionRunCurrentPane(t, func(context.Context, time.Duration) (string, error) {
		return paneID, nil
	})
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
		"case-sensitive",
		"session-window",
		"Picker inputs",
		`"launchPath"`,
		`"paneId"`,
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
	stubActionRunCurrentPane(t, func(context.Context, time.Duration) (string, error) {
		return "", errors.New("no current tmux client")
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

func TestRunPreparedActionDoesNotWriteJSONOnLaunchError(t *testing.T) {
	oldResolve := resolveConfiguredActionLaunch
	oldExecute := executeConfiguredActionLaunch
	t.Cleanup(func() {
		resolveConfiguredActionLaunch = oldResolve
		executeConfiguredActionLaunch = oldExecute
	})
	resolveConfiguredActionLaunch = func([]item.Item, item.Item, string, config.Config) (execute.Launch, map[string]string, error) {
		return execute.Launch{}, nil, nil
	}
	executeConfiguredActionLaunch = func(execute.Launch) (execute.LaunchResult, error) {
		return execute.LaunchResult{WindowID: "@1"}, errors.New("switch failed")
	}

	var stdout bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&stdout)
	err := runPreparedAction(cmd, actionRunInvocation{
		prepared: actionrun.Prepared{Action: config.Action{Name: "test"}},
	})
	if err == nil || !strings.Contains(err.Error(), "switch failed") {
		t.Fatalf("error = %v, want switch failure", err)
	}
	if stdout.Len() != 0 {
		t.Errorf("stdout = %q, want empty", stdout.String())
	}
}
