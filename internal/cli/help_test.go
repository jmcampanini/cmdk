package cli

import (
	"path/filepath"
	"testing"
)

func TestHelpCommandShowsTheSelectedCommandWithoutLoadingConfig(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "missing.toml")
	for _, command := range [][]string{nil, {"session", "window"}, {"launching"}, {"help"}} {
		args := append([]string{"--config", missing, "help"}, command...)
		got := executeRootForTest(t, args...)
		want := executeRootForTest(t, append(command, "--help")...)
		if got != want {
			t.Errorf("help %v = %q, want %q", command, got, want)
		}
	}
}
