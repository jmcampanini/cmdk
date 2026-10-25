package config

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
)

// ResolvePath validates an explicit config path or returns the default path.
func ResolvePath(configPath string) (string, error) {
	if configPath != "" {
		fi, err := os.Stat(configPath)
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				return "", fmt.Errorf("config file not found: %s", configPath)
			}
			return "", fmt.Errorf("config file not accessible: %w", err)
		}
		if !fi.Mode().IsRegular() {
			return "", fmt.Errorf("config path is not a regular file: %s", configPath)
		}
		return configPath, nil
	}
	return DefaultPath(), nil
}
