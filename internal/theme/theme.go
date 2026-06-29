package theme

import (
	"fmt"
	"image/color"
	"regexp"

	"charm.land/lipgloss/v2"
)

const (
	NameLight = "light"
	NameDark  = "dark"
)

// Config contains per-mode theme overrides loaded from config TOML.
// Supported keys are "light" and "dark", corresponding to [theme.light]
// and [theme.dark]. Overrides are applied on top of the active built-in theme.
type Config map[string]ModeConfig

// ModeConfig exposes compact semantic theme tokens for one built-in mode.
type ModeConfig struct {
	Accent     string `toml:"accent,omitempty"`
	AccentText string `toml:"accent_text,omitempty"`
	Cursor     string `toml:"cursor,omitempty"`
	Text       string `toml:"text,omitempty"`
	Muted      string `toml:"muted,omitempty"`
	Subtle     string `toml:"subtle,omitempty"`
	SelectedBg string `toml:"selected_bg,omitempty"`
	InputBg    string `toml:"input_bg,omitempty"`
	MatchBg    string `toml:"match_bg,omitempty"`
	Info       string `toml:"info,omitempty"`
	Success    string `toml:"success,omitempty"`
	Secondary  string `toml:"secondary,omitempty"`
	Warning    string `toml:"warning,omitempty"`
	Error      string `toml:"error,omitempty"`

	Roles RoleConfig `toml:"roles,omitempty"`
}

// RoleConfig contains optional visible-role overrides derived from semantic tokens.
type RoleConfig struct {
	WindowIcon  string `toml:"window_icon,omitempty"`
	DirIcon     string `toml:"dir_icon,omitempty"`
	ActionIcon  string `toml:"action_icon,omitempty"`
	SessionIcon string `toml:"session_icon,omitempty"`
	UnknownIcon string `toml:"unknown_icon,omitempty"`
	LoadingIcon string `toml:"loading_icon,omitempty"`
	BellIcon    string `toml:"bell_icon,omitempty"`
	ErrorIcon   string `toml:"error_icon,omitempty"`
}

type Theme struct {
	Name   string
	IsDark bool

	Tokens Tokens
	Roles  Roles
}

type Tokens struct {
	Accent     color.Color
	AccentText color.Color
	Cursor     color.Color
	Text       color.Color
	Muted      color.Color
	Subtle     color.Color
	SelectedBg color.Color
	InputBg    color.Color
	MatchBg    color.Color
	Info       color.Color
	Success    color.Color
	Secondary  color.Color
	Warning    color.Color
	Error      color.Color
}

type Roles struct {
	WindowIcon  color.Color
	DirIcon     color.Color
	ActionIcon  color.Color
	SessionIcon color.Color
	UnknownIcon color.Color
	LoadingIcon color.Color
	BellIcon    color.Color
	ErrorIcon   color.Color
}

var hexColor = regexp.MustCompile(`^#[0-9A-Fa-f]{6}$`)

func c(hex string) color.Color { return lipgloss.Color(hex) }

func Default(configs ...Config) Theme {
	return Dark(configs...)
}

func FromBackground(isDark bool, configs ...Config) Theme {
	if isDark {
		return Dark(configs...)
	}
	return Light(configs...)
}

func Light(configs ...Config) Theme {
	return applyConfigs(Theme{
		Name:   NameLight,
		IsDark: false,
		Tokens: Tokens{
			Accent:     c("#8839ef"), // Latte mauve
			AccentText: c("#eff1f5"), // Latte base
			Cursor:     c("#7287fd"), // Latte lavender
			Text:       c("#4c4f69"), // Latte text
			Muted:      c("#8c8fa1"), // Latte overlay1
			Subtle:     c("#acb0be"), // Latte surface2
			SelectedBg: c("#bcc0cc"), // Latte surface1
			InputBg:    c("#dce0e8"), // Latte crust
			MatchBg:    c("#e0c8f8"), // mauve-tinted match highlight
			Info:       c("#1e66f5"), // Latte blue
			Success:    c("#40a02b"), // Latte green
			Secondary:  c("#179299"), // Latte teal
			Warning:    c("#df8e1d"), // Latte yellow
			Error:      c("#d20f39"), // Latte red
		},
	}, configs...)
}

func Dark(configs ...Config) Theme {
	return applyConfigs(Theme{
		Name:   NameDark,
		IsDark: true,
		Tokens: Tokens{
			Accent:     c("#ca9ee6"), // Frappe mauve
			AccentText: c("#303446"), // Frappe base
			Cursor:     c("#babbf1"), // Frappe lavender
			Text:       c("#c6d0f5"), // Frappe text
			Muted:      c("#838ba7"), // Frappe overlay1
			Subtle:     c("#626880"), // Frappe surface2
			SelectedBg: c("#51576d"), // Frappe surface1
			InputBg:    c("#414559"), // Frappe surface0
			MatchBg:    c("#5b4b8a"), // mauve-tinted match highlight
			Info:       c("#8caaee"), // Frappe blue
			Success:    c("#a6d189"), // Frappe green
			Secondary:  c("#81c8be"), // Frappe teal
			Warning:    c("#e5c890"), // Frappe yellow
			Error:      c("#e78284"), // Frappe red
		},
	}, configs...)
}

