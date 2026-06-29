package theme

import "testing"

func TestResolve(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantTheme string
	}{
		{"default theme is dark", "", "dark"},
		{"explicit dark", "dark", "dark"},
		{"explicit light", "light", "light"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Resolve(tt.input)
			if err != nil {
				t.Fatalf("Resolve(%q) returned error: %v", tt.input, err)
			}
			if got.Name != tt.wantTheme {
				t.Fatalf("Resolve(%q) = %q, want %q", tt.input, got.Name, tt.wantTheme)
			}
		})
	}
}

func TestFromBackground(t *testing.T) {
	tests := []struct {
		name      string
		isDark    bool
		wantTheme string
	}{
		{"dark background", true, "dark"},
		{"light background", false, "light"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FromBackground(tt.isDark)
			if got.Name != tt.wantTheme {
				t.Fatalf("FromBackground(%v) = %q, want %q", tt.isDark, got.Name, tt.wantTheme)
			}
		})
	}
}

func TestResolve_UnknownThemeReturnsError(t *testing.T) {
	_, err := Resolve("bogus")
	if err == nil {
		t.Fatal("Resolve(\"bogus\") should return an error")
	}
}
