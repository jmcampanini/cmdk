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

const (
	iconWindow = "\uf2d0"
	iconDir    = "\uf07c"
)

var (
	binaryPath string
	tmuxSocket string
)

func tmuxCmd(args ...string) *exec.Cmd {
	return exec.Command("tmux", append([]string{"-L", tmuxSocket}, args...)...)
}

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
	tmuxSocket = fmt.Sprintf("cmdk-e2e-%d", os.Getpid())
	build := exec.Command("go", "build", "-o", binaryPath, "..")
	build.Stderr = os.Stderr
	if err := build.Run(); err != nil {
		panic("failed to build binary: " + err.Error())
	}

	code := m.Run()
	_ = tmuxCmd("kill-server").Run()
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
	cmd := tmuxCmd(args...)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("tmux new-session failed: %v\n%s", err, out)
	}
	return sess
}

func killSession(t *testing.T, sess string) {
	t.Helper()
	if err := tmuxCmd("kill-session", "-t", sess).Run(); err != nil {
		t.Logf("warning: failed to kill tmux session %s: %v", sess, err)
	}
}

func capturePane(t *testing.T, sess string) string {
	t.Helper()
	out, err := tmuxCmd("capture-pane", "-t", sess, "-p").Output()
	if err != nil {
		t.Fatalf("capture-pane failed: %v", err)
	}
	return string(out)
}

