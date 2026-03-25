package config

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"time"

	log "charm.land/log/v2"
	"github.com/BurntSushi/toml"

	"github.com/jmcampanini/cmdk/internal/icon"
)

type StageConfig struct {
	Type    string `toml:"type"`
	Key     string `toml:"key"`
	Text    string `toml:"text"`
	Default string `toml:"default"`
	Source  string `toml:"source"`
}

type Action struct {
	Name    string        `toml:"name"`
	Matches string        `toml:"matches"`
	Cmd     string        `toml:"cmd"`
	Icon    string        `toml:"icon"`
	Stages  []StageConfig `toml:"stages"`
}

type Behavior struct {
	AutoSelectSingle *bool `toml:"auto_select_single"`
}

func (b Behavior) ShouldAutoSelectSingle() bool {
	if b.AutoSelectSingle == nil {
		return true
	}
	return *b.AutoSelectSingle
}

type Timeout struct {
	Fetch  time.Duration `toml:"fetch"`
	Picker time.Duration `toml:"picker"`
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
	Actions  []Action                `toml:"actions"`
	Behavior Behavior                `toml:"behavior"`
	Timeout  Timeout                 `toml:"timeout"`
	Sources  map[string]SourceConfig `toml:"sources"`
	Display  Display                 `toml:"display"`
}

var validMatchTypes = []string{"root", "dir"}

var reservedKeys = []string{"path", "pane_id", "session", "window_index"}

var validStageKey = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*$`)

func DefaultConfig() Config {
	defaultShortenHome := "~"
	return Config{
		Timeout: Timeout{Fetch: 2 * time.Second, Picker: 2 * time.Second},
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
	if c.Timeout.Picker < 0 {
		return errors.New("timeout.picker cannot be negative")
	}
	if c.Timeout.Picker > 0 && c.Timeout.Picker < time.Millisecond {
		return fmt.Errorf("timeout.picker value %s is suspiciously small; use a duration string like \"2s\"", c.Timeout.Picker)
	}
	for name, sc := range c.Sources {
		if sc.Limit < 0 {
			return fmt.Errorf("sources.%s.limit cannot be negative", name)
		}
		if sc.MinScore < 0 {
			return fmt.Errorf("sources.%s.min_score cannot be negative", name)
		}
	}
	if err := validateActions(c.Actions); err != nil {
		return err
	}
	for match := range c.Display.Rules {
		if match == "" {
			return errors.New("display.rules: match key cannot be empty")
		}
	}
	return nil
}

func validateActions(actions []Action) error {
	for i, a := range actions {
		if a.Name == "" {
			return fmt.Errorf("actions[%d].name cannot be empty", i)
		}
		if a.Cmd == "" {
			return fmt.Errorf("actions[%d].cmd cannot be empty", i)
		}
		if a.Matches == "" {
			return fmt.Errorf("actions[%d].matches cannot be empty", i)
		}
		if !slices.Contains(validMatchTypes, a.Matches) {
			return fmt.Errorf("actions[%d].matches %q is not a valid match type (valid: %v)", i, a.Matches, validMatchTypes)
		}
		if a.Icon != "" {
			if _, err := icon.Resolve(a.Icon); err != nil {
				return fmt.Errorf("actions[%d].icon: %w", i, err)
			}
		}
		if err := validateStages(i, a.Stages); err != nil {
			return err
		}
	}
	return nil
}

func validateStages(actionIdx int, stages []StageConfig) error {
	seenKeys := make(map[string]bool, len(stages))
	for j, s := range stages {
		prefix := fmt.Sprintf("actions[%d].stages[%d]", actionIdx, j)
		if s.Key == "" {
			return fmt.Errorf("%s.key cannot be empty", prefix)
		}
		if !validStageKey.MatchString(s.Key) {
			return fmt.Errorf("%s.key %q must be a valid identifier (letters, digits, underscores; cannot start with a digit)", prefix, s.Key)
		}
		if seenKeys[s.Key] {
			return fmt.Errorf("%s.key %q is duplicate within this action", prefix, s.Key)
		}
		seenKeys[s.Key] = true
		if slices.Contains(reservedKeys, s.Key) {
			return fmt.Errorf("%s.key %q is reserved (reserved keys: %v)", prefix, s.Key, reservedKeys)
		}
		switch s.Type {
		case "prompt":
			if s.Text == "" {
				return fmt.Errorf("%s.text cannot be empty for prompt stage", prefix)
			}
			if s.Source != "" {
				return fmt.Errorf("%s.source must not be set for prompt stage", prefix)
			}
		case "picker":
			if s.Source == "" {
				return fmt.Errorf("%s.source cannot be empty for picker stage", prefix)
			}
			if s.Text != "" {
				return fmt.Errorf("%s.text must not be set for picker stage", prefix)
			}
			if s.Default != "" {
				return fmt.Errorf("%s.default must not be set for picker stage", prefix)
			}
		default:
			return fmt.Errorf("%s.type %q is not valid (valid: prompt, picker)", prefix, s.Type)
		}
	}
	return nil
}

func (c *Config) resolveIcons() error {
	for i := range c.Actions {
		if c.Actions[i].Icon != "" {
			resolved, err := icon.Resolve(c.Actions[i].Icon)
			if err != nil {
				return fmt.Errorf("action %q icon: %w", c.Actions[i].Name, err)
			}
			c.Actions[i].Icon = resolved
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
