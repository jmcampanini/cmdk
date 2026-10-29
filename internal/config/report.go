package config

import (
	"fmt"
	"io"
	"maps"
	"slices"
	"strings"
)

// WriteDerivedValues writes resolved action settings and icon-bearing values as
// TOML comments, so the complete configuration report remains reloadable.
func (c Config) WriteDerivedValues(out io.Writer) error {
	var report strings.Builder
	fmt.Fprintln(&report, "\n# Derived values (resolved action settings and icon aliases)")
	for i, action := range c.Actions {
		fmt.Fprintf(&report, "# actions[%d].launch_mode = %q\n", i, action.LaunchMode)
		if action.LaunchMode == LaunchModeSessionWindow {
			fmt.Fprintf(&report, "# actions[%d].window_name = %q\n", i, action.WindowName)
		}
		if action.Icon != "" {
			fmt.Fprintf(&report, "# actions[%d].icon = %q\n", i, action.Icon)
		}
		for j, stage := range action.Stages {
			if stage.Type == "picker" {
				fmt.Fprintf(&report, "# actions[%d].stages[%d].delimiter = %q\n", i, j, stage.Delimiter)
			}
		}
	}
	fmt.Fprintf(&report, "# display.shorten_home = %q\n", c.Display.ShortenHome)
	fmt.Fprintf(&report, "# display.truncation_symbol = %q\n", c.Display.TruncationSymbol)
	fmt.Fprintf(&report, "# display.tmux_session_truncation_symbol = %q\n", c.Display.TmuxSessionTruncationSymbol)
	for _, match := range slices.Sorted(maps.Keys(c.Display.Rules)) {
		fmt.Fprintf(&report, "# display.rules[%q] = %q\n", match, c.Display.Rules[match])
	}
	_, err := io.WriteString(out, report.String())
	return err
}
