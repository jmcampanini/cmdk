package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/jmcampanini/cmdk/internal/config"
	"github.com/jmcampanini/cmdk/internal/theme"
)

func TestConfigReportLoadsBackWithThemeOverrides(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte(`
[theme.dark]
accent_text = "#123456"

[theme.dark.roles]
session_icon = "#345678"
`), 0o644); err != nil {
		t.Fatal(err)
	}

	for _, args := range [][]string{
		{"config", "--config", path},
		{"config", "--config", path, "--provenance"},
	} {
		report := filepath.Join(t.TempDir(), "report.toml")
		if err := os.WriteFile(report, []byte(executeRootForTest(t, args...)), 0o644); err != nil {
			t.Fatal(err)
		}

		cfg, _, err := config.Load(report)
		if err != nil {
			t.Fatalf("cmdk %v: loading the report back: %v", args, err)
		}
		dark := theme.Dark(cfg.Theme.Overrides())
		if dark.Tokens.AccentText != theme.Color("#123456") {
			t.Errorf("cmdk %v: accent_text = %v, want #123456", args, dark.Tokens.AccentText)
		}
		if dark.Roles.SessionIcon != theme.Color("#345678") {
			t.Errorf("cmdk %v: roles.session_icon = %v, want #345678", args, dark.Roles.SessionIcon)
		}
	}
}
