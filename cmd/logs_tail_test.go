package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestTail_BasicLastN(t *testing.T) {
	path := writeLines(t, t.TempDir(), "a", "b", "c", "d", "e", "f", "g", "h", "i", "j")
	got := tailOutput(t, path, 3)
	want := "h\ni\nj\n"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestTail_FewerThanN(t *testing.T) {
	path := writeLines(t, t.TempDir(), "a", "b", "c")
	got := tailOutput(t, path, 10)
	want := "a\nb\nc\n"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestTail_EmptyFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.log")
	if err := os.WriteFile(path, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	got := tailOutput(t, path, 5)
	if got != "" {
		t.Errorf("expected empty output for empty file, got %q", got)
	}
}

func TestTail_NonExistentFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nope.log")
	f, err := os.Open(path)
	if err == nil {
		_ = f.Close()
		t.Fatal("expected error for non-existent file")
	}
	if !os.IsNotExist(err) {
		t.Fatalf("expected not-exist error, got %v", err)
	}
}

func TestLogsTail_ValidationErrors(t *testing.T) {
	tests := []struct {
		name    string
		n       int
		wantErr string
	}{
		{"zero", 0, "line count must be positive"},
		{"negative", -5, "line count must be positive"},
		{"over max", 10001, "at most 10000"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := runLogsTailCommand(logsTailOptions{lines: tt.n})
			if err == nil {
				t.Fatal("expected error")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("error %q does not contain %q", err.Error(), tt.wantErr)
			}
		})
	}
}

func TestTail_TrailingNewline(t *testing.T) {
	path := writeLines(t, t.TempDir(), "first", "second")
	got := tailOutput(t, path, 2)
	want := "first\nsecond\n"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestTail_NoTrailingNewline(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.log")
	if err := os.WriteFile(path, []byte("a\nb\nc"), 0o644); err != nil {
		t.Fatal(err)
	}
	got := tailOutput(t, path, 2)
	want := "b\nc"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}
