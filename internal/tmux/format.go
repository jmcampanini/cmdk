package tmux

import "strings"

const (
	tmuxFieldSeparator = "\t"
	tmuxEscapedNewline = "↵"
	tmuxEscapedTab     = "⇥"
)

// tmux output is not consistent about control characters in names: some paths
// emit actual tabs/newlines while others emit backslash escapes. Substitute
// actual controls before tab-delimiting, then decode tmux-style backslash
// escapes in Go so literal `\t`/`\n` names stay distinct.
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
