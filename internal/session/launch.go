package session

import (
	"errors"
	"path/filepath"
)

// ValidateLaunchDirectory returns the absolute path of an existing directory.
func ValidateLaunchDirectory(path string) (string, error) {
	if path == "" {
		return "", errors.New("path is required")
	}
	return resolveExistingDirectory(path)
}

// DefaultWindowNameForLaunchPath returns the directory basename for a window.
func DefaultWindowNameForLaunchPath(path string) string {
	return filepath.Base(filepath.Clean(path))
}