func Resolve(name string, configs ...Config) (Theme, error) {
	switch name {
	case "":
		return Default(configs...), nil
	case NameLight:
		return Light(configs...), nil
	case NameDark:
		return Dark(configs...), nil
	default:
		return Theme{}, fmt.Errorf("unknown theme %q (valid: light, dark)", name)
	}
}

func applyConfigs(t Theme, configs ...Config) Theme {
	t.Roles = deriveRoles(t.Tokens)
	for _, cfg := range configs {
		if cfg == nil {
			continue
		}
		if overrides, ok := cfg[t.Name]; ok {
			t = overrides.Apply(t)
		}
	}
	return t
}

// Apply applies semantic token overrides, re-derives roles from those tokens,
// and then applies explicit role overrides.
func (m ModeConfig) Apply(t Theme) Theme {
	setColor(&t.Tokens.Accent, m.Accent)
	setColor(&t.Tokens.AccentText, m.AccentText)
	setColor(&t.Tokens.Cursor, m.Cursor)
	setColor(&t.Tokens.Text, m.Text)
	setColor(&t.Tokens.Muted, m.Muted)
	setColor(&t.Tokens.Subtle, m.Subtle)
	setColor(&t.Tokens.SelectedBg, m.SelectedBg)
	setColor(&t.Tokens.InputBg, m.InputBg)
	setColor(&t.Tokens.MatchBg, m.MatchBg)
	setColor(&t.Tokens.Info, m.Info)
	setColor(&t.Tokens.Success, m.Success)
	setColor(&t.Tokens.Secondary, m.Secondary)
	setColor(&t.Tokens.Warning, m.Warning)
	setColor(&t.Tokens.Error, m.Error)

	t.Roles = deriveRoles(t.Tokens)
	setColor(&t.Roles.WindowIcon, m.Roles.WindowIcon)
	setColor(&t.Roles.DirIcon, m.Roles.DirIcon)
	setColor(&t.Roles.ActionIcon, m.Roles.ActionIcon)
	setColor(&t.Roles.SessionIcon, m.Roles.SessionIcon)
	setColor(&t.Roles.UnknownIcon, m.Roles.UnknownIcon)
	setColor(&t.Roles.LoadingIcon, m.Roles.LoadingIcon)
	setColor(&t.Roles.BellIcon, m.Roles.BellIcon)
	setColor(&t.Roles.ErrorIcon, m.Roles.ErrorIcon)
	return t
}

func deriveRoles(tokens Tokens) Roles {
	return Roles{
		WindowIcon:  tokens.Accent,
		DirIcon:     tokens.Info,
		ActionIcon:  tokens.Success,
		SessionIcon: tokens.Secondary,
		UnknownIcon: tokens.Muted,
		LoadingIcon: tokens.Muted,
		BellIcon:    tokens.Warning,
		ErrorIcon:   tokens.Error,
	}
}

func setColor(dst *color.Color, value string) {
	if value == "" {
		return
	}
	*dst = c(value)
}

func (cfg Config) Validate() error {
	for mode, overrides := range cfg {
		switch mode {
		case NameLight, NameDark:
		default:
			return fmt.Errorf("theme.%s is not valid (valid: light, dark)", mode)
		}
		if err := overrides.Validate("theme." + mode); err != nil {
			return err
		}
	}
	return nil
}

func (m ModeConfig) Validate(prefix string) error {
	for _, field := range m.colorFields(prefix) {
		if err := validateColor(field.path, field.value); err != nil {
			return err
		}
	}
	return nil
}

type colorField struct {
	path  string
	value string
}

func (m ModeConfig) colorFields(prefix string) []colorField {
	return []colorField{
		{prefix + ".accent", m.Accent},
		{prefix + ".accent_text", m.AccentText},
		{prefix + ".cursor", m.Cursor},
		{prefix + ".text", m.Text},
		{prefix + ".muted", m.Muted},
		{prefix + ".subtle", m.Subtle},
		{prefix + ".selected_bg", m.SelectedBg},
		{prefix + ".input_bg", m.InputBg},
		{prefix + ".match_bg", m.MatchBg},
		{prefix + ".info", m.Info},
		{prefix + ".success", m.Success},
		{prefix + ".secondary", m.Secondary},
		{prefix + ".warning", m.Warning},
		{prefix + ".error", m.Error},
		{prefix + ".roles.window_icon", m.Roles.WindowIcon},
		{prefix + ".roles.dir_icon", m.Roles.DirIcon},
		{prefix + ".roles.action_icon", m.Roles.ActionIcon},
		{prefix + ".roles.session_icon", m.Roles.SessionIcon},
		{prefix + ".roles.unknown_icon", m.Roles.UnknownIcon},
		{prefix + ".roles.loading_icon", m.Roles.LoadingIcon},
		{prefix + ".roles.bell_icon", m.Roles.BellIcon},
		{prefix + ".roles.error_icon", m.Roles.ErrorIcon},
	}
}

func validateColor(path string, value string) error {
	if value == "" {
		return nil
	}
	if !hexColor.MatchString(value) {
		return fmt.Errorf("%s must be a #RRGGBB hex color", path)
	}
	return nil
}
