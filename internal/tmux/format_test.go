package tmux

import "testing"

func TestTmuxNameFormatsUseRawTmuxFields(t *testing.T) {
	if tmuxEscapedSessionNameFormat != "#{session_name}" {
		t.Errorf("session name format = %q, want raw tmux field", tmuxEscapedSessionNameFormat)
	}
	if tmuxEscapedWindowNameFormat != "#{window_name}" {
		t.Errorf("window name format = %q, want raw tmux field", tmuxEscapedWindowNameFormat)
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
