package cli

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestCompletionWritesTheSelectedScriptWithoutLoadingConfig(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "missing.toml")
	for _, shell := range []struct {
		name      string
		scriptKey string
	}{
		{name: "bash", scriptKey: "__cmdk"},
		{name: "zsh", scriptKey: "#compdef cmdk"},
		{name: "fish", scriptKey: "complete -c cmdk"},
		{name: "powershell", scriptKey: "Register-ArgumentCompleter"},
	} {
		t.Run(shell.name, func(t *testing.T) {
			var withDescriptions string
			for _, noDescriptions := range []bool{false, true} {
				args := []string{"--config", missing, "completion", shell.name}
				if noDescriptions {
					args = append(args, "--no-descriptions")
				}
				script := executeRootForTest(t, args...)
				if !strings.Contains(script, shell.scriptKey) {
					t.Errorf("completion %s contains no %q", shell.name, shell.scriptKey)
				}
				if !noDescriptions {
					withDescriptions = script
				} else if script == withDescriptions {
					t.Error("--no-descriptions did not change the generated script")
				}
			}
		})
	}
}
