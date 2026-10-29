package cli

import (
	"os"
	"path/filepath"
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
	terminal := []string{"TERM=xterm-256color"}
	tests := []struct {
		name       string
		mode       colorMode
		isTerminal bool
		env        []string
		wantColor  bool
	}{
		{name: "auto on a pipe", mode: colorAuto, env: []string{"TERM=xterm-256color"}, wantColor: false},
		{name: "auto ignores forced TTY on a pipe", mode: colorAuto, env: append(terminal, "TTY_FORCE=1"), wantColor: false},
		{name: "auto ignores forced color on a pipe", mode: colorAuto, env: append(terminal, "CLICOLOR_FORCE=1"), wantColor: false},
		{name: "auto on a terminal", mode: colorAuto, isTerminal: true, env: terminal, wantColor: true},
		{name: "auto honors NO_COLOR", mode: colorAuto, isTerminal: true, env: append(terminal, "NO_COLOR=1"), wantColor: false},
		{name: "auto honors any nonempty NO_COLOR", mode: colorAuto, isTerminal: true, env: append(terminal, "NO_COLOR=anything"), wantColor: false},
		{name: "auto ignores empty NO_COLOR", mode: colorAuto, isTerminal: true, env: append(terminal, "NO_COLOR="), wantColor: true},
		{name: "auto honors TERM=dumb", mode: colorAuto, isTerminal: true, env: []string{"TERM=dumb"}, wantColor: false},
		{name: "auto ignores forced color on dumb terminal", mode: colorAuto, isTerminal: true, env: []string{"TERM=dumb", "CLICOLOR_FORCE=1"}, wantColor: false},
		{name: "never on a terminal", mode: colorNever, isTerminal: true, env: terminal, wantColor: false},
		{name: "always overrides NO_COLOR", mode: colorAlways, isTerminal: true, env: append(terminal, "NO_COLOR=1"), wantColor: true},
		{name: "always on a dumb pipe", mode: colorAlways, env: []string{"TERM=dumb", "NO_COLOR=1"}, wantColor: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			profile := resolveColorProfile(tt.mode, tt.isTerminal, tt.env)
			if got := rendersColor(profile); got != tt.wantColor {
				t.Errorf("profile = %s, rendersColor = %t, want %t", profile, got, tt.wantColor)
			}
		})
	}
	if profile := resolveColorProfile(colorNever, true, terminal); profile != colorprofile.ASCII {
		t.Errorf("never profile = %s, want Ascii", profile)
	}
}

func TestResolveColorProfileDoesNotRunTmux(t *testing.T) {
	dir := t.TempDir()
	marker := filepath.Join(dir, "tmux-called")
	if err := os.WriteFile(filepath.Join(dir, "tmux"), []byte("#!/bin/sh\n: > \"$CMDK_COLOR_PROBE_MARKER\"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)
	t.Setenv("CMDK_COLOR_PROBE_MARKER", marker)

	profile := resolveColorProfile(colorAuto, true, []string{"TERM=tmux-256color", "TMUX=socket,1,0"})

	if profile < colorprofile.ANSI256 {
		t.Errorf("profile = %s, want at least ANSI256 from TERM", profile)
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Errorf("tmux invocation marker stat = %v, want no invocation", err)
	}
}
