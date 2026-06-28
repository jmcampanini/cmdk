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

func TestDisplaySafeTmuxWindowNameHandlesActualControls(t *testing.T) {
	input := "actual\ttab actual\nnewline actual\rreturn"
	want := "actual" + tmuxEscapedTab + "tab actual" + tmuxEscapedNewline + "newline actual" + tmuxEscapedNewline + "return"

	if got := displaySafeTmuxWindowName(input); got != want {
		t.Errorf("displaySafeTmuxWindowName() = %q, want %q", got, want)
	}
}

func TestDisplaySafeTmuxWindowNamePreservesLiteralBackslashSequences(t *testing.T) {
	input := `literal\ttab literal\nnewline backslash\ unknown\x`

	if got := displaySafeTmuxWindowName(input); got != input {
		t.Errorf("displaySafeTmuxWindowName() = %q, want %q", got, input)
	}
}

func TestDisplaySafeTmuxWindowNameCollapsesDoubledLiteralBackslashes(t *testing.T) {
	input := `literal\\ttab literal\\nnewline`
	want := `literal\ttab literal\nnewline`

	if got := displaySafeTmuxWindowName(input); got != want {
		t.Errorf("displaySafeTmuxWindowName() = %q, want %q", got, want)
	}
}

func TestDisplaySafeTmuxSessionNameDecodesTmuxEscapedControlChars(t *testing.T) {
	input := "tmux\\ttab tmux\\nnewline"
	want := "tmux" + tmuxEscapedTab + "tab tmux" + tmuxEscapedNewline + "newline"

	if got := displaySafeTmuxSessionName(input); got != want {
		t.Errorf("displaySafeTmuxSessionName() = %q, want %q", got, want)
	}
}

func TestDisplaySafeTmuxSessionNamePreservesLiteralBackslashSequences(t *testing.T) {
	input := "literal\\\\ttab literal\\\\nnewline backslash\\\\"
	want := "literal\\ttab literal\\nnewline backslash\\"

	if got := displaySafeTmuxSessionName(input); got != want {
		t.Errorf("displaySafeTmuxSessionName() = %q, want %q", got, want)
	}
}

func TestDisplaySafeTmuxNamesReplaceUnsafeBytesAndControls(t *testing.T) {
	input := string([]byte{'o', 0xff, 'k', 0x1b})
	want := "o_k_"

	if got := displaySafeTmuxWindowName(input); got != want {
		t.Errorf("displaySafeTmuxWindowName() = %q, want %q", got, want)
	}
	if got := displaySafeTmuxSessionName(input); got != want {
		t.Errorf("displaySafeTmuxSessionName() = %q, want %q", got, want)
	}
}
