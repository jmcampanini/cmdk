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
