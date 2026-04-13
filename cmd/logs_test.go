package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func tailOutput(t *testing.T, path string, n int) string {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer func() { _ = f.Close() }()

	data, err := readTail(f, n)
	if err != nil {
		t.Fatalf("readTail: %v", err)
	}
	return string(data)
}

func writeLines(t *testing.T, dir string, lines ...string) string {
	t.Helper()
	path := filepath.Join(dir, "test.log")
	content := strings.Join(lines, "\n") + "\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	return path
}

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
	_, err := os.Open(path)
	if !os.IsNotExist(err) {
		t.Fatalf("expected not-exist error, got %v", err)
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
