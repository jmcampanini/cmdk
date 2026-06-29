package cmd

import (
	"bytes"
	"context"
	"encoding/json"
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

func useTempConfigHome(t *testing.T) string {
	t.Helper()
	xdg := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", xdg)
	oldConfigPath := configPath
	configPath = ""
	t.Cleanup(func() { configPath = oldConfigPath })
	return xdg
}

func TestSessionResolveCommandRequiresPath(t *testing.T) {
	cmd := newSessionResolveCommand()
	if err := cmd.Args(cmd, nil); err == nil {
		t.Fatal("expected error for missing path")
	}
}

func TestSessionResolveCommandUseDocumentsRequiredPath(t *testing.T) {
	cmd := newSessionResolveCommand()
	if !strings.Contains(cmd.Use, "<path>") {
		t.Errorf("Use = %q, want required <path>", cmd.Use)
	}
	if strings.Contains(cmd.Use, "[path]") {
		t.Errorf("Use = %q, should not document optional [path]", cmd.Use)
	}
}

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

func TestRunSessionWindowCommandNewShellCallsTmuxWindowFunction(t *testing.T) {
	useTempConfigHome(t)
	dir := filepath.Join(t.TempDir(), "scratch")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}

	oldCreate := createResolvedSessionWindow
	t.Cleanup(func() { createResolvedSessionWindow = oldCreate })

	called := false
	createResolvedSessionWindow = func(ctx context.Context, plan resolver.Plan, opts tmux.SessionWindowOptions) error {
		called = true
		if _, ok := ctx.Deadline(); ok {
			return errors.New("window context unexpectedly inherited resolve timeout")
		}
		if err := ctx.Err(); err != nil {
			return err
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
		if !opts.Switch {
			t.Error("Switch = false, want true")
		}
		return nil
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
	createResolvedSessionWindow = func(_ context.Context, _ resolver.Plan, opts tmux.SessionWindowOptions) error {
		if opts.NewShell {
			t.Error("NewShell = true, want false")
		}
		if !slices.Equal(opts.Command, wantCommand) {
			t.Errorf("Command = %q, want %q", opts.Command, wantCommand)
		}
		return nil
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

	createResolvedSessionWindow = func(_ context.Context, _ resolver.Plan, opts tmux.SessionWindowOptions) error {
		if opts.Name != "tests" {
			t.Errorf("Name = %q, want tests", opts.Name)
		}
		return nil
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

func TestSessionWindowCommandAllowsArgsAfterDashDash(t *testing.T) {
	useTempConfigHome(t)
	dir := filepath.Join(t.TempDir(), "scratch")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}

	oldCreate := createResolvedSessionWindow
	t.Cleanup(func() { createResolvedSessionWindow = oldCreate })

	createResolvedSessionWindow = func(_ context.Context, _ resolver.Plan, opts tmux.SessionWindowOptions) error {
		want := []string{"--flag", "value"}
		if !slices.Equal(opts.Command, want) {
			t.Errorf("Command = %q, want %q", opts.Command, want)
		}
		return nil
	}

	cmd := newSessionWindowCommand()
	cmd.SetArgs([]string{dir, "--", "--flag", "value"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
}

func TestWriteSessionPlan(t *testing.T) {
	plan := resolver.Plan{
		SessionKind:            resolver.KindRepo,
		SessionKey:             "/Users/me/Code/github.com/me/dotfiles",
		SessionDisplay:         "~/Code/github.com/me/dotfiles",
		LaunchPath:             "/Users/me/Code/github.com/me/dotfiles/main",
		PlannedTmuxSessionName: "Users/me/Code/github_com/me/dotfiles",
		PlannedTmuxWindowName:  "main",
	}
	var buf bytes.Buffer
	if err := writeSessionPlan(&buf, plan); err != nil {
		t.Fatal(err)
	}
	got := buf.String()
	for _, want := range []string{
		"session_kind:",
		"session_key:",
		"session_display:",
		"launch_path:",
		"planned_tmux_session_name:",
		"planned_tmux_window_name:",
		"Users/me/Code/github_com/me/dotfiles",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("human output missing %q\n%s", want, got)
		}
	}
	if strings.Contains(got, "session_id") {
		t.Errorf("human output should not use session_id for cmdk identity\n%s", got)
	}
	if strings.Contains(got, "display_label") {
		t.Errorf("human output should not contain display_label\n%s", got)
	}
}

func TestRunSessionResolveCommandJSON(t *testing.T) {
	useTempConfigHome(t)

	dir := filepath.Join(t.TempDir(), "scratch")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&buf)
	if err := runSessionResolveCommand(cmd, dir, sessionResolveOptions{json: true}); err != nil {
		t.Fatal(err)
	}

	var plan resolver.Plan
	if err := json.Unmarshal(buf.Bytes(), &plan); err != nil {
		t.Fatalf("invalid JSON %q: %v", buf.String(), err)
	}
	if plan.SessionKind != resolver.KindDirectory {
		t.Errorf("SessionKind = %q, want %q", plan.SessionKind, resolver.KindDirectory)
	}
	dirReal, err := filepath.EvalSymlinks(dir)
	if err != nil {
		t.Fatalf("EvalSymlinks(%q): %v", dir, err)
	}
	dirReal = filepath.Clean(dirReal)
	if plan.SessionKey != dirReal {
		t.Errorf("SessionKey = %q, want %q", plan.SessionKey, dirReal)
	}
	if strings.Contains(buf.String(), "session_id") {
		t.Errorf("JSON should not contain session_id: %s", buf.String())
	}
	if !strings.Contains(buf.String(), "session_display") {
		t.Errorf("JSON should contain session_display: %s", buf.String())
	}
	if strings.Contains(buf.String(), "display_label") {
		t.Errorf("JSON should not contain display_label: %s", buf.String())
	}
}

func TestRunSessionResolveCommandShortensSymlinkedHome(t *testing.T) {
	useTempConfigHome(t)

	root := t.TempDir()
	realHome := filepath.Join(root, "real-home")
	dir := filepath.Join(realHome, "project")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	linkHome := filepath.Join(root, "home-link")
	if err := os.Symlink(realHome, linkHome); err != nil {
		t.Skipf("symlink not available: %v", err)
	}
	t.Setenv("HOME", linkHome)

	var buf bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&buf)
	if err := runSessionResolveCommand(cmd, filepath.Join(linkHome, "project"), sessionResolveOptions{json: true}); err != nil {
		t.Fatal(err)
	}

	var plan resolver.Plan
	if err := json.Unmarshal(buf.Bytes(), &plan); err != nil {
		t.Fatalf("invalid JSON %q: %v", buf.String(), err)
	}
	if plan.SessionDisplay != "~/project" {
		t.Errorf("SessionDisplay = %q, want ~/project", plan.SessionDisplay)
	}
}

func TestRunSessionResolveCommandTimesOutGitProbe(t *testing.T) {
	xdg := useTempConfigHome(t)
	cfgDir := filepath.Join(xdg, "cmdk")
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cfgDir, "config.toml"), []byte("[timeout]\nfetch = \"10ms\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	bin := t.TempDir()
	gitPath := filepath.Join(bin, "git")
	if err := os.WriteFile(gitPath, []byte("#!/bin/sh\nwhile :; do :; done\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin)

	dir := filepath.Join(t.TempDir(), "scratch")
	if err := os.MkdirAll(filepath.Join(dir, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}

	cmd := &cobra.Command{}
	err := runSessionResolveCommand(cmd, dir, sessionResolveOptions{json: true})
	if err == nil {
		t.Fatal("expected timeout error")
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("error = %T %[1]v, want context deadline exceeded", err)
	}
}
