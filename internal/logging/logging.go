package logging

import (
	"io"
	"os"
	"path/filepath"

	"charm.land/log/v2"
)

var defaultDir = filepath.Join(os.Getenv("HOME"), ".local", "state", "cmdk")

type Logger struct {
	*log.Logger
	closer io.Closer
}

func (l *Logger) Close() error {
	return l.closer.Close()
}

func Setup() (*Logger, error) {
	return SetupWithDir(defaultDir)
}

func SetupWithDir(dir string) (*Logger, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}

	f, err := os.OpenFile(filepath.Join(dir, "cmdk.log"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return nil, err
	}

	l := log.New(f)
	l.SetReportTimestamp(true)
	return &Logger{Logger: l, closer: f}, nil
}
