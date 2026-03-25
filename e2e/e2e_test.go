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
	iconWindow     = "\uf2d0"
	iconDir        = "\uf07c"
	iconCmd        = "\uf120"
	defaultTimeout = 5 * time.Second
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

func waitForReady(t *testing.T, sess string) {
	t.Helper()
	waitForContent(t, sess, func(s string) bool {
		return strings.Contains(s, "cmdk")
	}, defaultTimeout)
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
	exitFilterModeE2E(t, sess)
	content := capturePane(t, sess)
	cmdCount := strings.Count(content, iconCmd)
	for range cmdCount {
		sendKeys(t, sess, "Down")
		time.Sleep(50 * time.Millisecond)
	}
	waitForContent(t, sess, func(s string) bool {
		return strings.Contains(s, iconDir)
	}, 3*time.Second)
}

func exitFilterModeE2E(t *testing.T, sess string) {
	t.Helper()
	sendKeys(t, sess, "Escape")
	time.Sleep(200 * time.Millisecond)
}

func scrollToWindowItems(t *testing.T, sess string) string {
	t.Helper()
	exitFilterModeE2E(t, sess)
	sendKeys(t, sess, "End")
	return waitForContent(t, sess, func(s string) bool {
		return strings.Contains(s, iconWindow)
	}, defaultTimeout)
}

func filterAndExecute(t *testing.T, sess string, query string) {
	t.Helper()
	typeText(t, sess, query)
	waitForContent(t, sess, func(s string) bool {
		return strings.Contains(s, "apply filter")
	}, defaultTimeout)
	sendKeys(t, sess, "Enter")
}

func selectDirAction(t *testing.T, sess string, actionName string) {
	t.Helper()
	navigateToDirItem(t, sess)
	sendKeys(t, sess, "Enter")

	waitForContent(t, sess, func(s string) bool {
		return strings.Contains(s, actionName)
	}, defaultTimeout)

	sendKeys(t, sess, "Down")
	time.Sleep(100 * time.Millisecond)
	sendKeys(t, sess, "Enter")
}

func assertMarkerContains(t *testing.T, marker string, expected string) {
	t.Helper()
	got, err := os.ReadFile(marker)
	if err != nil {
		t.Fatalf("failed to read marker: %v", err)
	}
	if !strings.Contains(string(got), expected) {
		t.Errorf("expected %q in marker output, got: %s", expected, got)
	}
}

func TestE2E_ItemsVisible(t *testing.T) {
	sess := startSession(t)
	defer killSession(t, sess)

	waitForReady(t, sess)

	content := scrollToWindowItems(t, sess)

	if !strings.Contains(content, sess+":") {
		t.Errorf("expected test session window %q in output\nCapture:\n%s", sess, content)
	}
}

func TestE2E_FilterItems(t *testing.T) {
	sess := startSession(t)
	defer killSession(t, sess)

	waitForReady(t, sess)

	typeText(t, sess, "cmdk-test")

	waitForContent(t, sess, func(s string) bool {
		return strings.Contains(s, sess) && strings.Contains(s, "apply filter")
	}, defaultTimeout)
}

func TestE2E_EscapeDuringFilterDoesNotQuit(t *testing.T) {
	sess := startSession(t)
	defer killSession(t, sess)

	waitForReady(t, sess)

	typeText(t, sess, "cmdk-test")

	waitForContent(t, sess, func(s string) bool {
		return strings.Contains(s, "apply filter")
	}, defaultTimeout)

	sendKeys(t, sess, "Escape")

	waitForContent(t, sess, func(s string) bool {
		return strings.Contains(s, "/ filter") && !strings.Contains(s, "apply filter")
	}, defaultTimeout)

	if !sessionExists(sess) {
		t.Fatal("session should still exist after Escape during filter")
	}
}

func TestE2E_EscapeQuits(t *testing.T) {
	sess := startSession(t)
	defer killSession(t, sess)

	waitForReady(t, sess)

	exitFilterModeE2E(t, sess)
	sendKeys(t, sess, "Escape")
	waitForExit(t, sess)
}