func sendKeys(t *testing.T, sess string, keys ...string) {
	t.Helper()
	args := append([]string{"send-keys", "-t", sess}, keys...)
	if out, err := tmuxCmd(args...).CombinedOutput(); err != nil {
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
	return tmuxCmd("has-session", "-t", sess).Run() == nil
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
	windowCount := strings.Count(content, iconWindow)
	for range windowCount {
		sendKeys(t, sess, "Down")
		time.Sleep(50 * time.Millisecond)
	}
	waitForContent(t, sess, func(s string) bool {
		return strings.Contains(s, iconDir)
	}, 3*time.Second)
}

func filterAndExecute(t *testing.T, sess string, query string) {
	t.Helper()
	sendKeys(t, sess, "/")
	time.Sleep(200 * time.Millisecond)
	typeText(t, sess, query)
	waitForContent(t, sess, func(s string) bool {
		return strings.Contains(s, "apply filter")
	}, 5*time.Second)
	sendKeys(t, sess, "Enter")
	time.Sleep(300 * time.Millisecond)
	sendKeys(t, sess, "Enter")
}

func TestE2E_ItemsVisible(t *testing.T) {
	sess := startSession(t)
	defer killSession(t, sess)

	content := waitForReady(t, sess)

	if !strings.Contains(content, sess+":") {
		t.Errorf("expected test session window %q in output\nCapture:\n%s", sess, content)
	}

	if !strings.Contains(content, iconWindow) {
		t.Errorf("expected window icon in output\nCapture:\n%s", content)
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
		return strings.Contains(s, sess) && strings.Contains(s, "apply filter")
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
		return strings.Contains(s, "apply filter")
	}, 5*time.Second)

	sendKeys(t, sess, "Escape")

	waitForContent(t, sess, func(s string) bool {
		return strings.Contains(s, sess) && !strings.Contains(s, "apply filter")
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

	if !strings.Contains(content, iconDir) {
		t.Errorf("expected dir icon in output when zoxide available\nCapture:\n%s", content)
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
		return strings.Contains(s, iconWindow)
	}, 5*time.Second)

	if !sessionExists(sess) {
		t.Fatal("session should still exist after Escape from dir-actions")
	}
}

func TestE2E_WithoutZoxide_StillLaunches(t *testing.T) {
	sess := startSession(t)
	defer killSession(t, sess)

	content := waitForReady(t, sess)

	if !strings.Contains(content, iconWindow) {
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
	filterAndExecute(t, sess, "xq-run")
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

	if !strings.Contains(content, iconWindow) {
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
	dir := filepath.Join(t.TempDir(), "cmdk-cwd-e2e-check")
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatal(err)
	}

	sess := "cmdk-test-" + strings.ReplaceAll(t.Name(), "/", "-")
	cmd := tmuxCmd("new-session", "-d", "-s", sess, "-c", dir, "-x", "120", "-y", "40",
		binaryPath, "--pane-id=%0")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("tmux new-session failed: %v\n%s", err, out)
	}
	defer killSession(t, sess)

	waitForReady(t, sess)
	sendKeys(t, sess, "End")

	waitForContent(t, sess, func(s string) bool {
		return strings.Contains(s, "cmdk-cwd-e2e-check")
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
	sess := "cmdk-test-" + strings.ReplaceAll(t.Name(), "/", "-")
	cmd := tmuxCmd("new-session", "-d", "-s", sess, "-x", "120", "-y", "80",
		"env", "XDG_CONFIG_HOME="+xdg, binaryPath, "--pane-id=%0")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("tmux new-session failed: %v\n%s", err, out)
	}
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

func TestE2E_DirCommandsVisible(t *testing.T) {
	requireZoxideEntries(t)

	xdg := writeConfig(t, `
[[dir_commands]]
name = "xq-yazi-action"
cmd = "echo yazi"
`)
	sess := startSessionWithEnv(t, map[string]string{"XDG_CONFIG_HOME": xdg})
	defer killSession(t, sess)

	waitForReady(t, sess)
	navigateToDirItem(t, sess)
	sendKeys(t, sess, "Enter")

	waitForContent(t, sess, func(s string) bool {
		return strings.Contains(s, "New window") && strings.Contains(s, "xq-yazi-action")
	}, 5*time.Second)
}

func TestE2E_DirCommandsOrder(t *testing.T) {
	requireZoxideEntries(t)

	xdg := writeConfig(t, `
[[dir_commands]]
name = "xq-alpha-dirc"
cmd = "echo alpha"

[[dir_commands]]
name = "xq-beta-dirc"
cmd = "echo beta"
`)
	sess := startSessionWithEnv(t, map[string]string{"XDG_CONFIG_HOME": xdg})
	defer killSession(t, sess)

	waitForReady(t, sess)
	navigateToDirItem(t, sess)
	sendKeys(t, sess, "Enter")

	content := waitForContent(t, sess, func(s string) bool {
		return strings.Contains(s, "New window") &&
			strings.Contains(s, "xq-alpha-dirc") &&
			strings.Contains(s, "xq-beta-dirc")
	}, 5*time.Second)

	newWindowIdx := strings.Index(content, "New window")
	alphaIdx := strings.Index(content, "xq-alpha-dirc")
	betaIdx := strings.Index(content, "xq-beta-dirc")

	if newWindowIdx > alphaIdx || alphaIdx > betaIdx {
		t.Errorf("items not in expected order: New window@%d alpha@%d beta@%d\nCapture:\n%s",
			newWindowIdx, alphaIdx, betaIdx, content)
	}
}

func TestE2E_NoDirCommandsShowsDefault(t *testing.T) {
	requireZoxideEntries(t)

	xdg := t.TempDir()
	sess := startSessionWithEnv(t, map[string]string{"XDG_CONFIG_HOME": xdg})
	defer killSession(t, sess)

	waitForReady(t, sess)
	navigateToDirItem(t, sess)
	sendKeys(t, sess, "Enter")

	waitForContent(t, sess, func(s string) bool {
		return strings.Contains(s, "New window")
	}, 5*time.Second)
}

func restrictedPATH(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	tmuxPath, err := exec.LookPath("tmux")
	if err != nil {
		t.Fatal("tmux not found")
	}
	if err := os.Symlink(tmuxPath, filepath.Join(dir, "tmux")); err != nil {
		t.Fatal(err)
	}
	return dir + ":/usr/bin:/bin"
}

func TestE2E_EnvVarsPresent(t *testing.T) {
	marker := filepath.Join(t.TempDir(), "cmdk-env")
	xdg := writeConfig(t, fmt.Sprintf(`
[[commands]]
name = "xq-dump-env"
cmd = "env | grep CMDK_ > '%s'"
`, marker))

	sess := startSessionWithEnv(t, map[string]string{"XDG_CONFIG_HOME": xdg})
	defer killSession(t, sess)

	waitForReady(t, sess)
	filterAndExecute(t, sess, "xq-dump")
	waitForExit(t, sess)

	got, err := os.ReadFile(marker)
	if err != nil {
		t.Fatalf("failed to read env dump: %v", err)
	}
	if !strings.Contains(string(got), "CMDK_PANE_ID=") {
		t.Errorf("expected CMDK_PANE_ID in env dump\nGot:\n%s", got)
	}
}

func TestE2E_CMDKPaneIDInShell(t *testing.T) {
	marker := filepath.Join(t.TempDir(), "pane-id")
	xdg := writeConfig(t, fmt.Sprintf(`
[[commands]]
name = "xq-pane-id"
cmd = "sh -c 'echo $CMDK_PANE_ID > %s'"
`, marker))

	sess := startSessionWithEnv(t, map[string]string{"XDG_CONFIG_HOME": xdg})
	defer killSession(t, sess)

	waitForReady(t, sess)
	filterAndExecute(t, sess, "xq-pane")
	waitForExit(t, sess)

	data, err := os.ReadFile(marker)
	if err != nil {
		t.Fatalf("failed to read pane-id marker: %v", err)
	}
	got := strings.TrimSpace(string(data))
	if got != "%0" {
		t.Errorf("CMDK_PANE_ID = %q, want %%0", got)
	}
}

func TestE2E_ZoxideUnavailable_ErrorItem(t *testing.T) {
	if !hasZoxide() {
		t.Skip("zoxide not available — can't test unavailability scenario")
	}

	path := restrictedPATH(t)
	sess := startSessionWithEnv(t, map[string]string{"PATH": path})
	defer killSession(t, sess)

	content := waitForReady(t, sess)

	if !strings.Contains(content, iconWindow) {
		t.Errorf("expected window items\nCapture:\n%s", content)
	}

	sendKeys(t, sess, "End")
	waitForContent(t, sess, func(s string) bool {
		return strings.Contains(s, "zoxide error")
	}, 5*time.Second)
}

func TestE2E_ErrorItemNotSelectable(t *testing.T) {
	if !hasZoxide() {
		t.Skip("zoxide not available — can't test error item scenario")
	}

	path := restrictedPATH(t)
	sess := startSessionWithEnv(t, map[string]string{"PATH": path})
	defer killSession(t, sess)

	waitForReady(t, sess)

	content := capturePane(t, sess)
	windowCount := strings.Count(content, iconWindow)
	for range windowCount {
		sendKeys(t, sess, "Down")
		time.Sleep(50 * time.Millisecond)
	}

	waitForContent(t, sess, func(s string) bool {
		return strings.Contains(s, "zoxide error")
	}, 3*time.Second)

	sendKeys(t, sess, "Enter")
	time.Sleep(500 * time.Millisecond)

	if !sessionExists(sess) {
		t.Fatal("session should still exist — error items should not be selectable")
	}

	content = capturePane(t, sess)
	if !strings.Contains(content, "zoxide error") {
		t.Errorf("error item should still be visible\nCapture:\n%s", content)
	}
}

func TestE2E_ExitCodePropagation(t *testing.T) {
	marker := filepath.Join(t.TempDir(), "exitcode")
	xdg := writeConfig(t, `
[[commands]]
name = "xq-exit-42"
cmd = "exit 42"
`)

	sess := "cmdk-test-" + strings.ReplaceAll(t.Name(), "/", "-")
	shellCmd := fmt.Sprintf("(%s --pane-id=%%0); echo EXITCODE=$? > '%s'", binaryPath, marker)
	cmd := tmuxCmd("new-session", "-d", "-s", sess, "-x", "120", "-y", "40",
		"env", "XDG_CONFIG_HOME="+xdg, "sh", "-c", shellCmd)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("tmux new-session failed: %v\n%s", err, out)
	}
	defer killSession(t, sess)

	waitForReady(t, sess)
	filterAndExecute(t, sess, "xq-exit")
	waitForExit(t, sess)

	got, err := os.ReadFile(marker)
	if err != nil {
		t.Fatalf("failed to read exitcode marker: %v", err)
	}
	if !strings.Contains(string(got), "EXITCODE=42") {
		t.Errorf("expected EXITCODE=42, got: %s", got)
	}
}

func TestE2E_DisplayPopup(t *testing.T) {
	sess := "cmdk-test-" + strings.ReplaceAll(t.Name(), "/", "-")
	shellCmd := fmt.Sprintf("%s --pane-id=$(tmux -L %s display-message -p '#{pane_id}')", binaryPath, tmuxSocket)
	cmd := tmuxCmd("new-session", "-d", "-s", sess, "-x", "120", "-y", "40", "sh", "-c", shellCmd)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("tmux new-session failed: %v\n%s", err, out)
	}
	defer killSession(t, sess)

	waitForReady(t, sess)

	sendKeys(t, sess, "Escape")
	waitForExit(t, sess)
}
