package tmux

import (
	"strings"
	"testing"
)

func TestTmuxNameFormatsSubstituteControlsBeforeDelimiting(t *testing.T) {
	formats := map[string]string{
		"session": tmuxEscapedSessionNameFormat,
		"window":  tmuxEscapedWindowNameFormat,
	}
	for name, format := range formats {
		if !strings.Contains(format, tmuxEscapedTab) || !strings.Contains(format, tmuxEscapedNewline) {
			t.Errorf("%s format = %q, want display-safe substitutions", name, format)
		}
		if !strings.Contains(format, "s|\t|"+tmuxEscapedTab) || !strings.Contains(format, "s|\n|"+tmuxEscapedNewline) {
			t.Errorf("%s format = %q, want tmux-side control character replacement", name, format)
		}
	}
}

func TestDisplaySafeTmuxTextDecodesTmuxEscapedControlChars(t *testing.T) {
	input := "tmux\\ttab tmux\\nnewline"
	want := "tmux" + tmuxEscapedTab + "tab tmux" + tmuxEscapedNewline + "newline"

	if got := displaySafeTmuxText(input); got != want {
		t.Errorf("displaySafeTmuxText() = %q, want %q", got, want)
	}
}

func TestDisplaySafeTmuxTextPreservesLiteralBackslashSequences(t *testing.T) {
	input := "literal\\\\ttab literal\\\\nnewline backslash\\\\"
	want := "literal\\ttab literal\\nnewline backslash\\"

	if got := displaySafeTmuxText(input); got != want {
		t.Errorf("displaySafeTmuxText() = %q, want %q", got, want)
	}
}

func TestDisplaySafeTmuxTextHandlesActualAndUnknownEscapes(t *testing.T) {
	input := "actual\ttab actual\nnewline unknown\\x trailing\\"
	want := "actual" + tmuxEscapedTab + "tab actual" + tmuxEscapedNewline + "newline unknown\\x trailing\\"

	if got := displaySafeTmuxText(input); got != want {
		t.Errorf("displaySafeTmuxText() = %q, want %q", got, want)
	}
}