func TestE2E_EnterExecutesAndExits(t *testing.T) {
	sess := startSession(t)
	defer killSession(t, sess)

	waitForReady(t, sess)

	filterAndExecute(t, sess, sess[:20])
	waitForExit(t, sess)
}

func TestE2E_ZoxideDirsVisible(t *testing.T) {
	requireZoxideEntries(t)
	sess := startSession(t)
	defer killSession(t, sess)

	waitForReady(t, sess)
	exitFilterModeE2E(t, sess)

	waitForContent(t, sess, func(s string) bool {
		return strings.Contains(s, iconDir)
	}, defaultTimeout)
}

func TestE2E_SelectDirShowsDirActions(t *testing.T) {
	requireZoxideEntries(t)

	xdg := writeConfig(t, `
[behavior]
auto_select_single = false
`)
	sess := startSessionWithEnv(t, map[string]string{"XDG_CONFIG_HOME": xdg})
	defer killSession(t, sess)

	waitForReady(t, sess)
	navigateToDirItem(t, sess)

	sendKeys(t, sess, "Enter")

	waitForContent(t, sess, func(s string) bool {
		return strings.Contains(s, "New window")
	}, defaultTimeout)
}

func TestE2E_EscapeFromDirActionsReturnsToRoot(t *testing.T) {
	requireZoxideEntries(t)

	xdg := writeConfig(t, `
[behavior]
auto_select_single = false
`)
	sess := startSessionWithEnv(t, map[string]string{"XDG_CONFIG_HOME": xdg})
	defer killSession(t, sess)

	waitForReady(t, sess)
	navigateToDirItem(t, sess)

	sendKeys(t, sess, "Enter")

	waitForContent(t, sess, func(s string) bool {
		return strings.Contains(s, "New window")
	}, defaultTimeout)

	sendKeys(t, sess, "Escape")

	waitForContent(t, sess, func(s string) bool {
		return strings.Contains(s, iconDir)
	}, defaultTimeout)

	if !sessionExists(sess) {
		t.Fatal("session should still exist after Escape from dir-actions")
	}
}

func TestE2E_WithoutZoxide_StillLaunches(t *testing.T) {
	sess := startSession(t)
	defer killSession(t, sess)

	waitForReady(t, sess)
	scrollToWindowItems(t, sess)
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
[[actions]]
name = "my-custom-cmd"
cmd = "echo hello"
matches = "root"
`)
	sess := startSessionWithEnv(t, map[string]string{"XDG_CONFIG_HOME": xdg})
	defer killSession(t, sess)

	waitForReady(t, sess)
	sendKeys(t, sess, "End")

	waitForContent(t, sess, func(s string) bool {
		return strings.Contains(s, "my-custom-cmd")
	}, defaultTimeout)
}

func TestE2E_ExecuteCustomCommand(t *testing.T) {
	marker := filepath.Join(t.TempDir(), "executed")
	xdg := writeConfig(t, fmt.Sprintf(`
[[actions]]
name = "xq-run-this"
cmd = "touch '%s'"
matches = "root"
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

	waitForReady(t, sess)
	exitFilterModeE2E(t, sess)

	topContent := capturePane(t, sess)
	if strings.Contains(topContent, "config error") {
		t.Errorf("unexpected config error\nCapture:\n%s", topContent)
	}

	sendKeys(t, sess, "End")

	waitForContent(t, sess, func(s string) bool {
		return strings.Contains(s, iconWindow)
	}, defaultTimeout)
}

func TestE2E_MalformedConfigShowsError(t *testing.T) {
	xdg := writeConfig(t, `[[[broken toml`)
	sess := startSessionWithEnv(t, map[string]string{"XDG_CONFIG_HOME": xdg})
	defer killSession(t, sess)

	waitForReady(t, sess)
	sendKeys(t, sess, "End")

	waitForContent(t, sess, func(s string) bool {
		return strings.Contains(s, "config error")
	}, defaultTimeout)

	exitFilterModeE2E(t, sess)
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
	typeText(t, sess, "cmdk-cwd-e2e")

	waitForContent(t, sess, func(s string) bool {
		return strings.Contains(s, "cmdk-cwd-e2e-check")
	}, defaultTimeout)
}

