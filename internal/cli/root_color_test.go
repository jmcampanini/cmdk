package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/charmbracelet/colorprofile"
)

func TestColorModeAcceptsOnlyTheFixedSet(t *testing.T) {
	var mode colorMode
	for _, value := range []string{"auto", "always", "never"} {
		if err := mode.Set(value); err != nil || string(mode) != value {
			t.Errorf("Set(%q) = %v, mode %q", value, err, mode)
		}
	}
	err := mode.Set("sometimes")
	if err == nil || !strings.Contains(err.Error(), "auto, always, or never") {
		t.Fatalf("Set(\"sometimes\") = %v, want the valid values named", err)
	}
}

func TestResolveColorProfile(t *testing.T) {
	// A bytes.Buffer is not a terminal, so auto detection reports NoTTY
	// unless TTY_FORCE makes colorprofile treat the stream as one.
	var pipe bytes.Buffer
	terminal := []string{"TTY_FORCE=1", "TERM=xterm-256color"}
	tests := []struct {
		name      string
		mode      colorMode
		env       []string
		wantColor bool
	}{
		{name: "auto on a pipe", mode: colorAuto, env: []string{"TERM=xterm-256color"}, wantColor: false},
		{name: "auto on a terminal", mode: colorAuto, env: terminal, wantColor: true},
		{name: "auto honors NO_COLOR", mode: colorAuto, env: append(terminal, "NO_COLOR=1"), wantColor: false},
		{name: "auto honors any nonempty NO_COLOR", mode: colorAuto, env: append(terminal, "NO_COLOR=anything"), wantColor: false},
		{name: "auto ignores empty NO_COLOR", mode: colorAuto, env: append(terminal, "NO_COLOR="), wantColor: true},
		{name: "auto honors TERM=dumb", mode: colorAuto, env: []string{"TTY_FORCE=1", "TERM=dumb"}, wantColor: false},
		{name: "never on a terminal", mode: colorNever, env: terminal, wantColor: false},
		{name: "always overrides NO_COLOR", mode: colorAlways, env: append(terminal, "NO_COLOR=1"), wantColor: true},
		{name: "always on a dumb pipe", mode: colorAlways, env: []string{"TERM=dumb", "NO_COLOR=1"}, wantColor: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			profile := resolveColorProfile(tt.mode, &pipe, tt.env)
			if got := rendersColor(profile); got != tt.wantColor {
				t.Errorf("profile = %s, rendersColor = %t, want %t", profile, got, tt.wantColor)
			}
		})
	}
	if profile := resolveColorProfile(colorNever, &pipe, terminal); profile != colorprofile.ASCII {
		t.Errorf("never profile = %s, want Ascii", profile)
	}
}
