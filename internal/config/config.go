package config

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"github.com/BurntSushi/toml"
)

const defaultFetchTimeout = 2 * time.Second

type Command struct {
	Name string `toml:"name"`
	Cmd  string `toml:"cmd"`
}

type Timeout struct {
	FetchMs int `toml:"fetch_ms"`
}

type Config struct {
	Commands []Command `toml:"commands"`
	Timeout  Timeout   `toml:"timeout"`
}

func Load(path string) (*Config, error) {
	var cfg Config
	_, err := toml.DecodeFile(path, &cfg)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return &Config{}, nil
		}
		return &Config{}, err
	}
	return &cfg, nil
}

func (c *Config) FetchTimeout() time.Duration {
	if c.Timeout.FetchMs <= 0 {
		return defaultFetchTimeout
	}
	return time.Duration(c.Timeout.FetchMs) * time.Millisecond
}

func DefaultPath() string {
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return filepath.Join(xdg, "cmdk", "config.toml")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "cmdk", "config.toml")
}
