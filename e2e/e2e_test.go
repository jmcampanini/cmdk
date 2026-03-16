package e2e

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

var binaryPath string

func TestMain(m *testing.M) {
	if _, err := exec.LookPath("tmux"); err != nil {
		fmt.Fprintln(os.Stderr, "SKIP: e2e tests require tmux in PATH")
		os.Exit(0)
	}

	tmp, err := os.MkdirTemp("", "cmdk-e2e-*")
	if err != nil {
		panic(err)
	}
	binaryPath = tmp + "/cmdk"
	build := exec.Command("go", "build", "-o", binaryPath, "..")
	build.Stderr = os.Stderr
	if err := build.Run(); err != nil {
		panic("failed to build binary: " + err.Error())
	}

	code := m.Run()
	if err := os.RemoveAll(tmp); err != nil {
		fmt.Fprintf(os.Stderr, "warning: failed to clean up %s: %v\n", tmp, err)
	}
	os.Exit(code)
}

func startSession(t *testing.T) string {
	t.Helper()
	return startSessionWithEnv(t, nil)
}

func startSessionWithEnv(t *testing.T, env map[string]string) string {
	t.Helper()
	sess := "cmdk-test-" + strings.ReplaceAll(t.Name(), "/", "-")
	args := []string{"new-session", "-d", "-s", sess, "-x", "120", "-y", "40"}
	if len(env) > 0 {
		args = append(args, "env")
		for k, v := range env {
			args = append(args, k+"="+v)
		}
	}
	args = append(args, binaryPath, "--pane-id=%0")
	cmd := exec.Command("tmux", args...)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("tmux new-session failed: %v\n%s", err, out)
	}
	return sess
}

func killSession(t *testing.T, sess string) {
	t.Helper()
	if err := exec.Command("tmux", "kill-session", "-t", sess).Run(); err != nil {
		t.Logf("warning: failed to kill tmux session %s: %v", sess, err)
	}
}

func capturePane(t *testing.T, sess string) string {
	t.Helper()
	out, err := exec.Command("tmux", "capture-pane", "-t", sess, "-p").Output()
	if err != nil {
		t.Fatalf("capture-pane failed: %v", err)
	}
	return string(out)
}

func sendKeys(t *testing.T, sess string, keys ...string) {
	t.Helper()
	args := append([]string{"send-keys", "-t", sess}, keys...)
	if out, err := exec.Command("tmux", args...).CombinedOutput(); err != nil {
		t.Fatalf("send-keys failed: %v\n%s", err, out)
	}
}

