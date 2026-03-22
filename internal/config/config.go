package config

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	log "charm.land/log/v2"
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
	Limit    int     `toml:"limit"`
	MinScore float64 `toml:"min_score"`
}

type Display struct {
	ShortenHome *string           `toml:"shorten_home"`
	Rules       map[string]string `toml:"rules"`
}

type Config struct {
	Commands    []Command               `toml:"commands"`
	DirCommands []Command               `toml:"dir_commands"`
	Timeout     Timeout                 `toml:"timeout"`
	Sources     map[string]SourceConfig `toml:"sources"`
	Display     Display                 `toml:"display"`
}

func DefaultConfig() Config {
	defaultShortenHome := "~"
	return Config{
		Timeout: Timeout{Fetch: 2 * time.Second},
		Sources: map[string]SourceConfig{"zoxide": {Limit: 0}},
		Display: Display{ShortenHome: &defaultShortenHome},
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
		if sc.MinScore < 0 {
			return fmt.Errorf("sources.%s.min_score cannot be negative", name)
		}
	}
	if err := validateCommands("commands", c.Commands); err != nil {
		return err
	}
	if err := validateCommands("dir_commands", c.DirCommands); err != nil {
		return err
	}
	for match := range c.Display.Rules {
		if match == "" {
			return errors.New("display.rules: match key cannot be empty")
		}
	}
	return nil
}

func validateCommands(section string, cmds []Command) error {
	for i, cmd := range cmds {
		if cmd.Name == "" {
			return fmt.Errorf("%s[%d].name cannot be empty", section, i)
		}
		if cmd.Cmd == "" {
			return fmt.Errorf("%s[%d].cmd cannot be empty", section, i)
		}
	}
	return nil
}

// Load always returns a valid *Config, even when err is non-nil (defaults are used as fallback).
func Load(path string) (*Config, error) {
	cfg := DefaultConfig()

	_, err := toml.DecodeFile(path, &cfg)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return newDefaultConfig(), nil
		}
		return newDefaultConfig(), err
	}

	defaults := DefaultConfig()

	// Backfill default source entries that the TOML file didn't mention.
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
	home, err := os.UserHomeDir()
	if err != nil {
		log.Warn("could not determine home directory for config path", "error", err)
	}
	return filepath.Join(home, ".config", "cmdk", "config.toml")
}
