package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jmcampanini/cmdk/internal/config"
	"github.com/jmcampanini/cmdk/internal/theme"
)

func TestConfigReportLoadsBackWithThemeOverrides(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config\ninjected = true\r\x1b[31m\n#.toml")
	if err := os.WriteFile(path, []byte(`
[[actions]]
name = "Editor"
matches = "dir"
cmd = "nvim"
icon = ":nf-dev-github:"
stages = [{ type = "picker", key = "file", source = "printf file", pass = 1 }]

[display]
truncation_symbol = ":nf-cod-ellipsis:"

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
		output := executeRootForTest(t, args...)
		for _, want := range []string{
			"# Derived values",
			`# actions[0].launch_mode = "session-window"`,
			`# actions[0].window_name = "{{.launch_basename}}"`,
			`# actions[0].stages[0].delimiter = "|"`,
			`# display.truncation_symbol = "\uea7c"`,
		} {
			if !strings.Contains(output, want) {
				t.Errorf("cmdk %v report missing %q", args, want)
			}
		}
		if err := os.WriteFile(report, []byte(output), 0o644); err != nil {
			t.Fatal(err)
		}

		cfg, _, err := config.Load(report)
		if err != nil {
			t.Fatalf("cmdk %v: loading the report back: %v", args, err)
		}
		if len(cfg.Actions) != 1 || cfg.Actions[0].LaunchMode != config.LaunchModeSessionWindow || cfg.Actions[0].Icon != "\ue709" {
			t.Fatalf("cmdk %v: actions = %+v, want resolved editor action", args, cfg.Actions)
		}
		action := cfg.Actions[0]
		if action.WindowName != "{{.launch_basename}}" || len(action.Stages) != 1 || action.Stages[0].Delimiter != "|" {
			t.Errorf("cmdk %v: action = %+v, want resolved window name and picker delimiter", args, action)
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