func TestE2E_ConfigCommandOrder(t *testing.T) {
	xdg := writeConfig(t, `
[[actions]]
name = "alpha-cmd"
cmd = "echo alpha"
matches = "root"

[[actions]]
name = "beta-cmd"
cmd = "echo beta"
matches = "root"

[[actions]]
name = "gamma-cmd"
cmd = "echo gamma"
matches = "root"
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
	}, defaultTimeout)

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

func TestE2E_DirActionsVisible(t *testing.T) {
	requireZoxideEntries(t)

	xdg := writeConfig(t, `
[[actions]]
name = "xq-yazi-action"
cmd = "echo yazi"
matches = "dir"
`)
	sess := startSessionWithEnv(t, map[string]string{"XDG_CONFIG_HOME": xdg})
	defer killSession(t, sess)

	waitForReady(t, sess)
	navigateToDirItem(t, sess)
	sendKeys(t, sess, "Enter")

	waitForContent(t, sess, func(s string) bool {
		return strings.Contains(s, "New window") && strings.Contains(s, "xq-yazi-action")
	}, defaultTimeout)
}

func TestE2E_DirActionsOrder(t *testing.T) {
	requireZoxideEntries(t)

	xdg := writeConfig(t, `
[[actions]]
name = "xq-alpha-dirc"
cmd = "echo alpha"
matches = "dir"

[[actions]]
name = "xq-beta-dirc"
cmd = "echo beta"
matches = "dir"
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
	}, defaultTimeout)

	newWindowIdx := strings.Index(content, "New window")
	alphaIdx := strings.Index(content, "xq-alpha-dirc")
	betaIdx := strings.Index(content, "xq-beta-dirc")

	if newWindowIdx > alphaIdx || alphaIdx > betaIdx {
		t.Errorf("items not in expected order: New window@%d alpha@%d beta@%d\nCapture:\n%s",
			newWindowIdx, alphaIdx, betaIdx, content)
	}
}

func TestE2E_NoDirActionsShowsDefault(t *testing.T) {
	requireZoxideEntries(t)

	xdg := writeConfig(t, `
[behavior]
auto_select_single = false
`)
	sess := startSessionWithEnv(t, map[string]string{"XDG_CONFIG_HOME": xdg})
	defer killSession(t, sess)

	waitForReady(t, sess)
	navigateToDirItem(t, sess)
	sendKeys(t, sess, "Enter")

	waitForContent(t, sess, func(s string) bool {
		return strings.Contains(s, "New window")
	}, defaultTimeout)
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
[[actions]]
name = "xq-dump-env"
cmd = "env | grep CMDK_ > '%s'"
matches = "root"
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
[[actions]]
name = "xq-pane-id"
cmd = "sh -c 'echo $CMDK_PANE_ID > %s'"
matches = "root"
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

	waitForReady(t, sess)

	exitFilterModeE2E(t, sess)

	waitForContent(t, sess, func(s string) bool {
		return strings.Contains(s, "zoxide error")
	}, defaultTimeout)

	sendKeys(t, sess, "End")

	waitForContent(t, sess, func(s string) bool {
		return strings.Contains(s, iconWindow)
	}, defaultTimeout)
}

