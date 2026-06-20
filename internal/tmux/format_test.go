package tmux

import (
	"strings"
	"testing"
)

func TestTmuxTextFormatEscapesActualAndTmuxEscapedControlChars(t *testing.T) {
	formats := map[string]string{
		"session": tmuxEscapedSessionNameFormat,
		"window":  tmuxEscapedWindowNameFormat,
	}
	checks := map[string]string{
		"actual tab":      "#{s|\t|" + tmuxEscapedTab + "|",
		"actual newline":  "#{s|\n|" + tmuxEscapedNewline + "|",
		"escaped tab":     `#{s|\\t|` + tmuxEscapedTab + `|`,
		"escaped newline": `#{s|\\n|` + tmuxEscapedNewline + `|`,
	}

	for name, format := range formats {
		for checkName, want := range checks {
			if !strings.Contains(format, want) {
				t.Errorf("%s format missing %s replacement %q in %q", name, checkName, want, format)
			}
		}
	}
}

func TestDisplaySafeTmuxTextReplacesActualAndTmuxEscapedControlChars(t *testing.T) {
	input := "actual\ttab actual\nnewline escaped\\ttab escaped\\nnewline"
	want := "actual" + tmuxEscapedTab + "tab actual" + tmuxEscapedNewline + "newline escaped" + tmuxEscapedTab + "tab escaped" + tmuxEscapedNewline + "newline"

	if got := displaySafeTmuxText(input); got != want {
		t.Errorf("displaySafeTmuxText() = %q, want %q", got, want)
	}
}
