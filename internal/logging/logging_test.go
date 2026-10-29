package logging

import (
	log "log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSetup_CreatesDirAndFile(t *testing.T) {
	prev := log.Default()
	defer log.SetDefault(prev)

	dir := filepath.Join(t.TempDir(), "state", "cmdk")
	binDir := t.TempDir()
	marker := filepath.Join(binDir, "tmux-called")
	if err := os.WriteFile(filepath.Join(binDir, "tmux"), []byte("#!/bin/sh\n: > \"$CMDK_LOGGING_TMUX_MARKER\"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", binDir)
	t.Setenv("CMDK_LOGGING_TMUX_MARKER", marker)
	t.Setenv("TERM", "tmux-256color")
	t.Setenv("TMUX", "socket,1,0")
	t.Setenv("TTY_FORCE", "1")
	t.Setenv("CLICOLOR_FORCE", "1")
	t.Setenv("NO_COLOR", "")

	logger, err := SetupWithDir(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	logFile := filepath.Join(dir, "cmdk.log")
	if _, err := os.Stat(logFile); os.IsNotExist(err) {
		t.Fatal("log file was not created")
	}

	log.Debug("debug message")
	log.Info("test message", "key", "value")

	content, err := os.ReadFile(logFile)
	if err != nil {
		t.Fatalf("failed to read log file: %v", err)
	}
	if !strings.Contains(string(content), "test message") {
		t.Errorf("log file does not contain expected message, got: %s", content)
	}
	if !strings.Contains(string(content), "level=INFO") || !strings.Contains(string(content), "key=value") {
		t.Errorf("log file lacks the message level or field, got: %s", content)
	}
	if strings.Contains(string(content), "debug message") {
		t.Errorf("log file contains a message below the default Info level: %s", content)
	}
	if strings.ContainsRune(string(content), '\x1b') {
		t.Errorf("log file contains terminal escapes: %q", content)
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Errorf("tmux invocation marker stat = %v, want no invocation", err)
	}

	if err := logger.Close(); err != nil {
		t.Errorf("Close() error: %v", err)
	}
}

func TestSetup_DefaultLoggerUsesCurrentHome(t *testing.T) {
	prev := log.Default()
	defer log.SetDefault(prev)

	home := t.TempDir()
	t.Setenv("HOME", home)
	dir := filepath.Join(home, ".local", "state", "cmdk")

	logger, err := Setup()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer func() { _ = logger.Close() }()

	log.Warn("default logger warning", "key", "value")

	logFile := filepath.Join(dir, "cmdk.log")
	content, err := os.ReadFile(logFile)
	if err != nil {
		t.Fatalf("failed to read log file: %v", err)
	}
	if !strings.Contains(string(content), "default logger warning") {
		t.Errorf("package-level log.Warn did not write to file, got: %s", content)
	}
}
