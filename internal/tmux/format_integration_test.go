package tmux

import (
	"fmt"
	"os"
	"os/exec"
	"testing"
	"time"
)

func requireTmuxBinary(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("tmux"); err != nil {
		t.Skip("tmux not available")
	}
}

func runIsolatedTmux(t *testing.T, socket string, args ...string) string {
	t.Helper()
	out, err := tryIsolatedTmux(socket, args...)
	if err != nil {
		t.Fatalf("tmux %v failed: %v\n%s", args, err, out)
	}
	return out
}

func tryIsolatedTmux(socket string, args ...string) (string, error) {
	cmdArgs := append([]string{"-L", socket, "-f", "/dev/null"}, args...)
	out, err := exec.Command("tmux", cmdArgs...).CombinedOutput()
	return string(out), err
}

func isolatedTmuxSocket(t *testing.T) string {
	t.Helper()
	requireTmuxBinary(t)
	socket := fmt.Sprintf("cmdk-tmux-test-%d-%d", os.Getpid(), time.Now().UnixNano())
	t.Cleanup(func() {
		_ = exec.Command("tmux", "-L", socket, "-f", "/dev/null", "kill-server").Run()
	})
	return socket
}

func TestTmuxWindowFormatEscapesActualControlCharacters(t *testing.T) {
	socket := isolatedTmuxSocket(t)
	if out, err := tryIsolatedTmux(socket, "new-session", "-d", "-s", "plain", "-n", "win\tname\nnext", "sleep", "1000"); err != nil {
		t.Skipf("tmux does not allow control characters in window names: %v\n%s", err, out)
	}

	out := runIsolatedTmux(t, socket, "list-windows", "-a", "-F", tmuxListWindowsFormat)
	items, err := ParseWindows(out)
	if err != nil {
		t.Fatalf("ParseWindows(%q) returned error: %v", out, err)
	}
	if len(items) != 1 {
		t.Fatalf("got %d items, want 1", len(items))
	}

	want := "tmux:win: 0 win" + tmuxEscapedTab + "name" + tmuxEscapedNewline + "next ‹ plain"
	if items[0].Display != want {
		t.Errorf("Display = %q, want %q", items[0].Display, want)
	}
}

func TestTmuxWindowFormatPreservesLiteralBackslashSequences(t *testing.T) {
	socket := isolatedTmuxSocket(t)
	runIsolatedTmux(t, socket, "new-session", "-d", "-s", "plain", "-n", `win\tname\nnext`, "sleep", "1000")

	out := runIsolatedTmux(t, socket, "list-windows", "-a", "-F", tmuxListWindowsFormat)
	items, err := ParseWindows(out)
	if err != nil {
		t.Fatalf("ParseWindows(%q) returned error: %v", out, err)
	}
	if len(items) != 1 {
		t.Fatalf("got %d items, want 1", len(items))
	}

	want := `tmux:win: 0 win\tname\nnext ‹ plain`
	if items[0].Display != want {
		t.Errorf("Display = %q, want %q", items[0].Display, want)
	}
}

func TestTmuxSessionFormatEscapesActualControlCharacters(t *testing.T) {
	socket := isolatedTmuxSocket(t)
	runIsolatedTmux(t, socket, "new-session", "-d", "-s", "plain", "sleep", "1000")
	if out, err := tryIsolatedTmux(socket, "rename-session", "sess\tname\nnext"); err != nil {
		t.Skipf("tmux does not allow control characters in session names: %v\n%s", err, out)
	}

	out := runIsolatedTmux(t, socket, "list-sessions", "-F", sessionListFormat)
	items, err := ParseSessions(out)
	if err != nil {
		t.Fatalf("ParseSessions(%q) returned error: %v", out, err)
	}
	if len(items) != 1 {
		t.Fatalf("got %d items, want 1", len(items))
	}

	want := "tmux:ses: sess" + tmuxEscapedTab + "name" + tmuxEscapedNewline + "next"
	if items[0].Display != want {
		t.Errorf("Display = %q, want %q", items[0].Display, want)
	}
}

func TestTmuxSessionFormatPreservesLiteralBackslashSequences(t *testing.T) {
	socket := isolatedTmuxSocket(t)
	runIsolatedTmux(t, socket, "new-session", "-d", "-s", "plain", "sleep", "1000")
	runIsolatedTmux(t, socket, "rename-session", `sess\tname\nnext`)

	out := runIsolatedTmux(t, socket, "list-sessions", "-F", sessionListFormat)
	items, err := ParseSessions(out)
	if err != nil {
		t.Fatalf("ParseSessions(%q) returned error: %v", out, err)
	}
	if len(items) != 1 {
		t.Fatalf("got %d items, want 1", len(items))
	}

	want := `tmux:ses: sess\tname\nnext`
	if items[0].Display != want {
		t.Errorf("Display = %q, want %q", items[0].Display, want)
	}
}
