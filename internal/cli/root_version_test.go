package cli

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
)

func TestVersionPrintsBuildIdentityWithoutPrerequisites(t *testing.T) {
	stubTmuxPrerequisite(t, func(context.Context) error {
		return errors.New("version must not check tmux")
	})
	missing := filepath.Join(t.TempDir(), "missing.toml")
	for _, args := range [][]string{
		{"--version", "--config", missing},
		{"--config", missing, "--version"},
		{"-v"},
	} {
		if got := executeRootForTest(t, args...); got != "cmdk version "+Version+"\n" {
			t.Errorf("cmdk %v = %q, want build version", args, got)
		}
	}
}
