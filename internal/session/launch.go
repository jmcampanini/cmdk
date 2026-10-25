package session

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// ValidateLaunchDirectory returns the absolute path of an existing directory.
func ValidateLaunchDirectory(path string) (string, error) {
	if path == "" {
		return "", errors.New("path is required")
	}
	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("resolving absolute path: %w", err)
	}
	absPath = filepath.Clean(absPath)
	info, err := os.Stat(absPath)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return "", fmt.Errorf("path does not exist: %s", absPath)
		}
		return "", fmt.Errorf("path is not accessible: %w", err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("path is not a directory: %s", absPath)
	}
	return absPath, nil
}

// DefaultWindowNameForLaunchPath returns the directory basename for a window.
func DefaultWindowNameForLaunchPath(path string) string {
	return filepath.Base(filepath.Clean(path))
}