func waitForContent(t *testing.T, sess string, check func(string) bool, timeout time.Duration) string {
	t.Helper()
	deadline := time.Now().Add(timeout)
	var content string
	for time.Now().Before(deadline) {
		content = capturePane(t, sess)
		if check(content) {
			return content
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for content.\nLast capture:\n%s", content)
	return ""
}

func sessionExists(sess string) bool {
	return exec.Command("tmux", "has-session", "-t", sess).Run() == nil
}

func waitForReady(t *testing.T, sess string) string {
	t.Helper()
	return waitForContent(t, sess, func(s string) bool {
		return strings.Contains(s, sess)
	}, 5*time.Second)
}

func waitForExit(t *testing.T, sess string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if !sessionExists(sess) {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatal("session still exists — process did not exit")
}

func typeText(t *testing.T, sess string, text string) {
	t.Helper()
	for _, ch := range text {
		sendKeys(t, sess, string(ch))
	}
}

func hasZoxide() bool {
	_, err := exec.LookPath("zoxide")
	return err == nil
}

func requireZoxideEntries(t *testing.T) {
	t.Helper()
	if !hasZoxide() {
		t.Skip("zoxide not available")
	}
	out, _ := exec.Command("zoxide", "query", "--list", "--score").Output()
	if len(strings.TrimSpace(string(out))) == 0 {
		t.Skip("no zoxide entries")
	}
}

func navigateToDirItem(t *testing.T, sess string) {
	t.Helper()
	content := capturePane(t, sess)
	windowCount := strings.Count(content, "window")
	for range windowCount {
		sendKeys(t, sess, "Down")
		time.Sleep(50 * time.Millisecond)
	}
	waitForContent(t, sess, func(s string) bool {
		return strings.Contains(s, "dir")
	}, 3*time.Second)
}

func TestE2E_ItemsVisible(t *testing.T) {
	sess := startSession(t)
	defer killSession(t, sess)

	content := waitForReady(t, sess)

	if !strings.Contains(content, sess+":") {
		t.Errorf("expected test session window %q in output\nCapture:\n%s", sess, content)
	}

	if !strings.Contains(content, "window") {
		t.Errorf("expected 'window' type in output\nCapture:\n%s", content)
	}
}

func TestE2E_FilterItems(t *testing.T) {
	sess := startSession(t)
	defer killSession(t, sess)

	waitForReady(t, sess)

	sendKeys(t, sess, "/")
	time.Sleep(200 * time.Millisecond)
	typeText(t, sess, "cmdk-test")

	waitForContent(t, sess, func(s string) bool {
		return strings.Contains(s, sess) && strings.Contains(s, "filtered")
	}, 5*time.Second)
}

func TestE2E_EscapeDuringFilterDoesNotQuit(t *testing.T) {
	sess := startSession(t)
	defer killSession(t, sess)

	waitForReady(t, sess)

	sendKeys(t, sess, "/")
	time.Sleep(200 * time.Millisecond)
	typeText(t, sess, "cmdk-test")

	waitForContent(t, sess, func(s string) bool {
		return strings.Contains(s, "filtered")
	}, 5*time.Second)

	sendKeys(t, sess, "Escape")

	waitForContent(t, sess, func(s string) bool {
		return strings.Contains(s, sess) && !strings.Contains(s, "filtered")
	}, 5*time.Second)

	if !sessionExists(sess) {
		t.Fatal("session should still exist after Escape during filter")
	}
}

func TestE2E_EscapeQuits(t *testing.T) {
	sess := startSession(t)
	defer killSession(t, sess)

	waitForReady(t, sess)

	sendKeys(t, sess, "Escape")
	waitForExit(t, sess)
}

func TestE2E_EnterExecutesAndExits(t *testing.T) {
	sess := startSession(t)
	defer killSession(t, sess)

	waitForReady(t, sess)

	sendKeys(t, sess, "Enter")
	waitForExit(t, sess)
}

func TestE2E_ZoxideDirsVisible(t *testing.T) {
	requireZoxideEntries(t)
	sess := startSession(t)
	defer killSession(t, sess)

	content := waitForReady(t, sess)

	if !strings.Contains(content, "dir") {
		t.Errorf("expected 'dir' type in output when zoxide available\nCapture:\n%s", content)
	}
}

func TestE2E_SelectDirShowsDirActions(t *testing.T) {
	requireZoxideEntries(t)

	sess := startSession(t)
	defer killSession(t, sess)

	waitForReady(t, sess)
	navigateToDirItem(t, sess)

	sendKeys(t, sess, "Enter")

	waitForContent(t, sess, func(s string) bool {
		return strings.Contains(s, "New window")
	}, 5*time.Second)
}

func TestE2E_EscapeFromDirActionsReturnsToRoot(t *testing.T) {
	requireZoxideEntries(t)

	sess := startSession(t)
	defer killSession(t, sess)

	waitForReady(t, sess)
	navigateToDirItem(t, sess)

	sendKeys(t, sess, "Enter")

	waitForContent(t, sess, func(s string) bool {
		return strings.Contains(s, "New window")
	}, 5*time.Second)

	sendKeys(t, sess, "Escape")

	waitForContent(t, sess, func(s string) bool {
		return strings.Contains(s, "window")
	}, 5*time.Second)

	if !sessionExists(sess) {
		t.Fatal("session should still exist after Escape from dir-actions")
	}
}

func TestE2E_WithoutZoxide_StillLaunches(t *testing.T) {
	sess := startSession(t)
	defer killSession(t, sess)

	content := waitForReady(t, sess)

	if !strings.Contains(content, "window") {
		t.Errorf("expected window items even without zoxide\nCapture:\n%s", content)
	}
}

func writeConfig(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	configDir := filepath.Join(dir, "cmdk")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(configDir, "config.toml"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestE2E_ConfigCommandsVisible(t *testing.T) {
	xdg := writeConfig(t, `
[[commands]]
name = "my-custom-cmd"
cmd = "echo hello"
`)
	sess := startSessionWithEnv(t, map[string]string{"XDG_CONFIG_HOME": xdg})
	defer killSession(t, sess)

	waitForReady(t, sess)
	sendKeys(t, sess, "End")

	waitForContent(t, sess, func(s string) bool {
		return strings.Contains(s, "my-custom-cmd")
	}, 5*time.Second)
}

func TestE2E_ExecuteCustomCommand(t *testing.T) {
	marker := filepath.Join(t.TempDir(), "executed")
	xdg := writeConfig(t, fmt.Sprintf(`
[[commands]]
name = "xq-run-this"
cmd = "touch '%s'"
`, marker))

	sess := startSessionWithEnv(t, map[string]string{"XDG_CONFIG_HOME": xdg})
	defer killSession(t, sess)

	waitForReady(t, sess)

	sendKeys(t, sess, "/")
	time.Sleep(200 * time.Millisecond)
	typeText(t, sess, "xq-run")
	waitForContent(t, sess, func(s string) bool {
		return strings.Contains(s, "filtered")
	}, 5*time.Second)
	sendKeys(t, sess, "Enter")
	time.Sleep(300 * time.Millisecond)
	sendKeys(t, sess, "Enter")

	waitForExit(t, sess)

	if _, err := os.Stat(marker); err != nil {
		t.Errorf("expected marker file to be created: %v", err)
	}
}

func TestE2E_NoConfigFile(t *testing.T) {
	xdg := t.TempDir()
	sess := startSessionWithEnv(t, map[string]string{"XDG_CONFIG_HOME": xdg})
	defer killSession(t, sess)

	content := waitForReady(t, sess)

	if !strings.Contains(content, "window") {
		t.Errorf("expected window items\nCapture:\n%s", content)
	}
	if strings.Contains(content, "config error") {
		t.Errorf("unexpected config error\nCapture:\n%s", content)
	}
}

func TestE2E_MalformedConfigShowsError(t *testing.T) {
	xdg := writeConfig(t, `[[[broken toml`)
	sess := startSessionWithEnv(t, map[string]string{"XDG_CONFIG_HOME": xdg})
	defer killSession(t, sess)

	waitForReady(t, sess)
	sendKeys(t, sess, "End")

	waitForContent(t, sess, func(s string) bool {
		return strings.Contains(s, "config error")
	}, 5*time.Second)

	sendKeys(t, sess, "Escape")
	waitForExit(t, sess)
}

func TestE2E_CWDVisible(t *testing.T) {
	dir, err := os.MkdirTemp("", "cmdk-cwd-e2e-*")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })

	sess := "cmdk-test-" + strings.ReplaceAll(t.Name(), "/", "-")
	cmd := exec.Command("tmux", "new-session", "-d", "-s", sess, "-c", dir, "-x", "120", "-y", "40",
		binaryPath, "--pane-id=%0")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("tmux new-session failed: %v\n%s", err, out)
	}
	defer killSession(t, sess)

	waitForReady(t, sess)
	sendKeys(t, sess, "End")

	waitForContent(t, sess, func(s string) bool {
		return strings.Contains(s, "cmdk-cwd-e2e")
	}, 5*time.Second)
}

func TestE2E_ConfigCommandOrder(t *testing.T) {
	xdg := writeConfig(t, `
[[commands]]
name = "alpha-cmd"
cmd = "echo alpha"

[[commands]]
name = "beta-cmd"
cmd = "echo beta"

[[commands]]
name = "gamma-cmd"
cmd = "echo gamma"
`)
	sess := startSessionWithEnv(t, map[string]string{"XDG_CONFIG_HOME": xdg})
	defer killSession(t, sess)

	waitForReady(t, sess)
	sendKeys(t, sess, "End")

	content := waitForContent(t, sess, func(s string) bool {
		return strings.Contains(s, "alpha-cmd") && strings.Contains(s, "gamma-cmd")
	}, 5*time.Second)

	alphaIdx := strings.Index(content, "alpha-cmd")
	betaIdx := strings.Index(content, "beta-cmd")
	gammaIdx := strings.Index(content, "gamma-cmd")

	if alphaIdx < 0 || betaIdx < 0 || gammaIdx < 0 {
		t.Fatalf("not all commands visible\nCapture:\n%s", content)
	}
	if alphaIdx > betaIdx || betaIdx > gammaIdx {
		t.Errorf("commands not in order: alpha@%d beta@%d gamma@%d\nCapture:\n%s",
			alphaIdx, betaIdx, gammaIdx, content)
	}
}
