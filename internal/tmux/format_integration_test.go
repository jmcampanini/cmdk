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
	cmdArgs := append([]string{"-L", socket, "-f", "/dev/null"}, args...)
	out, err := exec.Command("tmux", cmdArgs...).CombinedOutput()
	if err != nil {
		t.Fatalf("tmux %v failed: %v\n%s", args, err, out)
	}
	return string(out)
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
	runIsolatedTmux(t, socket, "new-session", "-d", "-s", "plain", "-n", "win\tname\nnext", "sleep", "1000")

	out := runIsolatedTmux(t, socket, "list-windows", "-a", "-F", tmuxListWindowsFormat)
	items, err := ParseWindows(out)
	if err != nil {
		t.Fatalf("ParseWindows(%q) returned error: %v", out, err)
	}
	if len(items) != 1 {
		t.Fatalf("got %d items, want 1", len(items))
	}

	want := "tmux: plain:0 win" + tmuxEscapedTab + "name" + tmuxEscapedNewline + "next"
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

	want := `tmux: plain:0 win\tname\nnext`
	if items[0].Display != want {
		t.Errorf("Display = %q, want %q", items[0].Display, want)
	}
}

func TestTmuxSessionFormatEscapesActualControlCharacters(t *testing.T) {
	socket := isolatedTmuxSocket(t)
	runIsolatedTmux(t, socket, "new-session", "-d", "-s", "plain", "sleep", "1000")
	runIsolatedTmux(t, socket, "rename-session", "sess\tname\nnext")

	out := runIsolatedTmux(t, socket, "list-sessions", "-F", sessionListFormat)
	items, err := ParseSessions(out)
	if err != nil {
		t.Fatalf("ParseSessions(%q) returned error: %v", out, err)
	}
	if len(items) != 1 {
		t.Fatalf("got %d items, want 1", len(items))
	}

	want := "tmux: sess" + tmuxEscapedTab + "name" + tmuxEscapedNewline + "next"
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

	want := `tmux: sess\tname\nnext`
	if items[0].Display != want {
		t.Errorf("Display = %q, want %q", items[0].Display, want)
	}
}
