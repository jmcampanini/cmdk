package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	resolver "github.com/jmcampanini/cmdk/internal/session"
)

func useAttachTestHooks(t *testing.T, check func(context.Context, resolver.Plan, string, string, int) error) func() bool {
	t.Helper()
	oldAttach := attachResolvedSession
	oldInside := isInsideTmux
	called := false
	attachResolvedSession = func(ctx context.Context, plan resolver.Plan, launchPath, windowName string, maxNameLength int) error {
		called = true
		if check != nil {
			return check(ctx, plan, launchPath, windowName, maxNameLength)
		}
		return nil
	}
	isInsideTmux = func() bool { return false }
	t.Cleanup(func() {
		attachResolvedSession = oldAttach
		isInsideTmux = oldInside
	})
	return func() bool { return called }
}

func writeStartupConfig(t *testing.T, xdg, path string) {
	t.Helper()
	cfgDir := filepath.Join(xdg, "cmdk")
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	content := fmt.Sprintf("[startup]\npath = %q\n", path)
	if err := os.WriteFile(filepath.Join(cfgDir, "config.toml"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func realAttachPath(t *testing.T, path string) string {
	t.Helper()
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		t.Fatalf("EvalSymlinks(%q): %v", path, err)
	}
	return filepath.Clean(resolved)
}

func TestAttachCommandUseDocumentsOptionalPath(t *testing.T) {
	cmd := newAttachCommand()
	if !strings.Contains(cmd.Use, "[path]") {
		t.Errorf("Use = %q, want optional [path]", cmd.Use)
	}
}

func TestAttachCommandRejectsExtraArgs(t *testing.T) {
	cmd := newAttachCommand()
	if err := cmd.Args(cmd, []string{"one", "two"}); err == nil {
		t.Fatal("expected error for extra args")
	}
}

func TestRunAttachCommandRejectsInsideTmux(t *testing.T) {
	oldInside := isInsideTmux
	isInsideTmux = func() bool { return true }
	t.Cleanup(func() { isInsideTmux = oldInside })

	err := runAttachCommand(&cobra.Command{}, []string{"."})
	if err == nil {
		t.Fatal("expected inside-tmux error")
	}
	if !strings.Contains(err.Error(), "inside tmux") || !strings.Contains(err.Error(), "outside tmux") {
		t.Errorf("error = %q, want inside/outside tmux guidance", err.Error())
	}
}

func TestRunAttachCommandRequiresConfiguredOrExplicitPath(t *testing.T) {
	useTempConfigHome(t)
	called := useAttachTestHooks(t, nil)

	err := runAttachCommand(&cobra.Command{}, nil)
	if err == nil {
		t.Fatal("expected missing startup path error")
	}
	if !strings.Contains(err.Error(), "[startup].path") || !strings.Contains(err.Error(), "cmdk attach <path>") {
		t.Errorf("error = %q, want setup guidance", err.Error())
	}
	if called() {
		t.Fatal("attach should not be called when startup path is missing")
	}
}

func TestRunAttachCommandUsesConfiguredStartupPath(t *testing.T) {
	xdg := useTempConfigHome(t)
	dir := filepath.Join(t.TempDir(), "project")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeStartupConfig(t, xdg, dir)

	called := useAttachTestHooks(t, func(ctx context.Context, plan resolver.Plan, launchPath, windowName string, maxNameLength int) error {
		if _, ok := ctx.Deadline(); ok {
			t.Fatal("attach context unexpectedly inherited resolve timeout")
		}
		want := realAttachPath(t, dir)
		if plan.SessionKey != want {
			t.Errorf("SessionKey = %q, want %q", plan.SessionKey, want)
		}
		if launchPath != filepath.Clean(dir) {
			t.Errorf("launchPath = %q, want %q", launchPath, filepath.Clean(dir))
		}
		if windowName != filepath.Base(dir) {
			t.Errorf("windowName = %q, want %q", windowName, filepath.Base(dir))
		}
		if maxNameLength != 20 {
			t.Errorf("maxNameLength = %d, want default 20", maxNameLength)
		}
		return nil
	})

	if err := runAttachCommand(&cobra.Command{}, nil); err != nil {
		t.Fatal(err)
	}
	if !called() {
		t.Fatal("attach was not called")
	}
}

func TestRunAttachCommandPathArgOverridesConfiguredStartupPath(t *testing.T) {
	xdg := useTempConfigHome(t)
	configured := filepath.Join(t.TempDir(), "configured")
	explicit := filepath.Join(t.TempDir(), "explicit")
	for _, dir := range []string{configured, explicit} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	writeStartupConfig(t, xdg, configured)

	called := useAttachTestHooks(t, func(_ context.Context, plan resolver.Plan, launchPath, windowName string, _ int) error {
		want := realAttachPath(t, explicit)
		if plan.SessionKey != want {
			t.Errorf("SessionKey = %q, want explicit path %q", plan.SessionKey, want)
		}
		if launchPath != filepath.Clean(explicit) {
			t.Errorf("launchPath = %q, want explicit path %q", launchPath, filepath.Clean(explicit))
		}
		if windowName != filepath.Base(explicit) {
			t.Errorf("windowName = %q, want %q", windowName, filepath.Base(explicit))
		}
		return nil
	})

	if err := runAttachCommand(&cobra.Command{}, []string{explicit}); err != nil {
		t.Fatal(err)
	}
	if !called() {
		t.Fatal("attach was not called")
	}
}

func TestRunAttachCommandExpandsHomeInConfiguredStartupPath(t *testing.T) {
	xdg := useTempConfigHome(t)
	home := t.TempDir()
	dir := filepath.Join(home, "project")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", home)
	writeStartupConfig(t, xdg, "~/project")

	called := useAttachTestHooks(t, func(_ context.Context, plan resolver.Plan, launchPath, windowName string, _ int) error {
		want := realAttachPath(t, dir)
		if plan.SessionKey != want {
			t.Errorf("SessionKey = %q, want expanded home path %q", plan.SessionKey, want)
		}
		if launchPath != filepath.Clean(dir) {
			t.Errorf("launchPath = %q, want expanded home path %q", launchPath, filepath.Clean(dir))
		}
		if windowName != filepath.Base(dir) {
			t.Errorf("windowName = %q, want %q", windowName, filepath.Base(dir))
		}
		return nil
	})

	if err := runAttachCommand(&cobra.Command{}, nil); err != nil {
		t.Fatal(err)
	}
	if !called() {
		t.Fatal("attach was not called")
	}
}

func TestRunAttachCommandThreadsConfiguredWindowNameMaxLength(t *testing.T) {
	xdg := useTempConfigHome(t)
	dir := filepath.Join(t.TempDir(), "project")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	cfgDir := filepath.Join(xdg, "cmdk")
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	content := fmt.Sprintf("[startup]\npath = %q\n\n[behavior]\nwindow_name_max_length = 7\n", dir)
	if err := os.WriteFile(filepath.Join(cfgDir, "config.toml"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	called := useAttachTestHooks(t, func(_ context.Context, _ resolver.Plan, _, _ string, maxNameLength int) error {
		if maxNameLength != 7 {
			t.Errorf("maxNameLength = %d, want configured 7", maxNameLength)
		}
		return nil
	})

	if err := runAttachCommand(&cobra.Command{}, nil); err != nil {
		t.Fatal(err)
	}
	if !called() {
		t.Fatal("attach was not called")
	}
}
