package cmd

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	resolver "github.com/jmcampanini/cmdk/internal/session"
	"github.com/jmcampanini/cmdk/internal/tmux"
)

func TestSessionWindowCommandRequiresPath(t *testing.T) {
	cmd := newSessionWindowCommand()
	if err := cmd.Args(cmd, nil); err == nil {
		t.Fatal("expected error for missing path")
	}
}

func TestSessionWindowCommandUseDocumentsRequiredPath(t *testing.T) {
	cmd := newSessionWindowCommand()
	if !strings.Contains(cmd.Use, "<path>") {
		t.Errorf("Use = %q, want required <path>", cmd.Use)
	}
	if strings.Contains(cmd.Use, "[path]") {
		t.Errorf("Use = %q, should not document optional [path]", cmd.Use)
	}
}

func TestRunSessionWindowCommandErrorsWithNoMode(t *testing.T) {
	useTempConfigHome(t)
	dir := filepath.Join(t.TempDir(), "scratch")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}

	cmd := &cobra.Command{}
	err := runSessionWindowCommand(cmd, []string{dir}, sessionWindowOptions{})
	if err == nil {
		t.Fatal("expected mode error")
	}
	if !strings.Contains(err.Error(), "--new or command args") {
		t.Errorf("error = %q, want mode guidance", err.Error())
	}
}

func TestRunSessionWindowCommandNewShellDefaultsToBackground(t *testing.T) {
	useTempConfigHome(t)
	dir := filepath.Join(t.TempDir(), "scratch")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}

	oldCreate := createResolvedSessionWindow
	t.Cleanup(func() { createResolvedSessionWindow = oldCreate })

	called := false
	createResolvedSessionWindow = func(ctx context.Context, plan resolver.Plan, launchPath string, opts tmux.SessionWindowOptions) (tmux.SessionWindowResult, error) {
		called = true
		if _, ok := ctx.Deadline(); ok {
			return tmux.SessionWindowResult{}, errors.New("window context unexpectedly inherited resolve timeout")
		}
		if err := ctx.Err(); err != nil {
			return tmux.SessionWindowResult{}, err
		}
		if plan.SessionKind != resolver.KindDirectory {
			t.Errorf("SessionKind = %q, want %q", plan.SessionKind, resolver.KindDirectory)
		}
		if !opts.NewShell {
			t.Error("NewShell = false, want true")
		}
		if len(opts.Command) != 0 {
			t.Errorf("Command = %q, want empty", opts.Command)
		}
		if opts.Switch {
			t.Error("Switch = true, want false")
		}
		return tmux.SessionWindowResult{}, nil
	}

	cmd := &cobra.Command{}
	if err := runSessionWindowCommand(cmd, []string{dir}, sessionWindowOptions{newShell: true}); err != nil {
		t.Fatal(err)
	}
	if !called {
		t.Fatal("createResolvedSessionWindow was not called")
	}
}

func TestRunSessionWindowCommandCommandModePassesArgvUnchanged(t *testing.T) {
	useTempConfigHome(t)
	dir := filepath.Join(t.TempDir(), "scratch")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}

	oldCreate := createResolvedSessionWindow
	t.Cleanup(func() { createResolvedSessionWindow = oldCreate })

	wantCommand := []string{"echo", "hello $HOME", "|", "tee", "x"}
	createResolvedSessionWindow = func(_ context.Context, _ resolver.Plan, launchPath string, opts tmux.SessionWindowOptions) (tmux.SessionWindowResult, error) {
		if opts.NewShell {
			t.Error("NewShell = true, want false")
		}
		if !slices.Equal(opts.Command, wantCommand) {
			t.Errorf("Command = %q, want %q", opts.Command, wantCommand)
		}
		return tmux.SessionWindowResult{}, nil
	}

	cmd := &cobra.Command{}
	args := append([]string{dir}, wantCommand...)
	if err := runSessionWindowCommand(cmd, args, sessionWindowOptions{dashSeen: true, argsLenAtDash: 1}); err != nil {
		t.Fatal(err)
	}
}

