package tmux

import (
	"strings"
	"unicode"
)

const (
	tmuxFieldSeparator = "\t"
	tmuxEscapedNewline = "↵"
	tmuxEscapedTab     = "⇥"
)

// tmux output is not consistent about control characters in names: window
// names are emitted with actual tabs/newlines, while session names are emitted
// with tmux-style backslash escapes. Substitute actual controls before
// tab-delimiting, then use field-specific display cleanup below.
var (
	tmuxEscapedSessionNameFormat = tmuxEscapedFormat("session_name")
	tmuxEscapedWindowNameFormat  = tmuxEscapedFormat("window_name")
)

func tmuxEscapedFormat(name string) string {
	return "#{s|\t|" + tmuxEscapedTab + "|:#{s|\n|" + tmuxEscapedNewline + "|:#{" + name + "}}}"
}

func tmuxFormatFields(fields ...string) string {
	return strings.Join(fields, tmuxFieldSeparator)
}

// displaySafeTmuxWindowName returns a TUI-safe tmux window name. tmux emits
// window_name with actual control bytes, so tmuxEscapedFormat handles tabs and
// newlines before field splitting; this function is only defensive cleanup for
// any controls that remain and deliberately preserves literal backslash text
// such as `\t` and `\n`.
func displaySafeTmuxWindowName(s string) string {
	return displaySafeTmuxControls(s)
}

// displaySafeTmuxSessionName returns a TUI-safe tmux session name. tmux emits
// session_name control characters as backslash escapes and doubles literal
// backslashes, unlike window_name, so session fields must decode tmux's escape
// layer before applying the same display cleanup. The result is for display and
// template metadata only; tmux commands should target session_id.
func displaySafeTmuxSessionName(s string) string {
	return displaySafeTmuxControls(decodeTmuxSessionNameEscapes(s))
}

func decodeTmuxSessionNameEscapes(s string) string {
	var b strings.Builder
	b.Grow(len(s))

	for i := 0; i < len(s); i++ {
		if s[i] != '\\' || i+1 >= len(s) {
			b.WriteByte(s[i])
			continue
		}

		next := s[i+1]
		switch next {
		case 't':
			b.WriteString(tmuxEscapedTab)
		case 'n':
			b.WriteString(tmuxEscapedNewline)
		case '\\':
			b.WriteByte('\\')
		default:
			b.WriteByte('\\')
			b.WriteByte(next)
		}
		i++
	}

	return b.String()
}

func displaySafeTmuxControls(s string) string {
	s = strings.ToValidUTF8(s, "_")

	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		switch r {
		case '\t':
			b.WriteString(tmuxEscapedTab)
		case '\n', '\r':
			b.WriteString(tmuxEscapedNewline)
		default:
			if unicode.IsControl(r) {
				b.WriteByte('_')
			} else {
				b.WriteRune(r)
			}
		}
	}
	return b.String()
}

func cleanTmuxLine(line string) string {
	return strings.TrimRight(line, "\r")
}

func tmuxLines(output string) []string {
	rawLines := strings.Split(output, "\n")
	lines := make([]string, 0, len(rawLines))
	for _, line := range rawLines {
		line = cleanTmuxLine(line)
		if line != "" {
			lines = append(lines, line)
		}
	}
	return lines
}

func splitTmuxFields(line string, expected int) ([]string, bool) {
	fields := strings.Split(cleanTmuxLine(line), tmuxFieldSeparator)
	if len(fields) != expected {
		return nil, false
	}
	return fields, true
}
