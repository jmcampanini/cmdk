package theme

import (
	"reflect"
	"testing"

	"charm.land/lipgloss/v2"
)

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

func TestDarkUsesFrappeDefaults(t *testing.T) {
	dark := Dark()
	checks := []struct {
		name string
		got  any
		want any
	}{
		{"accent", dark.Tokens.Accent, lipgloss.Color("#ca9ee6")},
		{"accent text", dark.Tokens.AccentText, lipgloss.Color("#303446")},
		{"text", dark.Tokens.Text, lipgloss.Color("#c6d0f5")},
		{"selected bg", dark.Tokens.SelectedBg, lipgloss.Color("#51576d")},
		{"input bg", dark.Tokens.InputBg, lipgloss.Color("#414559")},
		{"dir role", dark.Roles.DirIcon, lipgloss.Color("#8caaee")},
	}
	for _, check := range checks {
		if !reflect.DeepEqual(check.got, check.want) {
			t.Errorf("%s = %v, want %v", check.name, check.got, check.want)
		}
	}
}

func TestOverridesReplaceSemanticTokensAndReDeriveRoles(t *testing.T) {
	overrides := Overrides{
		NameDark: {Tokens: Tokens{
			Accent:  Color("#111111"),
			Info:    Color("#222222"),
			Warning: Color("#333333"),
		}},
	}

	dark := Dark(overrides)

	if !reflect.DeepEqual(dark.Tokens.Accent, lipgloss.Color("#111111")) {
		t.Errorf("accent = %v, want override", dark.Tokens.Accent)
	}
	if !reflect.DeepEqual(dark.Roles.WindowIcon, lipgloss.Color("#111111")) {
		t.Errorf("window icon = %v, want derived accent override", dark.Roles.WindowIcon)
	}
	if !reflect.DeepEqual(dark.Roles.DirIcon, lipgloss.Color("#222222")) {
		t.Errorf("dir icon = %v, want derived info override", dark.Roles.DirIcon)
	}
	if !reflect.DeepEqual(dark.Roles.BellIcon, lipgloss.Color("#333333")) {
		t.Errorf("bell icon = %v, want derived warning override", dark.Roles.BellIcon)
	}
}

func TestRoleOverridesWinOverSemanticDerivation(t *testing.T) {
	overrides := Overrides{
		NameDark: {
			Roles:  Roles{DirIcon: Color("#444444")},
			Tokens: Tokens{Info: Color("#222222")},
		},
	}

	dark := Dark(overrides)

	if !reflect.DeepEqual(dark.Tokens.Info, lipgloss.Color("#222222")) {
		t.Errorf("info = %v, want semantic override", dark.Tokens.Info)
	}
	if !reflect.DeepEqual(dark.Roles.DirIcon, lipgloss.Color("#444444")) {
		t.Errorf("dir icon = %v, want role override", dark.Roles.DirIcon)
	}
}

func TestOverridesApplyOnlyToMatchingMode(t *testing.T) {
	overrides := Overrides{
		NameLight: {Tokens: Tokens{Accent: Color("#111111")}},
	}

	dark := Dark(overrides)
	if reflect.DeepEqual(dark.Tokens.Accent, lipgloss.Color("#111111")) {
		t.Error("dark theme should not use light override")
	}
}
