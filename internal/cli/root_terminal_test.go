package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func nonTerminalFile(t *testing.T) *os.File {
	t.Helper()
	f, err := os.Create(filepath.Join(t.TempDir(), "stream"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = f.Close() })
	return f
}

func TestCheckLauncherTerminalRejectsNonTerminalStdout(t *testing.T) {
	err := checkLauncherTerminal(nonTerminalFile(t), nonTerminalFile(t))
	if err == nil || !strings.Contains(err.Error(), "stdout is not a terminal") || !strings.Contains(err.Error(), "tmux popup") {
		t.Fatalf("error = %v, want stdout guidance", err)
	}
}

func TestCheckCursorControl(t *testing.T) {
	for termType, wantErr := range map[string]string{
		"":               "TERM is not set",
		"dumb":           "TERM is dumb",
		"tmux-256color":  "",
		"xterm-256color": "",
	} {
		err := checkCursorControl(termType)
		if wantErr == "" {
			if err != nil {
				t.Errorf("checkCursorControl(%q) = %v, want nil", termType, err)
			}
			continue
		}
		if err == nil || !strings.Contains(err.Error(), wantErr) || !strings.Contains(err.Error(), "set TERM") {
			t.Errorf("checkCursorControl(%q) = %v, want %q with guidance", termType, err, wantErr)
		}
	}
}
