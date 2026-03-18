package config

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"github.com/BurntSushi/toml"
)

type Command struct {
	Name string `toml:"name"`
	Cmd  string `toml:"cmd"`
}

type Timeout struct {
	Fetch time.Duration `toml:"fetch"`
}

type SourceConfig struct {
	Limit int `toml:"limit"`
}

type Config struct {
	Commands []Command               `toml:"commands"`
	Timeout  Timeout                 `toml:"timeout"`
	Sources  map[string]SourceConfig `toml:"sources"`
}

func DefaultConfig() Config {
	return Config{
		Timeout: Timeout{Fetch: 2 * time.Second},
		Sources: map[string]SourceConfig{"zoxide": {Limit: 20}},
	}
}

func (c Config) Validate() error {
	if c.Timeout.Fetch < 0 {
		return errors.New("timeout.fetch cannot be negative")
	}
	if c.Timeout.Fetch > 0 && c.Timeout.Fetch < time.Millisecond {
		return fmt.Errorf("timeout.fetch value %s is suspiciously small; use a duration string like \"2s\"", c.Timeout.Fetch)
	}
	for name, sc := range c.Sources {
		if sc.Limit < 0 {
			return fmt.Errorf("sources.%s.limit cannot be negative", name)
		}
	}
	for i, cmd := range c.Commands {
		if cmd.Name == "" {
			return fmt.Errorf("commands[%d].name cannot be empty", i)
		}
		if cmd.Cmd == "" {
			return fmt.Errorf("commands[%d].cmd cannot be empty", i)
		}
	}
	return nil
}

func Load(path string) (*Config, error) {
	cfg := DefaultConfig()

	_, err := toml.DecodeFile(path, &cfg)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return newDefaultConfig(), nil
		}
		return newDefaultConfig(), err
	}

	// Backfill default source entries that the TOML file didn't mention.
	defaults := DefaultConfig()
	for name, sc := range defaults.Sources {
		if _, ok := cfg.Sources[name]; !ok {
			cfg.Sources[name] = sc
		}
	}

	if err := cfg.Validate(); err != nil {
		return newDefaultConfig(), err
	}
	return &cfg, nil
}

func newDefaultConfig() *Config {
	cfg := DefaultConfig()
	return &cfg
}

func DefaultPath() string {
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return filepath.Join(xdg, "cmdk", "config.toml")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "cmdk", "config.toml")
}
