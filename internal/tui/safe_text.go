package tui

import "github.com/jmcampanini/cmdk/internal/safetext"

func escapeTerminalControls(s string) string {
	return safetext.EscapeTerminalControls(s)
}