func TestRunSessionWindowCommandRejectsCommandWithoutDashDash(t *testing.T) {
	cmd := &cobra.Command{}
	err := runSessionWindowCommand(cmd, []string{".", "echo", "hi"}, sessionWindowOptions{})
	if err == nil {
		t.Fatal("expected delimiter error")
	}
	if !strings.Contains(err.Error(), "--") {
		t.Errorf("error = %q, want -- guidance", err.Error())
	}
}

func TestSplitSessionWindowArgsAllowsFlagTerminatorBeforeDashPath(t *testing.T) {
	path, commandArgs, commandDelimiter, err := splitSessionWindowArgs(
		[]string{"-project"},
		sessionWindowOptions{dashSeen: true, argsLenAtDash: 0},
	)
	if err != nil {
		t.Fatal(err)
	}
	if path != "-project" {
		t.Errorf("path = %q, want -project", path)
	}
	if len(commandArgs) != 0 {
		t.Errorf("commandArgs = %q, want empty", commandArgs)
	}
	if commandDelimiter {
		t.Error("commandDelimiter = true, want false")
	}
}

func TestSplitSessionWindowArgsAllowsDashPathAndCommandDelimiter(t *testing.T) {
	path, commandArgs, commandDelimiter, err := splitSessionWindowArgs(
		[]string{"-project", "--", "echo", "hi"},
		sessionWindowOptions{dashSeen: true, argsLenAtDash: 0},
	)
	if err != nil {
		t.Fatal(err)
	}
	if path != "-project" {
		t.Errorf("path = %q, want -project", path)
	}
	want := []string{"echo", "hi"}
	if !slices.Equal(commandArgs, want) {
		t.Errorf("commandArgs = %q, want %q", commandArgs, want)
	}
	if !commandDelimiter {
		t.Error("commandDelimiter = false, want true")
	}
}

func TestSplitSessionWindowArgsRejectsExtraArgsBeforeDashDash(t *testing.T) {
	_, _, _, err := splitSessionWindowArgs(
		[]string{".", "extra", "echo"},
		sessionWindowOptions{dashSeen: true, argsLenAtDash: 2},
	)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "exactly one path") {
		t.Errorf("error = %q, want path count context", err.Error())
	}
}

func TestRunSessionWindowCommandNameOverride(t *testing.T) {
	useTempConfigHome(t)
	dir := filepath.Join(t.TempDir(), "scratch")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}

	oldCreate := createResolvedSessionWindow
	t.Cleanup(func() { createResolvedSessionWindow = oldCreate })

	createResolvedSessionWindow = func(_ context.Context, _ resolver.Plan, launchPath string, opts tmux.SessionWindowOptions) (tmux.SessionWindowResult, error) {
		if opts.Name != "tests" {
			t.Errorf("Name = %q, want tests", opts.Name)
		}
		return tmux.SessionWindowResult{}, nil
	}

	cmd := &cobra.Command{}
	if err := runSessionWindowCommand(cmd, []string{dir}, sessionWindowOptions{newShell: true, name: "tests", nameSet: true}); err != nil {
		t.Fatal(err)
	}
}

func TestRunSessionWindowCommandNewPlusCommandErrors(t *testing.T) {
	cmd := &cobra.Command{}
	err := runSessionWindowCommand(cmd, []string{".", "echo", "hi"}, sessionWindowOptions{newShell: true, dashSeen: true, argsLenAtDash: 1})
	if err == nil {
		t.Fatal("expected mode conflict error")
	}
	if !strings.Contains(err.Error(), "--new") || !strings.Contains(err.Error(), "command") {
		t.Errorf("error = %q, want --new command conflict", err.Error())
	}
}

func TestRunSessionWindowCommandEmptyNameErrors(t *testing.T) {
	cmd := &cobra.Command{}
	err := runSessionWindowCommand(cmd, []string{"."}, sessionWindowOptions{newShell: true, nameSet: true})
	if err == nil {
		t.Fatal("expected name error")
	}
	if !strings.Contains(err.Error(), "--name") || !strings.Contains(err.Error(), "empty") {
		t.Errorf("error = %q, want empty --name context", err.Error())
	}
}

