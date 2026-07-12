package e2e

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	cmdktmux "github.com/jmcampanini/cmdk/internal/tmux"
)

const (
	iconWindow     = "\ueb7f"
	iconDir        = "\ueaf7"
	iconAction     = "\ueb63"
	iconSession    = "\ueb23"
	defaultTimeout = 5 * time.Second
)

var (
	binaryPath string
	tmuxSocket string
)

func tmuxCmdForSocket(socket string, args ...string) *exec.Cmd {
	return exec.Command("tmux", append([]string{"-L", socket, "-f", "/dev/null"}, args...)...)
}

func tmuxCmd(args ...string) *exec.Cmd {
	return tmuxCmdForSocket(tmuxSocket, args...)
}

func useIsolatedTmuxSocket(t *testing.T) {
	t.Helper()
	// Tests that need an empty tmux server should not kill the shared e2e
	// server mid-package; on CI, starting a new session on a just-killed socket
	// can race the server shutdown and fail with "server exited unexpectedly".
	oldSocket := tmuxSocket
	socket := fmt.Sprintf("%s-%d", oldSocket, time.Now().UnixNano())
	tmuxSocket = socket
	t.Cleanup(func() {
		_ = tmuxCmdForSocket(socket, "kill-server").Run()
		tmuxSocket = oldSocket
	})
}

func TestMain(m *testing.M) {
	tmuxSocket = fmt.Sprintf("cmdk-e2e-%d", os.Getpid())
	if err := cmdktmux.CheckPrerequisite(context.Background()); err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: e2e tests: %v\n", err)
		os.Exit(1)
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
	_ = tmuxCmd("kill-server").Run()
	if err := os.RemoveAll(tmp); err != nil {
		fmt.Fprintf(os.Stderr, "warning: failed to clean up %s: %v\n", tmp, err)
	}
	os.Exit(code)
}

