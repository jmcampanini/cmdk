package tmux

import "strings"

const (
	tmuxFieldSeparator = "\t"
	tmuxEscapedNewline = "↵"
	tmuxEscapedTab     = "⇥"

	// tmux escapes control characters in format output (for example, tabs as
	// `\t`) and doubles literal backslashes. Keep the raw fields in the tmux
	// format and decode them in Go so literal `\t`/`\n` names stay distinct
	// from actual tab/newline characters.
	tmuxEscapedSessionNameFormat = "#{session_name}"
	tmuxEscapedWindowNameFormat  = "#{window_name}"
)

func tmuxFormatFields(fields ...string) string {
	return strings.Join(fields, tmuxFieldSeparator)
}

func displaySafeTmuxText(s string) string {
	var b strings.Builder
	b.Grow(len(s))

	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '\t':
			b.WriteString(tmuxEscapedTab)
		case '\n':
			b.WriteString(tmuxEscapedNewline)
		case '\\':
			if i+1 >= len(s) {
				b.WriteByte('\\')
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
		default:
			b.WriteByte(s[i])
		}
	}

	return b.String()
}

func cleanTmuxLine(line string) string {
	return strings.TrimRight(line, "\r")
}

func splitTmuxFields(line string, expected int) ([]string, bool) {
	fields := strings.Split(cleanTmuxLine(line), tmuxFieldSeparator)
	if len(fields) != expected {
		return nil, false
	}
	return fields, true
}
