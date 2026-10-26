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
		overrides[mode] = theme.ModeOverrides{
			Tokens: theme.Tokens{
				Accent:     themeColor(m.Accent),
				AccentText: themeColor(m.AccentText),
				Cursor:     themeColor(m.Cursor),
				Text:       themeColor(m.Text),
				Muted:      themeColor(m.Muted),
				Subtle:     themeColor(m.Subtle),
				SelectedBg: themeColor(m.SelectedBg),
				InputBg:    themeColor(m.InputBg),
				MatchBg:    themeColor(m.MatchBg),
				Info:       themeColor(m.Info),
				Success:    themeColor(m.Success),
				Secondary:  themeColor(m.Secondary),
				Warning:    themeColor(m.Warning),
				Error:      themeColor(m.Error),
			},
			Roles: theme.Roles{
				WindowIcon:  themeColor(m.Roles.WindowIcon),
				DirIcon:     themeColor(m.Roles.DirIcon),
				ActionIcon:  themeColor(m.Roles.ActionIcon),
				SessionIcon: themeColor(m.Roles.SessionIcon),
				UnknownIcon: themeColor(m.Roles.UnknownIcon),
				LoadingIcon: themeColor(m.Roles.LoadingIcon),
				BellIcon:    themeColor(m.Roles.BellIcon),
				ErrorIcon:   themeColor(m.Roles.ErrorIcon),
			},
		}
	}
	return overrides
}

func themeColor(hex string) color.Color {
	if hex == "" {
		return nil
	}
	return theme.Color(hex)
}

func (c ThemeConfig) validate() error {
	for mode, m := range c {
		switch mode {
		case theme.NameLight, theme.NameDark:
		default:
			return fmt.Errorf("theme.%s is not valid (valid: light, dark)", mode)
		}
		for _, field := range m.colorFields("theme." + mode) {
			if field.value != "" && !hexColor.MatchString(field.value) {
				return fmt.Errorf("%s must be a #RRGGBB hex color", field.path)
			}
		}
	}
	return nil
}

type themeColorPath struct {
	path  string
	value string
}

func (m ThemeModeConfig) colorFields(prefix string) []themeColorPath {
	return []themeColorPath{
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