func TestE2E_TmuxSentinel(t *testing.T) {
	useIsolatedTmuxSocket(t)
	sess := "cmdk-tmux-sentinel"
	if out, err := tmuxCmd("new-session", "-d", "-s", sess).CombinedOutput(); err != nil {
		t.Fatalf("tmux new-session failed: %v\n%s", err, out)
	}
	out, err := tmuxCmd("display-message", "-p", "-t", sess, "#{session_name}").CombinedOutput()
	if err != nil {
		t.Fatalf("tmux display-message failed: %v\n%s", err, out)
	}
	if got := strings.TrimSpace(string(out)); got != sess {
		t.Fatalf("tmux session name = %q, want %q", got, sess)
	}
	fmt.Println("CMDK_E2E_TMUX_SENTINEL=PASS")
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

func waitForFile(t *testing.T, path string, timeout time.Duration) string {
	t.Helper()
	deadline := time.Now().Add(timeout)
	var lastErr error
	for time.Now().Before(deadline) {
		data, err := os.ReadFile(path)
		if err == nil {
			return string(data)
		}
		lastErr = err
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s: %v", path, lastErr)
	return ""
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

func requireGitE2E(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
}

func runGitE2E(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=cmdk e2e",
		"GIT_AUTHOR_EMAIL=cmdk@example.invalid",
		"GIT_COMMITTER_NAME=cmdk e2e",
		"GIT_COMMITTER_EMAIL=cmdk@example.invalid",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git -C %s %s failed: %v\n%s", dir, strings.Join(args, " "), err, out)
	}
	return string(out)
}

func initRepoE2E(t *testing.T, path string) {
	t.Helper()
	requireGitE2E(t)
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("git", "init", path)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git init failed: %v\n%s", err, out)
	}
	runGitE2E(t, path, "config", "user.name", "cmdk e2e")
	runGitE2E(t, path, "config", "user.email", "cmdk@example.invalid")
	if err := os.WriteFile(filepath.Join(path, "README.md"), []byte("test\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGitE2E(t, path, "add", "README.md")
	runGitE2E(t, path, "commit", "-m", "initial")
}

func realPathE2E(t *testing.T, path string) string {
	t.Helper()
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		t.Fatalf("EvalSymlinks(%q): %v", path, err)
	}
	return filepath.Clean(resolved)
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
	content := waitForContent(t, sess, func(s string) bool {
		return strings.Contains(s, iconDir)
	}, 3*time.Second)
	firstDir := strings.Index(content, iconDir)
	contentBeforeFirstDir := content[:firstDir]
	itemsBeforeDirs := strings.Count(contentBeforeFirstDir, iconWindow) +
		strings.Count(contentBeforeFirstDir, iconSession) +
		strings.Count(contentBeforeFirstDir, iconAction)
	for range itemsBeforeDirs {
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

func windowFilterQuery(sess string) string {
	return "tmux win " + sess
}

func sessionWindowMarker(sess string) string {
	return "‹ " + sess
}

func navigateToWindowItems(t *testing.T, sess string) string {
	t.Helper()
	sendKeys(t, sess, "/")
	query := windowFilterQuery(sess)
	typeText(t, sess, query)
	marker := sessionWindowMarker(sess)
	return waitForContent(t, sess, func(s string) bool {
		return strings.Contains(s, iconWindow) && strings.Contains(s, marker)
	}, defaultTimeout)
}

func showWindowItems(t *testing.T, sess string) string {
	t.Helper()
	exitFilterModeE2E(t, sess)
	return navigateToWindowItems(t, sess)
}

func filterAndExecute(t *testing.T, sess string, query string) {
	t.Helper()
	typeText(t, sess, query)
	waitForContent(t, sess, func(s string) bool {
		return strings.Contains(s, "apply filter")
	}, defaultTimeout)
	sendKeys(t, sess, "Enter")
}

func drillIntoSession(t *testing.T, sess string) {
	t.Helper()
	typeText(t, sess, sess)
	waitForContent(t, sess, func(s string) bool {
		return strings.Contains(s, "apply filter")
	}, defaultTimeout)

	// Applying the filter usually reveals the session and its matching window.
	// When only the session remains, auto-select may drill into it immediately.
	sendKeys(t, sess, "Enter")
	content := waitForContent(t, sess, func(s string) bool {
		return strings.Contains(s, "clear filter") || strings.Contains(s, "Switch to session")
	}, defaultTimeout)
	if strings.Contains(content, "Switch to session") {
		return
	}

	// The exact session row ranks above its longer matching window rows, so Home
	// deterministically selects the session row.
	sendKeys(t, sess, "Home")
	sendKeys(t, sess, "Enter")

	waitForContent(t, sess, func(s string) bool {
		return strings.Contains(s, "Switch to session")
	}, defaultTimeout)
}

func selectDirAction(t *testing.T, sess string, actionName string) {
	t.Helper()
	navigateToDirItem(t, sess)
	sendKeys(t, sess, "Enter")

	waitForContent(t, sess, func(s string) bool {
		return strings.Contains(s, actionName)
	}, defaultTimeout)

	// Drill-down re-enters filter mode (start_in_filter default), so no '/' needed.
	filterAndExecute(t, sess, actionName)
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

	content := showWindowItems(t, sess)

	marker := sessionWindowMarker(sess)
	if !strings.Contains(content, marker) {
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

	filterAndExecute(t, sess, windowFilterQuery(sess))
	waitForExit(t, sess)
}

func TestE2E_SessionsVisible(t *testing.T) {
	sess := startSession(t)
	defer killSession(t, sess)

	waitForReady(t, sess)

	// Filter to the current session so it is on screen regardless of how many
	// other root items sort above it; the session renders with the session icon.
	typeText(t, sess, sess)
	waitForContent(t, sess, func(s string) bool {
		return strings.Contains(s, iconSession)
	}, defaultTimeout)
}

func TestE2E_SessionDrillInOrder(t *testing.T) {
	xdg := writeConfig(t, `
[[actions]]
name = "xq-sess-action"
cmd = "true"
matches = "session"
`)
	sess := startSessionWithEnv(t, map[string]string{"XDG_CONFIG_HOME": xdg})
	defer killSession(t, sess)

	waitForReady(t, sess)
	drillIntoSession(t, sess)

	content := waitForContent(t, sess, func(s string) bool {
		return strings.Contains(s, "Switch to session") &&
			strings.Contains(s, "xq-sess-action") &&
			strings.Contains(s, iconWindow)
	}, defaultTimeout)

	switchIdx := strings.Index(content, "Switch to session")
	actionIdx := strings.Index(content, "xq-sess-action")
	windowIdx := strings.Index(content, iconWindow)
	if windowIdx < 0 || windowIdx > switchIdx || switchIdx > actionIdx {
		t.Errorf("expected order window < Switch to session < session action; got window@%d switch@%d action@%d:\n%s",
			windowIdx, switchIdx, actionIdx, content)
	}
}

func TestE2E_SessionSwitchActionSwitchesAndExits(t *testing.T) {
	sess := startSession(t)
	defer killSession(t, sess)

	waitForReady(t, sess)
	drillIntoSession(t, sess)

	// Windows sort before actions in session child lists, so filter to the
	// built-in action before executing it.
	filterAndExecute(t, sess, "Switch to session")
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

	// First Escape exits filter mode (drill-down re-enters filter).
	exitFilterModeE2E(t, sess)

	// Second Escape navigates back to root.
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
	showWindowItems(t, sess)
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

func shellQuoteE2E(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}

func runDetachedSessionWindowNew(t *testing.T, path string) string {
	t.Helper()
	xdg := writeConfig(t, "")
	marker := filepath.Join(t.TempDir(), "window-exit")
	sess := "cmdk-window-" + strings.ReplaceAll(t.Name(), "/", "-") + fmt.Sprintf("-%d", time.Now().UnixNano())
	shellCmd := fmt.Sprintf("%s session window %s --new; echo EXITCODE=$? > %s; sleep 1",
		shellQuoteE2E(binaryPath), shellQuoteE2E(path), shellQuoteE2E(marker))
	cmd := tmuxCmd("new-session", "-d", "-s", sess, "-x", "120", "-y", "40",
		"env", "XDG_CONFIG_HOME="+xdg, "sh", "-c", shellCmd)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("tmux new-session failed: %v\n%s", err, out)
	}
	t.Cleanup(func() { killSession(t, sess) })
	return waitForFile(t, marker, defaultTimeout)
}

type managedSessionE2E struct {
	ID   string
	Kind string
}

func findManagedSessionE2E(t *testing.T, key string) managedSessionE2E {
	t.Helper()
	out, err := tmuxCmd("list-sessions", "-F", "#{session_id}\t#{@cmdk_session_kind}\t#{@cmdk_session_key}").Output()
	if err != nil {
		t.Fatalf("list-sessions failed: %v", err)
	}
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimRight(line, "\r")
		if line == "" {
			continue
		}
		fields := strings.Split(line, "\t")
		if len(fields) != 3 {
			t.Fatalf("malformed list-sessions row %q", line)
		}
		if fields[2] == key {
			return managedSessionE2E{ID: fields[0], Kind: fields[1]}
		}
	}
	t.Fatalf("no managed session found for key %q\n%s", key, out)
	return managedSessionE2E{}
}

func currentWindowNameE2E(t *testing.T, sessionID string) string {
	t.Helper()
	out, err := tmuxCmd("list-windows", "-t", sessionID, "-f", "#{window_active}", "-F", "#{window_name}").Output()
	if err != nil {
		t.Fatalf("list active window failed: %v", err)
	}
	return strings.TrimSpace(string(out))
}

func windowNamesE2E(t *testing.T, sessionID string) map[string]string {
	t.Helper()
	out, err := tmuxCmd("list-windows", "-t", sessionID, "-F", "#{window_name}\t#{pane_current_path}").Output()
	if err != nil {
		t.Fatalf("list-windows failed: %v", err)
	}
	windows := map[string]string{}
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimRight(line, "\r")
		if line == "" {
			continue
		}
		fields := strings.Split(line, "\t")
		if len(fields) != 2 {
			t.Fatalf("malformed list-windows row %q", line)
		}
		windows[fields[0]] = fields[1]
	}
	return windows
}

func TestE2E_SessionWindowCreatesNonGitDirectorySession(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "scratch")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	dirReal := realPathE2E(t, dir)

	result := runDetachedSessionWindowNew(t, dir)
	if result != "EXITCODE=0\n" {
		t.Fatalf("window exit marker = %q, want EXITCODE=0", result)
	}

	session := findManagedSessionE2E(t, dirReal)
	if session.Kind != "directory" {
		t.Errorf("session kind = %q, want directory", session.Kind)
	}
	windows := windowNamesE2E(t, session.ID)
	if windows["scratch"] != dirReal {
		t.Errorf("scratch window cwd = %q, want %q (all windows: %v)", windows["scratch"], dirReal, windows)
	}
}

func TestE2E_SessionWindowCommandCreatesRunningCommandWindow(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "scratch")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	dirReal := realPathE2E(t, dir)
	commandMarker := filepath.Join(t.TempDir(), "command-ran")
	exitMarker := filepath.Join(t.TempDir(), "window-exit")
	xdg := writeConfig(t, "")
	sess := "cmdk-window-command-" + strings.ReplaceAll(t.Name(), "/", "-") + fmt.Sprintf("-%d", time.Now().UnixNano())
	commandScript := fmt.Sprintf("printf done > %s; sleep 1", shellQuoteE2E(commandMarker))
	shellCmd := fmt.Sprintf("%s session window %s --name runner -- sh -lc %s; echo EXITCODE=$? > %s; sleep 1",
		shellQuoteE2E(binaryPath), shellQuoteE2E(dir), shellQuoteE2E(commandScript), shellQuoteE2E(exitMarker))
	cmd := tmuxCmd("new-session", "-d", "-s", sess, "-x", "120", "-y", "40",
		"env", "XDG_CONFIG_HOME="+xdg, "sh", "-c", shellCmd)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("tmux new-session failed: %v\n%s", err, out)
	}
	t.Cleanup(func() { killSession(t, sess) })

	if result := waitForFile(t, exitMarker, defaultTimeout); result != "EXITCODE=0\n" {
		t.Fatalf("window exit marker = %q, want EXITCODE=0", result)
	}
	if got := waitForFile(t, commandMarker, defaultTimeout); got != "done" {
		t.Fatalf("command marker = %q, want done", got)
	}

	session := findManagedSessionE2E(t, dirReal)
	windows := windowNamesE2E(t, session.ID)
	if windows["runner"] != dirReal {
		t.Errorf("runner window cwd = %q, want %q (all windows: %v)", windows["runner"], dirReal, windows)
	}
}

