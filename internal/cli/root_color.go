package cli

import (
	"errors"
	"io"
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
func resolveColorProfile(mode colorMode, stdout io.Writer, env []string) colorprofile.Profile {
	switch mode {
	case colorNever:
		return colorprofile.ASCII
	case colorAlways:
		forced := slices.DeleteFunc(slices.Clone(env), func(kv string) bool {
			return strings.HasPrefix(kv, "NO_COLOR=")
		})
		forced = append(forced, "CLICOLOR_FORCE=1")
		if profile := colorprofile.Detect(stdout, forced); profile >= colorprofile.ANSI {
			return profile
		}
		return colorprofile.ANSI
	default:
		// colorprofile parses NO_COLOR as a boolean, so NO_COLOR=anything
		// would keep color; the contract is that any nonempty value disables
		// it.
		if slices.ContainsFunc(env, func(kv string) bool {
			return strings.HasPrefix(kv, "NO_COLOR=") && kv != "NO_COLOR="
		}) {
			return colorprofile.ASCII
		}
		return colorprofile.Detect(stdout, env)
	}
}

// rendersColor reports whether the profile can render color, which is the
// only case in which asking the terminal for its background can change what
// the person sees.
func rendersColor(profile colorprofile.Profile) bool {
	return profile >= colorprofile.ANSI
}
