package cli

import (
	"errors"
	"slices"
	"strings"

	"github.com/charmbracelet/colorprofile"

	"github.com/jmcampanini/cmdk/internal/theme"
)

// colorMode is the --color value: a fixed set, so pflag rejects any other
// value by name while parsing and the invocation fails as a usage error.
type colorMode string

const (
	colorAuto   colorMode = "auto"
	colorAlways colorMode = "always"
	colorNever  colorMode = "never"
)

func (m *colorMode) String() string {
	return string(*m)
}

func (m *colorMode) Set(value string) error {
	switch colorMode(value) {
	case colorAuto, colorAlways, colorNever:
		*m = colorMode(value)
		return nil
	default:
		return errors.New("must be auto, always, or never")
	}
}

func (*colorMode) Type() string {
	return "string"
}

// themeMode is the --theme value: empty for detection, or one of the two
// built-in palettes, so pflag rejects any other value by name while parsing.
type themeMode string

func (m *themeMode) String() string {
	return string(*m)
}

func (m *themeMode) Set(value string) error {
	switch value {
	case "", theme.NameLight, theme.NameDark:
		*m = themeMode(value)
		return nil
	default:
		return errors.New("must be light or dark")
	}
}

func (*themeMode) Type() string {
	return "string"
}

// resolveColorProfile applies the --color precedence for the stream the
// launcher draws on. auto is detection for that stream, in which a nonempty
// NO_COLOR, TERM=dumb, or a non-terminal disables color. never forces the
// no-color profile. always overrides detection and NO_COLOR, falling back to
// ANSI when the environment still reports no color.
func resolveColorProfile(mode colorMode, isTerminal bool, env []string) colorprofile.Profile {
	if mode == colorNever {
		return colorprofile.ASCII
	}

	var termType, noColor string
	for _, entry := range env {
		key, value, _ := strings.Cut(entry, "=")
		switch key {
		case "TERM":
			termType = value
		case "NO_COLOR":
			noColor = value
		}
	}
	if mode == colorAuto && (!isTerminal || termType == "dumb" || noColor != "") {
		return colorprofile.ASCII
	}

	if mode == colorAlways {
		env = append(slices.Clone(env), "NO_COLOR=", "CLICOLOR_FORCE=1")
	}
	// Detect also runs an unbounded "tmux info" subprocess. Env and Terminfo
	// inspect local capabilities without letting color detection hang startup.
	profile := max(colorprofile.Env(env), colorprofile.Terminfo(termType))
	if mode == colorAlways {
		return max(profile, colorprofile.ANSI)
	}
	return profile
}

// rendersColor reports whether the profile can render color, which is the
// only case in which asking the terminal for its background can change what
// the person sees.
func rendersColor(profile colorprofile.Profile) bool {
	return profile >= colorprofile.ANSI
}