func TestE2E_SessionWindowCreatesRepoWorktreeSessionAndWindows(t *testing.T) {
	requireGitE2E(t)
	container := filepath.Join(t.TempDir(), "dotfiles")
	main := filepath.Join(container, "main")
	feature := filepath.Join(container, "wt-feature")
	initRepoE2E(t, main)
	runGitE2E(t, main, "worktree", "add", "-b", "cmdk-e2e-feature", feature)

	containerReal := realPathE2E(t, container)
	featureReal := realPathE2E(t, feature)
	mainReal := realPathE2E(t, main)

	result := runDetachedSessionWindowNew(t, feature)
	if result != "EXITCODE=0\n" {
		t.Fatalf("window exit marker = %q, want EXITCODE=0", result)
	}

	session := findManagedSessionE2E(t, containerReal)
	if session.Kind != "repo" {
		t.Errorf("session kind = %q, want repo", session.Kind)
	}
	windows := windowNamesE2E(t, session.ID)
	if windows["wt-feature"] != featureReal {
		t.Errorf("wt-feature window cwd = %q, want %q (all windows: %v)", windows["wt-feature"], featureReal, windows)
	}
	if active := currentWindowNameE2E(t, session.ID); active != "wt-feature" {
		t.Fatalf("active window = %q, want wt-feature", active)
	}

	result = runDetachedSessionWindowNew(t, main)
	if result != "EXITCODE=0\n" {
		t.Fatalf("second window exit marker = %q, want EXITCODE=0", result)
	}
	if active := currentWindowNameE2E(t, session.ID); active != "wt-feature" {
		t.Errorf("active window changed to %q, want wt-feature", active)
	}
	windows = windowNamesE2E(t, session.ID)
	if windows["main"] != mainReal {
		t.Errorf("main window cwd = %q, want %q (all windows: %v)", windows["main"], mainReal, windows)
	}
}

