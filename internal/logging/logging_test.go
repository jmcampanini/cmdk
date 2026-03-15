package logging

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSetup_CreatesDirAndFile(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "state", "cmdk")

	logger, err := SetupWithDir(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	logFile := filepath.Join(dir, "cmdk.log")
	if _, err := os.Stat(logFile); os.IsNotExist(err) {
		t.Fatal("log file was not created")
	}

	logger.Info("test message", "key", "value")

	content, err := os.ReadFile(logFile)
	if err != nil {
		t.Fatalf("failed to read log file: %v", err)
	}
	if !strings.Contains(string(content), "test message") {
		t.Errorf("log file does not contain expected message, got: %s", content)
	}

	if err := logger.Close(); err != nil {
		t.Errorf("Close() error: %v", err)
	}
}
