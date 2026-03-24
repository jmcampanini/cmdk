package config

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	log "charm.land/log/v2"
	"github.com/BurntSushi/toml"

	"github.com/jmcampanini/cmdk/internal/icon"
	"github.com/jmcampanini/cmdk/internal/item"
)

type Command struct {
	Name   string `toml:"name"`
	Cmd    string `toml:"cmd"`
	Icon   string `toml:"icon"`
	Prompt string `toml:"prompt"`
}

func (c Command) HasPrompt() bool {
	return c.Prompt != ""
}

func (c Command) Action() item.ActionType {
	if c.HasPrompt() {
		return item.ActionTextInput
	}
	return item.ActionExecute
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
	DirActions  []Command               `toml:"dir_actions"`
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
	if err := validateCommands("dir_actions", c.DirActions); err != nil {
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
		if cmd.Icon != "" {
			if _, err := icon.Resolve(cmd.Icon); err != nil {
				return fmt.Errorf("%s[%d].icon: %w", section, i, err)
			}
		}
		if cmd.HasPrompt() && !strings.Contains(cmd.Cmd, ".prompt") && !strings.Contains(cmd.Cmd, "CMDK_PROMPT") {
			return fmt.Errorf("%s[%d]: prompt is set but cmd does not reference {{.prompt}} or CMDK_PROMPT", section, i)
		}
	}
	return nil
}

func (c *Config) resolveIcons() error {
	if err := resolveCommandIcons(c.Commands); err != nil {
		return fmt.Errorf("resolving command icons: %w", err)
	}
	if err := resolveCommandIcons(c.DirActions); err != nil {
		return fmt.Errorf("resolving dir_action icons: %w", err)
	}
	return nil
}

func resolveCommandIcons(cmds []Command) error {
	for i := range cmds {
		if cmds[i].Icon != "" {
			resolved, err := icon.Resolve(cmds[i].Icon)
			if err != nil {
				return fmt.Errorf("command %q icon: %w", cmds[i].Name, err)
			}
			cmds[i].Icon = resolved
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
	if err := cfg.resolveIcons(); err != nil {
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