func TestE2E_ConfigCommandsVisible(t *testing.T) {
	xdg := writeConfig(t, `
[[actions]]
name = "xq-visible-cmd"
cmd = "echo hello"
matches = "root"
`)
	sess := startSessionWithEnv(t, map[string]string{"XDG_CONFIG_HOME": xdg})
	defer killSession(t, sess)

	waitForReady(t, sess)
	typeText(t, sess, "xq-visible-cmd")

	waitForContent(t, sess, func(s string) bool {
		return strings.Contains(s, "xq-visible-cmd")
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

func TestE2E_RootLaunchPathActionCreatesManagedSession(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "scratch")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	dirClean := filepath.Clean(dir)
	dirReal := realPathE2E(t, dir)
	marker := filepath.Join(t.TempDir(), "root-launch-path")
	xdg := writeConfig(t, fmt.Sprintf(`
[[actions]]
name = "xq-launch-path"
matches = "root"
launch_path = "%s"
cmd = "printf '%%s\\n%%s\\n' \"$PWD\" {{sq .launch_path}} > '%s'; sleep 30"
`, dir, marker))

	sess := startSessionWithEnv(t, map[string]string{"XDG_CONFIG_HOME": xdg})
	defer killSession(t, sess)

	waitForReady(t, sess)
	filterAndExecute(t, sess, "xq-launch-path")

	lines := strings.Split(strings.TrimSpace(waitForFile(t, marker, defaultTimeout)), "\n")
	if len(lines) != 2 {
		t.Fatalf("marker lines = %#v, want 2", lines)
	}
	wantLines := []string{dirReal, dirClean}
	for i, got := range lines {
		if got != wantLines[i] {
			t.Fatalf("marker line %d = %q, want %q (all lines: %#v)", i, got, wantLines[i], lines)
		}
	}

	session := findManagedSessionE2E(t, dirReal)
	windows := windowNamesE2E(t, session.ID)
	if windows["scratch"] != dirReal {
		t.Errorf("scratch window cwd = %q, want %q (all windows: %v)", windows["scratch"], dirReal, windows)
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

	navigateToWindowItems(t, sess)
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

func TestE2E_ConfigCommandOrder(t *testing.T) {
	useIsolatedTmuxSocket(t)
	xdg := writeConfig(t, `
[[actions]]
name = "xqorder-alpha"
cmd = "echo alpha"
matches = "root"

[[actions]]
name = "xqorder-beta"
cmd = "echo beta"
matches = "root"

[[actions]]
name = "xqorder-gamma"
cmd = "echo gamma"
matches = "root"
`)
	sess := "cmdk-test-" + strings.ReplaceAll(t.Name(), "/", "-")
	cmd := tmuxCmd("new-session", "-d", "-s", sess, "-x", "120", "-y", "80",
		"env", "XDG_CONFIG_HOME="+xdg, "PATH="+restrictedPATH(t), binaryPath, "--pane-id=%0")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("tmux new-session failed: %v\n%s", err, out)
	}
	defer killSession(t, sess)

	waitForReady(t, sess)
	exitFilterModeE2E(t, sess)
	sendKeys(t, sess, "End")

	content := waitForContent(t, sess, func(s string) bool {
		return strings.Contains(s, "xqorder-alpha") && strings.Contains(s, "xqorder-gamma")
	}, defaultTimeout)

	alphaIdx := strings.Index(content, "xqorder-alpha")
	betaIdx := strings.Index(content, "xqorder-beta")
	gammaIdx := strings.Index(content, "xqorder-gamma")

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
		return strings.Contains(s, "New window") &&
			strings.Contains(s, "xq-yazi-action")
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

	content := waitForContent(t, sess, func(s string) bool {
		return strings.Contains(s, "New window")
	}, defaultTimeout)
	if strings.Contains(content, "New tmux window") {
		t.Fatalf("unexpected removed built-in action in capture:\n%s", content)
	}
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

	navigateToWindowItems(t, sess)
}

func TestE2E_ErrorItemOpensDetails(t *testing.T) {
	if !hasZoxide() {
		t.Skip("zoxide not available — can't test error item scenario")
	}

	path := restrictedPATH(t)
	sess := startSessionWithEnv(t, map[string]string{"PATH": path})
	defer killSession(t, sess)

	waitForReady(t, sess)

	typeText(t, sess, "zoxide error")
	waitForContent(t, sess, func(s string) bool {
		return strings.Contains(s, "zoxide error")
	}, 3*time.Second)

	sendKeys(t, sess, "Enter")
	waitForContent(t, sess, func(s string) bool {
		return strings.Contains(s, "Error details") && strings.Contains(s, "zoxide error")
	}, defaultTimeout)

	if !sessionExists(sess) {
		t.Fatal("session should still exist after opening error details")
	}

	sendKeys(t, sess, "Escape")
	waitForContent(t, sess, func(s string) bool {
		return strings.Contains(s, "zoxide error") && !strings.Contains(s, "Error details")
	}, defaultTimeout)
}

func TestE2E_LongPickerErrorDetailsWrapsAndScrolls(t *testing.T) {
	tmp := t.TempDir()
	script := filepath.Join(tmp, "long-picker-error.sh")
	scriptBody := `#!/bin/sh
i=1
while [ "$i" -le 140 ]; do
  printf 'ERR %03d this is a deliberately very long stderr line for cmdk wrapping and scrolling verification with enough words to wrap naturally across a narrow pane; token=%s\n' "$i" "abcdefghijklmnopqrstuvwxyz0123456789abcdefghijklmnopqrstuvwxyz0123456789" >&2
  i=$((i + 1))
done
exit 7
`
	if err := os.WriteFile(script, []byte(scriptBody), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(script, 0o755); err != nil {
		t.Fatal(err)
	}

	xdg := writeConfig(t, fmt.Sprintf(`
[[actions]]
name = "Long Picker Error"
matches = "root"
cmd = "true"
stages = [
  { type = "picker", key = "choice", source = %q },
]
`, shellQuoteE2E(script)))
	sess := startSessionWithEnv(t, map[string]string{"XDG_CONFIG_HOME": xdg})
	defer killSession(t, sess)

	waitForReady(t, sess)
	filterAndExecute(t, sess, "Long")

	waitForContent(t, sess, func(s string) bool {
		return strings.Contains(s, "picker source failed")
	}, defaultTimeout)

	sendKeys(t, sess, "Enter")
	content := waitForContent(t, sess, func(s string) bool {
		return strings.Contains(s, "Error details") &&
			strings.Contains(s, "ERR 001") &&
			strings.Contains(s, "deliberately very long stderr line")
	}, defaultTimeout)
	if !strings.Contains(content, "wrapping and scrolling") {
		t.Fatalf("details should show wrapped stderr text\nCapture:\n%s", content)
	}

	sendKeys(t, sess, "NPage")
	time.Sleep(100 * time.Millisecond)
	sendKeys(t, sess, "End")
	waitForContent(t, sess, func(s string) bool {
		return strings.Contains(s, "Error details") && strings.Contains(s, "ERR 140")
	}, defaultTimeout)

	sendKeys(t, sess, "Escape")
	waitForContent(t, sess, func(s string) bool {
		return strings.Contains(s, "picker source failed") && !strings.Contains(s, "Error details")
	}, defaultTimeout)
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

// A literal display-popup e2e is not viable here: the detached test server has
// no attached client and capture-pane cannot see popup overlays. Running cmdk
// as the pane command has the same lifetime semantics as display-popup -E, so
// error-screen persistence in the pane plus the post-dismissal exit code is
// the machine-checkable equivalent of "the popup no longer flashes shut".
func TestE2E_LaunchPathCmdFailureShowsErrorScreenAndExitsClean(t *testing.T) {
	home := t.TempDir()
	marker := filepath.Join(t.TempDir(), "exitcode")
	xdg := writeConfig(t, `
[[actions]]
name = "xq-bad-launch"
matches = "root"
cmd = "true"
launch_path_cmd = "sh -c 'echo out; echo err >&2; exit 23'"
`)

	sess := "cmdk-test-" + strings.ReplaceAll(t.Name(), "/", "-")
	shellCmd := fmt.Sprintf("(%s --pane-id=%%0); echo EXITCODE=$? > '%s'", binaryPath, marker)
	cmd := tmuxCmd("new-session", "-d", "-s", sess, "-x", "120", "-y", "40",
		"env", "XDG_CONFIG_HOME="+xdg, "HOME="+home, "sh", "-c", shellCmd)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("tmux new-session failed: %v\n%s", err, out)
	}
	defer killSession(t, sess)

	waitForReady(t, sess)
	filterAndExecute(t, sess, "xq-bad-launch")

	waitForContent(t, sess, func(s string) bool {
		return strings.Contains(s, "Error details") &&
			strings.Contains(s, "Source: launch") &&
			strings.Contains(s, "launch_path_cmd failed: exit status 23") &&
			strings.Contains(s, "Exit code: 23") &&
			strings.Contains(s, "echo out; echo err >&2; exit 23") &&
			strings.Contains(s, "\n out") &&
			strings.Contains(s, "\n err")
	}, defaultTimeout)

	if !sessionExists(sess) {
		t.Fatal("session exited while the error screen should be showing")
	}

	sendKeys(t, sess, "Escape")
	waitForContent(t, sess, func(s string) bool {
		return strings.Contains(s, "xq-bad-launch") && !strings.Contains(s, "Error details")
	}, defaultTimeout)

	exitFilterModeE2E(t, sess)
	sendKeys(t, sess, "Escape")
	waitForExit(t, sess)

	got, err := os.ReadFile(marker)
	if err != nil {
		t.Fatalf("failed to read exitcode marker: %v", err)
	}
	if !strings.Contains(string(got), "EXITCODE=0") {
		t.Errorf("expected EXITCODE=0 after dismissing a launch failure, got: %s", got)
	}

	logPath := filepath.Join(home, ".local", "state", "cmdk", "cmdk.log")
	logContent := waitForFile(t, logPath, defaultTimeout)
	for _, want := range []string{
		"launch resolution failed",
		"exit_code=23",
		"echo out; echo err",
		"err",
	} {
		if !strings.Contains(logContent, want) {
			t.Errorf("log should contain %q, got:\n%s", want, logContent)
		}
	}
}

func TestE2E_DisplayPopup(t *testing.T) {
	sess := "cmdk-test-" + strings.ReplaceAll(t.Name(), "/", "-")
	shellCmd := fmt.Sprintf("%s --pane-id=$(tmux -L %s -f /dev/null display-message -p '#{pane_id}')", binaryPath, tmuxSocket)
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
launch_mode = "shell"
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
launch_mode = "shell"
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

func TestE2E_ShortenTruncateFlag(t *testing.T) {
	xdg := writeConfig(t, "")
	cmd := exec.Command(binaryPath, "shorten", "--truncate", "2", "/usr/local/bin/foo")
	cmd.Env = append(os.Environ(), "XDG_CONFIG_HOME="+xdg)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("shorten failed: %v\n%s", err, out)
	}
	got := strings.TrimSpace(string(out))
	if got != "bin/foo" {
		t.Errorf("got %q, want %q", got, "bin/foo")
	}
}

func TestE2E_ShortenTruncateWithSymbolFlag(t *testing.T) {
	xdg := writeConfig(t, "")
	cmd := exec.Command(binaryPath, "shorten", "--truncate", "2", "--truncate-symbol", "…", "/usr/local/bin/foo")
	cmd.Env = append(os.Environ(), "XDG_CONFIG_HOME="+xdg)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("shorten failed: %v\n%s", err, out)
	}
	got := strings.TrimSpace(string(out))
	if got != "…/bin/foo" {
		t.Errorf("got %q, want %q", got, "…/bin/foo")
	}
}

func TestE2E_ShortenTruncateFlagOverridesConfig(t *testing.T) {
	xdg := writeConfig(t, `
[display]
truncation_length = 3
truncation_symbol = "…"
`)
	cmd := exec.Command(binaryPath, "shorten", "--truncate", "1", "/a/b/c/d")
	cmd.Env = append(os.Environ(), "XDG_CONFIG_HOME="+xdg)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("shorten failed: %v\n%s", err, out)
	}
	got := strings.TrimSpace(string(out))
	if got != "…/d" {
		t.Errorf("got %q, want %q (flag overrides length, config symbol falls through)", got, "…/d")
	}
}

func TestE2E_ShortenNoTruncateFlag(t *testing.T) {
	xdg := writeConfig(t, "")
	cmd := exec.Command(binaryPath, "shorten", "/usr/local/bin/foo")
	cmd.Env = append(os.Environ(), "XDG_CONFIG_HOME="+xdg)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("shorten failed: %v\n%s", err, out)
	}
	got := strings.TrimSpace(string(out))
	if got != "/usr/local/bin/foo" {
		t.Errorf("got %q, want full path unchanged", got)
	}
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

func TestE2E_StartInFilterFalse_BrowseMode(t *testing.T) {
	xdg := writeConfig(t, `
[behavior]
start_in_filter = false
`)
	sess := startSessionWithEnv(t, map[string]string{"XDG_CONFIG_HOME": xdg})
	defer killSession(t, sess)

	waitForReady(t, sess)

	// In browse mode, the status bar shows "/ filter" hint instead of "esc cancel".
	waitForContent(t, sess, func(s string) bool {
		return strings.Contains(s, "/ filter")
	}, defaultTimeout)

	// Pressing / should enter filter mode.
	sendKeys(t, sess, "/")
	waitForContent(t, sess, func(s string) bool {
		return strings.Contains(s, "esc cancel")
	}, defaultTimeout)
}
