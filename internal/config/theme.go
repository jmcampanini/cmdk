package config

import (
	"fmt"
	"image/color"
	"regexp"

	"github.com/jmcampanini/cmdk/internal/theme"
)

// ThemeConfig holds per-mode color overrides keyed by appearance mode, as
// written under [theme.light] and [theme.dark].
type ThemeConfig map[string]ThemeModeConfig

// ThemeModeConfig lists the semantic token and role colors a mode overrides.
// Every field is an optional #RRGGBB hex color.
type ThemeModeConfig struct {
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

	Roles ThemeRoleConfig `toml:"roles,omitempty"`
}

// ThemeRoleConfig lists explicit icon colors that replace the ones derived
// from the semantic tokens.
type ThemeRoleConfig struct {
	WindowIcon  string `toml:"window_icon,omitempty"`
	DirIcon     string `toml:"dir_icon,omitempty"`
	ActionIcon  string `toml:"action_icon,omitempty"`
	SessionIcon string `toml:"session_icon,omitempty"`
	UnknownIcon string `toml:"unknown_icon,omitempty"`
	LoadingIcon string `toml:"loading_icon,omitempty"`
	BellIcon    string `toml:"bell_icon,omitempty"`
	ErrorIcon   string `toml:"error_icon,omitempty"`
}

var hexColor = regexp.MustCompile(`^#[0-9A-Fa-f]{6}$`)

// Overrides converts the validated overrides into the theme colors they select.
func (c ThemeConfig) Overrides() theme.Overrides {
	overrides := make(theme.Overrides, len(c))
	for mode, m := range c {
		var o theme.ModeOverrides
		for _, field := range m.colorFields(&o) {
			if field.value != "" {
				*field.dst = theme.Color(field.value)
			}
		}
		overrides[mode] = o
	}
	return overrides
}

func (c ThemeConfig) validate() error {
	for mode, m := range c {
		switch mode {
		case theme.NameLight, theme.NameDark:
		default:
			return fmt.Errorf("theme.%s is not valid (valid: light, dark)", mode)
		}
		var unused theme.ModeOverrides
		for _, field := range m.colorFields(&unused) {
			if field.value != "" && !hexColor.MatchString(field.value) {
				return fmt.Errorf("theme.%s.%s must be a #RRGGBB hex color", mode, field.key)
			}
		}
	}
	return nil
}

// themeColorOverride ties one configuration key to its value and to the theme
// color it overrides, so validation and conversion walk the same list.
type themeColorOverride struct {
	key   string
	value string
	dst   *color.Color
}

func (m ThemeModeConfig) colorFields(o *theme.ModeOverrides) []themeColorOverride {
	return []themeColorOverride{
		{"accent", m.Accent, &o.Tokens.Accent},
		{"accent_text", m.AccentText, &o.Tokens.AccentText},
		{"cursor", m.Cursor, &o.Tokens.Cursor},
		{"text", m.Text, &o.Tokens.Text},
		{"muted", m.Muted, &o.Tokens.Muted},
		{"subtle", m.Subtle, &o.Tokens.Subtle},
		{"selected_bg", m.SelectedBg, &o.Tokens.SelectedBg},
		{"input_bg", m.InputBg, &o.Tokens.InputBg},
		{"match_bg", m.MatchBg, &o.Tokens.MatchBg},
		{"info", m.Info, &o.Tokens.Info},
		{"success", m.Success, &o.Tokens.Success},
		{"secondary", m.Secondary, &o.Tokens.Secondary},
		{"warning", m.Warning, &o.Tokens.Warning},
		{"error", m.Error, &o.Tokens.Error},
		{"roles.window_icon", m.Roles.WindowIcon, &o.Roles.WindowIcon},
		{"roles.dir_icon", m.Roles.DirIcon, &o.Roles.DirIcon},
		{"roles.action_icon", m.Roles.ActionIcon, &o.Roles.ActionIcon},
		{"roles.session_icon", m.Roles.SessionIcon, &o.Roles.SessionIcon},
		{"roles.unknown_icon", m.Roles.UnknownIcon, &o.Roles.UnknownIcon},
		{"roles.loading_icon", m.Roles.LoadingIcon, &o.Roles.LoadingIcon},
		{"roles.bell_icon", m.Roles.BellIcon, &o.Roles.BellIcon},
		{"roles.error_icon", m.Roles.ErrorIcon, &o.Roles.ErrorIcon},
	}
}