func TestE2E_ErrorItemNotSelectable(t *testing.T) {
	if !hasZoxide() {
		t.Skip("zoxide not available — can't test error item scenario")
	}

	path := restrictedPATH(t)
	sess := startSessionWithEnv(t, map[string]string{"PATH": path})
	defer killSession(t, sess)

	waitForReady(t, sess)

	exitFilterModeE2E(t, sess)
	content := capturePane(t, sess)
	cmdCount := strings.Count(content, iconCmd)
	for range cmdCount {
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
[[actions]]
name = "xq-exit-42"
cmd = "exit 42"
matches = "root"
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

	exitFilterModeE2E(t, sess)
	sendKeys(t, sess, "Escape")
	waitForExit(t, sess)
}

func TestE2E_PromptStage(t *testing.T) {
	requireZoxideEntries(t)

	marker := filepath.Join(t.TempDir(), "prompt-result")
	xdg := writeConfig(t, fmt.Sprintf(`
[behavior]
auto_select_single = false

[[actions]]
name = "xq-prompt-action"
matches = "dir"
cmd = "sh -c 'echo {{.branch}} > %s'"
stages = [
  { type = "prompt", text = "Branch:", key = "branch" },
]
`, marker))

	sess := startSessionWithEnv(t, map[string]string{"XDG_CONFIG_HOME": xdg})
	defer killSession(t, sess)

	waitForReady(t, sess)
	selectDirAction(t, sess, "xq-prompt-action")

	waitForContent(t, sess, func(s string) bool {
		return strings.Contains(s, "Branch:")
	}, defaultTimeout)

	typeText(t, sess, "feature/test")
	sendKeys(t, sess, "Enter")
	waitForExit(t, sess)

	assertMarkerContains(t, marker, "feature/test")
}

func TestE2E_PickerStage(t *testing.T) {
	requireZoxideEntries(t)

	marker := filepath.Join(t.TempDir(), "picker-result")
	xdg := writeConfig(t, fmt.Sprintf(`
[behavior]
auto_select_single = false

[[actions]]
name = "xq-picker-action"
matches = "dir"
cmd = "sh -c 'echo {{.chosen}} > %s'"
stages = [
  { type = "picker", source = "printf 'xq-alpha\\nxq-beta\\nxq-gamma'", key = "chosen" },
]
`, marker))

	sess := startSessionWithEnv(t, map[string]string{"XDG_CONFIG_HOME": xdg})
	defer killSession(t, sess)

	waitForReady(t, sess)
	selectDirAction(t, sess, "xq-picker-action")

	waitForContent(t, sess, func(s string) bool {
		return strings.Contains(s, "xq-alpha") && strings.Contains(s, "xq-beta")
	}, defaultTimeout)

	filterAndExecute(t, sess, "xq-alpha")
	waitForExit(t, sess)

	assertMarkerContains(t, marker, "xq-alpha")
}

func TestE2E_RootActionWithStages(t *testing.T) {
	marker := filepath.Join(t.TempDir(), "root-stage-result")
	xdg := writeConfig(t, fmt.Sprintf(`
[[actions]]
name = "xq-root-prompt"
matches = "root"
cmd = "sh -c 'echo {{.val}} > %s'"
stages = [
  { type = "prompt", text = "Value:", key = "val" },
]
`, marker))

	sess := startSessionWithEnv(t, map[string]string{"XDG_CONFIG_HOME": xdg})
	defer killSession(t, sess)

	waitForReady(t, sess)
	filterAndExecute(t, sess, "xq-root")

	waitForContent(t, sess, func(s string) bool {
		return strings.Contains(s, "Value:")
	}, defaultTimeout)

	typeText(t, sess, "hello")
	sendKeys(t, sess, "Enter")
	waitForExit(t, sess)

	assertMarkerContains(t, marker, "hello")
}

func TestE2E_EscFromPromptStage(t *testing.T) {
	requireZoxideEntries(t)

	xdg := writeConfig(t, `
[behavior]
auto_select_single = false

[[actions]]
name = "xq-esc-prompt"
matches = "dir"
cmd = "echo {{.val}}"
stages = [
  { type = "prompt", text = "Value:", key = "val" },
]
`)

	sess := startSessionWithEnv(t, map[string]string{"XDG_CONFIG_HOME": xdg})
	defer killSession(t, sess)

	waitForReady(t, sess)
	selectDirAction(t, sess, "xq-esc-prompt")

	waitForContent(t, sess, func(s string) bool {
		return strings.Contains(s, "Value:")
	}, defaultTimeout)

	sendKeys(t, sess, "Escape")

	waitForContent(t, sess, func(s string) bool {
		return strings.Contains(s, "xq-esc-prompt") && strings.Contains(s, "New window")
	}, defaultTimeout)

	if !sessionExists(sess) {
		t.Fatal("session should still exist after Esc from prompt stage")
	}
}
