package theme

import "testing"

// stubDarkBackground must not be used with t.Parallel — it mutates a package-level var.
func stubDarkBackground(t *testing.T, dark bool) {
	t.Helper()
	prev := hasDarkBackground
	t.Cleanup(func() { hasDarkBackground = prev })
	hasDarkBackground = func() bool { return dark }
}

func TestResolve(t *testing.T) {
	tests := []struct {
		name       string
		darkBg     bool
		input      string
		wantTheme  string
	}{
		{"auto-detect dark background", true, "", "dark"},
		{"auto-detect light background", false, "", "light"},
		{"explicit theme overrides auto-detect", false, "dark", "dark"},
		{"explicit light overrides dark background", true, "light", "light"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stubDarkBackground(t, tt.darkBg)

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

func TestResolve_UnknownThemeReturnsError(t *testing.T) {
	_, err := Resolve("bogus")
	if err == nil {
		t.Fatal("Resolve(\"bogus\") should return an error")
	}
}
