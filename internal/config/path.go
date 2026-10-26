package config

import (
	"errors"
	"os"
	"path/filepath"

	log "charm.land/log/v2"
)

// ResolvePath returns the configuration file a command loads. A supplied path
// is required: it must name an existing regular file, and an empty value is an
// error. Without a supplied path the file is discovered with DefaultPath and
// may be absent.
func ResolvePath(supplied string, isSupplied bool) (string, error) {
	if !isSupplied {
		return DefaultPath(), nil
	}
	if supplied == "" {
		return "", errors.New("config path cannot be empty")
	}
	if err := validateRequiredRegularFile(supplied); err != nil {
		return "", err
	}
	return supplied, nil
}

// DefaultPath returns $XDG_CONFIG_HOME/cmdk/config.toml, or
// ~/.config/cmdk/config.toml when XDG_CONFIG_HOME is unset, empty, or not an
// absolute path, which the XDG Base Directory specification treats as invalid.
func DefaultPath() string {
	if xdg := os.Getenv("XDG_CONFIG_HOME"); filepath.IsAbs(xdg) {
		return filepath.Join(xdg, "cmdk", "config.toml")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		log.Warn("could not determine home directory for config path", "error", err)
	}
	return filepath.Join(home, ".config", "cmdk", "config.toml")
}
