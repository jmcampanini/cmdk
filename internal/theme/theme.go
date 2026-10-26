// Package theme resolves launcher colors and applies per-mode overrides.
package theme

import (
	"fmt"
	"image/color"

	"charm.land/lipgloss/v2"
)

const (
	// NameLight selects the Catppuccin Latte base palette.
	NameLight = "light"
	// NameDark selects the Catppuccin Frappe base palette.
	NameDark = "dark"
)

// Overrides replaces built-in colors per appearance mode. Keys are NameLight
// and NameDark; overrides for other keys are ignored.
type Overrides map[string]ModeOverrides

// ModeOverrides lists the colors that replace one built-in mode's palette.
// A nil color keeps the built-in value; roles derive from the overridden
// tokens unless set explicitly.
type ModeOverrides struct {
	Roles  Roles
	Tokens Tokens
}

// Theme contains the resolved appearance mode, semantic colors, and icon roles.
type Theme struct {
	Name   string
	IsDark bool

	Tokens Tokens
	Roles  Roles
}

// Tokens assigns colors to semantic launcher elements and status meanings.
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

// Roles assigns colors to launcher item and status icons.
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

// Color returns the terminal color for a #RRGGBB hex value.
func Color(hex string) color.Color { return lipgloss.Color(hex) }

// Default returns the dark theme with any supplied overrides applied in order.
func Default(overrides ...Overrides) Theme {
	return Dark(overrides...)
}

// FromBackground selects the appearance matching the detected terminal background.
func FromBackground(isDark bool, overrides ...Overrides) Theme {
	if isDark {
		return Dark(overrides...)
	}
	return Light(overrides...)
}

// Light returns the Catppuccin Latte palette with supplied overrides applied in order.
func Light(overrides ...Overrides) Theme {
	return applyOverrides(Theme{
		Name:   NameLight,
		IsDark: false,
		Tokens: Tokens{
			Accent:     Color("#8839ef"), // Latte mauve
			AccentText: Color("#eff1f5"), // Latte base
			Cursor:     Color("#7287fd"), // Latte lavender
			Text:       Color("#4c4f69"), // Latte text
			Muted:      Color("#8c8fa1"), // Latte overlay1
			Subtle:     Color("#acb0be"), // Latte surface2
			SelectedBg: Color("#bcc0cc"), // Latte surface1
			InputBg:    Color("#dce0e8"), // Latte crust
			MatchBg:    Color("#e0c8f8"), // mauve-tinted match highlight
			Info:       Color("#1e66f5"), // Latte blue
			Success:    Color("#40a02b"), // Latte green
			Secondary:  Color("#179299"), // Latte teal
			Warning:    Color("#df8e1d"), // Latte yellow
			Error:      Color("#d20f39"), // Latte red
		},
	}, overrides...)
}

// Dark returns the Catppuccin Frappe palette with supplied overrides applied in order.
func Dark(overrides ...Overrides) Theme {
	return applyOverrides(Theme{
		Name:   NameDark,
		IsDark: true,
		Tokens: Tokens{
			Accent:     Color("#ca9ee6"), // Frappe mauve
			AccentText: Color("#303446"), // Frappe base
			Cursor:     Color("#babbf1"), // Frappe lavender
			Text:       Color("#c6d0f5"), // Frappe text
			Muted:      Color("#838ba7"), // Frappe overlay1
			Subtle:     Color("#626880"), // Frappe surface2
			SelectedBg: Color("#51576d"), // Frappe surface1
			InputBg:    Color("#414559"), // Frappe surface0
			MatchBg:    Color("#5b4b8a"), // mauve-tinted match highlight
			Info:       Color("#8caaee"), // Frappe blue
			Success:    Color("#a6d189"), // Frappe green
			Secondary:  Color("#81c8be"), // Frappe teal
			Warning:    Color("#e5c890"), // Frappe yellow
			Error:      Color("#e78284"), // Frappe red
		},
	}, overrides...)
}

// Resolve selects light or dark by name, using the default for an empty name.
func Resolve(name string, overrides ...Overrides) (Theme, error) {
	switch name {
	case "":
		return Default(overrides...), nil
	case NameLight:
		return Light(overrides...), nil
	case NameDark:
		return Dark(overrides...), nil
	default:
		return Theme{}, fmt.Errorf("unknown theme %q (valid: light, dark)", name)
	}
}

func applyOverrides(t Theme, overrides ...Overrides) Theme {
	t.Roles = deriveRoles(t.Tokens)
	for _, o := range overrides {
		if mode, ok := o[t.Name]; ok {
			t = mode.Apply(t)
		}
	}
	return t
}

// Apply replaces the theme's tokens with the set overrides, re-derives roles
// from the resulting tokens, and then applies explicit role overrides.
func (m ModeOverrides) Apply(t Theme) Theme {
	override(&t.Tokens.Accent, m.Tokens.Accent)
	override(&t.Tokens.AccentText, m.Tokens.AccentText)
	override(&t.Tokens.Cursor, m.Tokens.Cursor)
	override(&t.Tokens.Text, m.Tokens.Text)
	override(&t.Tokens.Muted, m.Tokens.Muted)
	override(&t.Tokens.Subtle, m.Tokens.Subtle)
	override(&t.Tokens.SelectedBg, m.Tokens.SelectedBg)
	override(&t.Tokens.InputBg, m.Tokens.InputBg)
	override(&t.Tokens.MatchBg, m.Tokens.MatchBg)
	override(&t.Tokens.Info, m.Tokens.Info)
	override(&t.Tokens.Success, m.Tokens.Success)
	override(&t.Tokens.Secondary, m.Tokens.Secondary)
	override(&t.Tokens.Warning, m.Tokens.Warning)
	override(&t.Tokens.Error, m.Tokens.Error)

	t.Roles = deriveRoles(t.Tokens)
	override(&t.Roles.WindowIcon, m.Roles.WindowIcon)
	override(&t.Roles.DirIcon, m.Roles.DirIcon)
	override(&t.Roles.ActionIcon, m.Roles.ActionIcon)
	override(&t.Roles.SessionIcon, m.Roles.SessionIcon)
	override(&t.Roles.UnknownIcon, m.Roles.UnknownIcon)
	override(&t.Roles.LoadingIcon, m.Roles.LoadingIcon)
	override(&t.Roles.BellIcon, m.Roles.BellIcon)
	override(&t.Roles.ErrorIcon, m.Roles.ErrorIcon)
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

func override(dst *color.Color, value color.Color) {
	if value == nil {
		return
	}
	*dst = value
}