func TestSessionWindowCommandParsesFlagsOnlyBeforeDashDash(t *testing.T) {
	useTempConfigHome(t)
	dir := filepath.Join(t.TempDir(), "scratch")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}

	oldCreate := createResolvedSessionWindow
	t.Cleanup(func() { createResolvedSessionWindow = oldCreate })
	stubTmuxPrerequisite(t, func(context.Context) error { return nil })

	var got tmux.SessionWindowOptions
	createResolvedSessionWindow = func(_ context.Context, _ resolver.Plan, _ string, opts tmux.SessionWindowOptions) (tmux.SessionWindowResult, error) {
		got = opts
		return tmux.SessionWindowResult{}, nil
	}

	tests := []struct {
		name        string
		args        []string
		wantSwitch  bool
		wantCommand []string
	}{
		{name: "payload flag", args: []string{dir, "--", "--flag", "value"}, wantCommand: []string{"--flag", "value"}},
		{name: "switch before delimiter", args: []string{dir, "--switch", "--", "echo", "hello"}, wantSwitch: true, wantCommand: []string{"echo", "hello"}},
		{name: "switch after delimiter", args: []string{dir, "--", "--switch"}, wantCommand: []string{"--switch"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got = tmux.SessionWindowOptions{}
			cmd := newSessionWindowCommand()
			cmd.SetArgs(test.args)
			if err := cmd.Execute(); err != nil {
				t.Fatal(err)
			}
			if got.Switch != test.wantSwitch {
				t.Errorf("Switch = %t, want %t", got.Switch, test.wantSwitch)
			}
			if !slices.Equal(got.Command, test.wantCommand) {
				t.Errorf("Command = %q, want %q", got.Command, test.wantCommand)
			}
		})
	}
}

func TestRunSessionWindowCommandThreadsConfiguredWindowNameMaxLength(t *testing.T) {
	xdg := useTempConfigHome(t)
	cfgDir := filepath.Join(xdg, "cmdk")
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cfgDir, "config.toml"), []byte("[behavior]\nwindow_name_max_length = 7\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(t.TempDir(), "a-very-long-directory-name")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}

	oldCreate := createResolvedSessionWindow
	t.Cleanup(func() { createResolvedSessionWindow = oldCreate })

	called := false
	createResolvedSessionWindow = func(_ context.Context, _ resolver.Plan, _ string, opts tmux.SessionWindowOptions) (tmux.SessionWindowResult, error) {
		called = true
		if opts.Name != "a-very-long-directory-name" {
			t.Errorf("Name = %q, want untruncated basename", opts.Name)
		}
		if opts.MaxNameLength != 7 {
			t.Errorf("MaxNameLength = %d, want configured 7", opts.MaxNameLength)
		}
		return tmux.SessionWindowResult{}, nil
	}

	cmd := &cobra.Command{}
	if err := runSessionWindowCommand(cmd, []string{dir}, sessionWindowOptions{newShell: true}); err != nil {
		t.Fatal(err)
	}
	if !called {
		t.Fatal("createResolvedSessionWindow was not called")
	}
}

func TestRunSessionWindowCommandDefaultsWindowNameMaxLength(t *testing.T) {
	useTempConfigHome(t)
	dir := filepath.Join(t.TempDir(), "scratch")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}

	oldCreate := createResolvedSessionWindow
	t.Cleanup(func() { createResolvedSessionWindow = oldCreate })

	called := false
	createResolvedSessionWindow = func(_ context.Context, _ resolver.Plan, _ string, opts tmux.SessionWindowOptions) (tmux.SessionWindowResult, error) {
		called = true
		if opts.MaxNameLength != 20 {
			t.Errorf("MaxNameLength = %d, want default 20", opts.MaxNameLength)
		}
		return tmux.SessionWindowResult{}, nil
	}

	cmd := &cobra.Command{}
	if err := runSessionWindowCommand(cmd, []string{dir}, sessionWindowOptions{newShell: true}); err != nil {
		t.Fatal(err)
	}
	if !called {
		t.Fatal("createResolvedSessionWindow was not called")
	}
}
