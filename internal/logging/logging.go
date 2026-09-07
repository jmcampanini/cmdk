// Package logging directs cmdk diagnostics to its persistent state-directory log.
package logging

import (
	"io"
	"os"
	"path/filepath"

	"charm.land/log/v2"
)

var defaultDir = filepath.Join(os.Getenv("HOME"), ".local", "state", "cmdk")

const logFileName = "cmdk.log"

// Logger owns the open file used by the process-wide structured logger.
type Logger struct {
	closer io.Closer
}

// Close releases the log file opened during setup.
func (l *Logger) Close() error {
	return l.closer.Close()
}

// Setup configures the process-wide logger under the user's local state directory.
func Setup() (*Logger, error) {
	return SetupWithDir(defaultDir)
}

// SetupWithDir appends logs to cmdk.log in dir and installs the process-wide logger.
func SetupWithDir(dir string) (*Logger, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}

	f, err := os.OpenFile(filepath.Join(dir, logFileName), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return nil, err
	}

	l := log.New(f)
	l.SetReportTimestamp(true)
	log.SetDefault(l)
	return &Logger{closer: f}, nil
}
