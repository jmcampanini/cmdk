package theme

import (
	"fmt"
	"image/color"
	"os"

	"charm.land/lipgloss/v2"
)

type Theme struct {
	Name   string
	IsDark bool

	Accent    color.Color
	AccentDim color.Color

	Text     color.Color
	Subtext1 color.Color
	Subtext0 color.Color
	Overlay1 color.Color
	Overlay0 color.Color
	Surface2 color.Color
	Surface1 color.Color
	Surface0 color.Color
	Base     color.Color
	Mantle   color.Color
	Crust    color.Color

	TypeWindow color.Color
	TypeDir    color.Color
	TypeCmd    color.Color

	MatchHighlight color.Color
	TextboxBg      color.Color
}

func c(hex string) color.Color { return lipgloss.Color(hex) }

// TODO(#26): replace with async tea.RequestBackgroundColor to avoid
// blocking startup and silent dark fallback on detection failure.
var hasDarkBackground = func() bool {
	return lipgloss.HasDarkBackground(os.Stdin, os.Stdout)
}

func Light() Theme {
	return Theme{
		Name:      "light",
		IsDark:    false,
		Accent:    c("#8839ef"),
		AccentDim: c("#7287fd"),
		Text:      c("#4c4f69"),
		Subtext1:  c("#5c5f77"),
		Subtext0:  c("#6c6f85"),
		Overlay1:  c("#8c8fa1"),
		Overlay0:  c("#9ca0b0"),
		Surface2:  c("#acb0be"),
		Surface1:  c("#bcc0cc"),
		Surface0:  c("#ccd0da"),
		Base:      c("#eff1f5"),
		Mantle:    c("#e6e9ef"),
		Crust:     c("#dce0e8"),

		TypeWindow: c("#8839ef"),
		TypeDir:    c("#1e66f5"),
		TypeCmd:    c("#40a02b"),

		MatchHighlight: c("#e0c8f8"),
		TextboxBg:      c("#dce0e8"),
	}
}

func Dark() Theme {
	return Theme{
		Name:      "dark",
		IsDark:    true,
		Accent:    c("#cba6f7"),
		AccentDim: c("#b4befe"),
		Text:      c("#cdd6f4"),
		Subtext1:  c("#bac2de"),
		Subtext0:  c("#a6adc8"),
		Overlay1:  c("#7f849c"),
		Overlay0:  c("#6c7086"),
		Surface2:  c("#585b70"),
		Surface1:  c("#45475a"),
		Surface0:  c("#313244"),
		Base:      c("#1e1e2e"),
		Mantle:    c("#181825"),
		Crust:     c("#11111b"),

		TypeWindow: c("#cba6f7"),
		TypeDir:    c("#89b4fa"),
		TypeCmd:    c("#a6e3a1"),

		MatchHighlight: c("#5b3d8f"),
		TextboxBg:      c("#313244"),
	}
}

func Resolve(name string) (Theme, error) {
	switch name {
	case "":
		if hasDarkBackground() {
			return Dark(), nil
		}
		return Light(), nil
	case "light":
		return Light(), nil
	case "dark":
		return Dark(), nil
	default:
		return Theme{}, fmt.Errorf("unknown theme %q (valid: light, dark)", name)
	}
}
