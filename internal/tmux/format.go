package tmux

import "strings"

const (
	tmuxFieldSeparator = "\t"
	tmuxEscapedNewline = "↵"
	tmuxEscapedTab     = "⇥"
)

type tmuxTextReplacement struct {
	// outputPattern is the bytes tmux may emit in list-* output.
	outputPattern string
	// formatPattern is the escaped form tmux's #{s|...|...|:...} modifier expects.
	formatPattern string
	glyph         string
}

var tmuxTextReplacements = []tmuxTextReplacement{
	{outputPattern: "\t", formatPattern: "\t", glyph: tmuxEscapedTab},
	{outputPattern: "\n", formatPattern: "\n", glyph: tmuxEscapedNewline},
	{outputPattern: `\t`, formatPattern: `\\t`, glyph: tmuxEscapedTab},
	{outputPattern: `\n`, formatPattern: `\\n`, glyph: tmuxEscapedNewline},
}

var (
	tmuxEscapedSessionNameFormat = tmuxTextFormat("#{session_name}")
	tmuxEscapedWindowNameFormat  = tmuxTextFormat("#{window_name}")
)

func tmuxTextFormat(expr string) string {
	for _, repl := range tmuxTextReplacements {
		expr = tmuxSubstitute(repl.formatPattern, repl.glyph, expr)
	}
	return expr
}

func tmuxSubstitute(pattern, replacement, expr string) string {
	return "#{s|" + pattern + "|" + replacement + "|:" + expr + "}"
}

func tmuxFormatFields(fields ...string) string {
	return strings.Join(fields, tmuxFieldSeparator)
}

func displaySafeTmuxText(s string) string {
	for _, repl := range tmuxTextReplacements {
		s = strings.ReplaceAll(s, repl.outputPattern, repl.glyph)
	}
	return s
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
