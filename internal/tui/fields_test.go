package tui

import "testing"

func TestExtractField_IndexZero_ReturnsWholeLine(t *testing.T) {
	got := extractField("alice|alice@example.com", "|", 0)
	if got != "alice|alice@example.com" {
		t.Errorf("extractField(_, _, 0) = %q, want whole line", got)
	}
}

func TestExtractField_ValidIndex(t *testing.T) {
	line := "alice|alice@example.com|admin"
	tests := []struct {
		index int
		want  string
	}{
		{1, "alice"},
		{2, "alice@example.com"},
		{3, "admin"},
	}
	for _, tt := range tests {
		got := extractField(line, "|", tt.index)
		if got != tt.want {
			t.Errorf("extractField(_, _, %d) = %q, want %q", tt.index, got, tt.want)
		}
	}
}

func TestExtractField_OutOfBounds_FallsBackToWholeLine(t *testing.T) {
	line := "one|two"
	got := extractField(line, "|", 3)
	if got != line {
		t.Errorf("extractField(_, _, 3) = %q, want whole line %q", got, line)
	}
}

func TestExtractField_SingleField_NoDelimiter(t *testing.T) {
	line := "nodelimiter"
	got := extractField(line, "|", 1)
	if got != "nodelimiter" {
		t.Errorf("extractField single field = %q, want %q", got, "nodelimiter")
	}
}

func TestExtractField_SingleField_OutOfBounds(t *testing.T) {
	line := "nodelimiter"
	got := extractField(line, "|", 2)
	if got != line {
		t.Errorf("extractField single field OOB = %q, want whole line", got)
	}
}

func TestExtractField_MultiCharDelimiter(t *testing.T) {
	line := "alice::bob::charlie"
	got := extractField(line, "::", 2)
	if got != "bob" {
		t.Errorf("extractField multi-char delim = %q, want %q", got, "bob")
	}
}

func TestExtractField_EmptyFieldValue(t *testing.T) {
	line := "name|"
	got := extractField(line, "|", 2)
	if got != "" {
		t.Errorf("extractField empty field = %q, want empty string", got)
	}
}

func TestExtractField_PreservesWhitespace(t *testing.T) {
	line := " alice | bob "
	got := extractField(line, "|", 1)
	if got != " alice " {
		t.Errorf("extractField should preserve whitespace, got %q, want %q", got, " alice ")
	}
}
