package cli

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestShortenAppliesConfigOverrides(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte("[display]\ntruncation_length = -1\ntruncation_symbol = ':nf-invalid:'\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	root := newRootCommand()
	var stdout bytes.Buffer
	root.SetOut(&stdout)
	root.SetArgs([]string{"shorten", "--config", path, "--truncate", "2", "--truncate-symbol", ":nf-cod-ellipsis:", "/usr/local/bin"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	if got, want := stdout.String(), "\uea7c/local/bin\n"; got != want {
		t.Errorf("shorten stdout = %q, want %q", got, want)
	}
}

func TestShortenReturnsOutputError(t *testing.T) {
	useTempConfigHome(t)
	root := newRootCommand()
	want := errors.New("output unavailable")
	root.SetOut(errorWriter{err: want})
	root.SetArgs([]string{"shorten", "/usr/local/bin"})

	if err := root.Execute(); !errors.Is(err, want) {
		t.Errorf("shorten error = %v, want %v", err, want)
	}
}
