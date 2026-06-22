package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	resolver "github.com/jmcampanini/cmdk/internal/session"
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

func TestWriteSessionPlan(t *testing.T) {
	plan := resolver.Plan{
		SessionKind:            resolver.KindRepo,
		SessionKey:             "/Users/me/Code/github.com/me/dotfiles",
		DisplayLabel:           "~/Code/github.com/me/dotfiles",
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
		"display_label:",
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
	if plan.DisplayLabel != "~/project" {
		t.Errorf("DisplayLabel = %q, want ~/project", plan.DisplayLabel)
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
