package theme

import "testing"

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
