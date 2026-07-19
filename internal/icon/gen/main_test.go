package main

import (
	"bytes"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
	"unicode"
)

func TestDescriptionFromAlias(t *testing.T) {
	tests := []struct {
		key  string
		want string
	}{
		{"cod-terminal_tmux", "Terminal tmux"},
		{"dev-go", "Go"},
		{"oct-git_branch", "Git branch"},
		{"cod-arrow_circle_down", "Arrow circle down"},
		{"dev-a", "A"},
	}
	for _, tt := range tests {
		got := descriptionFromAlias(tt.key)
		if got != tt.want {
			t.Errorf("descriptionFromAlias(%q) = %q, want %q", tt.key, got, tt.want)
		}
	}
}

func TestDescriptionFromAlias_NoDash(t *testing.T) {
	got := descriptionFromAlias("nodash")
	if got != "" {
		t.Errorf("descriptionFromAlias(\"nodash\") = %q, want empty", got)
	}
}

func TestParseGlyphCode_Valid(t *testing.T) {
	tests := []struct {
		code string
		want rune
	}{
		{"0", 0},
		{"41", 'A'},
		{"d7ff", 0xD7FF},   // last code point before the surrogate range
		{"e000", 0xE000},   // first code point after the surrogate range
		{"ffff", 0xFFFF},   // top of the Basic Multilingual Plane
		{"10000", 0x10000}, // first supplementary code point
		{"10ffff", unicode.MaxRune},
	}
	for _, tt := range tests {
		got, err := parseGlyphCode("cod-ok", tt.code)
		if err != nil {
			t.Errorf("parseGlyphCode(%q) error: %v", tt.code, err)
			continue
		}
		if got != tt.want {
			t.Errorf("parseGlyphCode(%q) = %#x, want %#x", tt.code, got, tt.want)
		}
	}
}

func TestParseGlyphCode_Invalid(t *testing.T) {
	tests := []string{
		"110000",    // unicode.MaxRune + 1
		"ffffffff",  // max uint32
		"1ffffffff", // overflows 32 bits
		"not-hex",
		"-1",
		"0x41",
		"",
	}
	for _, code := range tests {
		_, err := parseGlyphCode("cod-bad", code)
		if err == nil {
			t.Errorf("parseGlyphCode(%q) succeeded, want error", code)
			continue
		}
		msg := err.Error()
		if !strings.Contains(msg, `"cod-bad"`) || !strings.Contains(msg, code) {
			t.Errorf("parseGlyphCode(%q) error %q does not identify glyph key and code", code, msg)
		}
	}
}

func TestParseGlyphCode_RejectsAllSurrogates(t *testing.T) {
	for cp := 0xD800; cp <= 0xDFFF; cp++ {
		code := strconv.FormatInt(int64(cp), 16)
		if _, err := parseGlyphCode("cod-surrogate", code); err == nil {
			t.Errorf("parseGlyphCode(%q) succeeded, want error", code)
		}
	}
}

func TestBuildEntries_ValidFixture(t *testing.T) {
	entries, err := buildEntries(readFixture(t, "glyphs_valid.json"))
	if err != nil {
		t.Fatalf("buildEntries: %v", err)
	}
	want := []entry{
		{"nf-cod-account", 0xEB99, "Account"},
		{"nf-cod-after_surrogates", 0xE000, "After surrogates"},
		{"nf-cod-before_surrogates", 0xD7FF, "Before surrogates"},
		{"nf-cod-bmp_max", 0xFFFF, "Bmp max"},
		{"nf-cod-max_scalar", unicode.MaxRune, "Max scalar"},
		{"nf-cod-min_scalar", 0, "Min scalar"},
		{"nf-cod-supplementary_min", 0x10000, "Supplementary min"},
		{"nf-dev-go", 0xE724, "Go"},
		{"nf-oct-git_branch", 0xF418, "Git branch"},
	}
	if !slices.Equal(entries, want) {
		t.Errorf("buildEntries = %v, want %v", entries, want)
	}
}

func TestBuildEntries_InvalidFixtures(t *testing.T) {
	tests := []struct {
		fixture string
		key     string
		code    string
	}{
		{"glyphs_surrogate.json", "cod-surrogate_low", "d800"},
		{"glyphs_out_of_range.json", "cod-beyond_max_scalar", "110000"},
		{"glyphs_uint32_max.json", "cod-uint32_max", "ffffffff"},
		{"glyphs_overflow.json", "cod-overflow", "1ffffffff"},
		{"glyphs_malformed_hex.json", "cod-malformed", "not-hex"},
	}
	for _, tt := range tests {
		_, err := buildEntries(readFixture(t, tt.fixture))
		if err == nil {
			t.Errorf("buildEntries(%s) succeeded, want error", tt.fixture)
			continue
		}
		msg := err.Error()
		if !strings.Contains(msg, tt.key) || !strings.Contains(msg, tt.code) {
			t.Errorf("buildEntries(%s) error %q does not identify glyph key and code", tt.fixture, msg)
		}
		if strings.Contains(msg, "�") {
			t.Errorf("buildEntries(%s) error %q contains a replacement character", tt.fixture, msg)
		}
	}
}

func TestRender_ValidFixtureDeterministic(t *testing.T) {
	render1 := renderFixture(t, "glyphs_valid.json")
	render2 := renderFixture(t, "glyphs_valid.json")
	if !bytes.Equal(render1, render2) {
		t.Fatal("render produced different output for identical input")
	}

	src := string(render1)
	if !strings.Contains(src, `\U0010ffff`) {
		t.Errorf("rendered source missing escaped max scalar U+10FFFF:\n%s", src)
	}
	if strings.Contains(src, "�") || strings.Contains(src, "\\ufffd") {
		t.Errorf("rendered source contains a replacement character:\n%s", src)
	}
}

func renderFixture(t *testing.T, name string) []byte {
	t.Helper()
	entries, err := buildEntries(readFixture(t, name))
	if err != nil {
		t.Fatalf("buildEntries: %v", err)
	}
	src, err := render(entries)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	return src
}

func readFixture(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestHasAllowedPrefix(t *testing.T) {
	tests := []struct {
		key  string
		want bool
	}{
		{"cod-terminal", true},
		{"dev-go", true},
		{"oct-git_branch", true},
		{"md-folder", false},
		{"fa-terminal", false},
		{"METADATA", false},
	}
	for _, tt := range tests {
		got := hasAllowedPrefix(tt.key)
		if got != tt.want {
			t.Errorf("hasAllowedPrefix(%q) = %v, want %v", tt.key, got, tt.want)
		}
	}
}
