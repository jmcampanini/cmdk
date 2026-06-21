package cmd

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	resolver "github.com/jmcampanini/cmdk/internal/session"
)

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
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	oldConfigPath := configPath
	configPath = ""
	defer func() { configPath = oldConfigPath }()

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
