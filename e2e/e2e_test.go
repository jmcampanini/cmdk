package e2e

import (
	"fmt"
	"os"
	"os/exec"
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
	sess := "cmdk-test-" + strings.ReplaceAll(t.Name(), "/", "-")
	cmd := exec.Command("tmux", "new-session", "-d", "-s", sess, "-x", "120", "-y", "40",
		binaryPath, "--pane-id=%0")
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
