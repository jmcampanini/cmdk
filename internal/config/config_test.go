package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.Timeout.Fetch != 2*time.Second {
		t.Errorf("Timeout.Fetch = %s, want 2s", cfg.Timeout.Fetch)
	}
	if cfg.Sources["zoxide"].Limit != 0 {
		t.Errorf("Sources[zoxide].Limit = %d, want 0", cfg.Sources["zoxide"].Limit)
	}
	if cfg.Sources["zoxide"].MinScore != 0 {
		t.Errorf("Sources[zoxide].MinScore = %f, want 0", cfg.Sources["zoxide"].MinScore)
	}
	if len(cfg.Commands) != 0 {
		t.Errorf("Commands = %d, want 0", len(cfg.Commands))
	}
	if cfg.Display.ShortenHome == nil || *cfg.Display.ShortenHome != "~" {
		t.Errorf("Display.ShortenHome = %v, want pointer to \"~\"", cfg.Display.ShortenHome)
	}
}

func TestValidate_Valid(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Commands = []Command{{Name: "htop", Cmd: "htop"}}
	if err := cfg.Validate(); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidate_ZeroTimeout(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Timeout.Fetch = 0
	if err := cfg.Validate(); err != nil {
		t.Errorf("zero timeout should be valid: %v", err)
	}
}

func TestValidate_NegativeTimeout(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Timeout.Fetch = -1 * time.Second
	if err := cfg.Validate(); err == nil {
		t.Error("expected error for negative timeout")
	}
}

func TestValidate_NegativeLimit(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Sources["zoxide"] = SourceConfig{Limit: -1}
	if err := cfg.Validate(); err == nil {
		t.Error("expected error for negative limit")
	}
}

func TestValidate_NegativeMinScore(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Sources["zoxide"] = SourceConfig{MinScore: -1.0}
	if err := cfg.Validate(); err == nil {
		t.Error("expected error for negative min_score")
	}
}

func TestValidate_EmptyCommandName(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Commands = []Command{{Name: "", Cmd: "htop"}}
	if err := cfg.Validate(); err == nil {
		t.Error("expected error for empty command name")
	}
}

func TestValidate_EmptyCommandCmd(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Commands = []Command{{Name: "htop", Cmd: ""}}
	if err := cfg.Validate(); err == nil {
		t.Error("expected error for empty command cmd")
	}
}

func TestLoad_ValidTOML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(path, []byte(`
[timeout]
fetch = "1500ms"

[[commands]]
name = "htop"
cmd = "htop"

[[commands]]
name = "logs"
cmd = "tail -f /var/log/syslog"
`), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg == nil {
		t.Fatal("expected non-nil config")
	}
	if len(cfg.Commands) != 2 {
		t.Fatalf("got %d commands, want 2", len(cfg.Commands))
	}
	if cfg.Commands[0].Name != "htop" {
		t.Errorf("commands[0].Name = %q, want %q", cfg.Commands[0].Name, "htop")
	}
	if cfg.Commands[1].Cmd != "tail -f /var/log/syslog" {
		t.Errorf("commands[1].Cmd = %q", cfg.Commands[1].Cmd)
	}
	if cfg.Timeout.Fetch != 1500*time.Millisecond {
		t.Errorf("timeout.fetch = %s, want 1500ms", cfg.Timeout.Fetch)
	}
}

func TestLoad_MissingFile_ReturnsDefaults(t *testing.T) {
	cfg, err := Load("/nonexistent/path/config.toml")
	if err != nil {
		t.Fatalf("expected nil error, got: %v", err)
	}
	if cfg == nil {
		t.Fatal("expected non-nil config")
	}
	defaults := DefaultConfig()
	if cfg.Timeout.Fetch != defaults.Timeout.Fetch {
		t.Errorf("Timeout.Fetch = %s, want %s", cfg.Timeout.Fetch, defaults.Timeout.Fetch)
	}
	if cfg.Sources["zoxide"].Limit != defaults.Sources["zoxide"].Limit {
		t.Errorf("Sources[zoxide].Limit = %d, want %d", cfg.Sources["zoxide"].Limit, defaults.Sources["zoxide"].Limit)
	}
}

func TestLoad_MalformedTOML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(path, []byte(`[[[broken`), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(path)
	if err == nil {
		t.Fatal("expected error for malformed TOML")
	}
	if cfg == nil {
		t.Fatal("expected non-nil config even on error")
	}
}

func TestLoad_EmptyFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(path, []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg == nil {
		t.Fatal("expected non-nil config for empty file")
	}
	if cfg.Timeout.Fetch != 2*time.Second {
		t.Errorf("Timeout.Fetch = %s, want 2s (default preserved)", cfg.Timeout.Fetch)
	}
}

func TestLoad_PreservesOrder(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(path, []byte(`
[[commands]]
name = "alpha"
cmd = "echo alpha"

[[commands]]
name = "beta"
cmd = "echo beta"

[[commands]]
name = "gamma"
cmd = "echo gamma"
`), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cfg.Commands) != 3 {
		t.Fatalf("got %d commands, want 3", len(cfg.Commands))
	}
	want := []string{"alpha", "beta", "gamma"}
	for i, w := range want {
		if cfg.Commands[i].Name != w {
			t.Errorf("commands[%d].Name = %q, want %q", i, cfg.Commands[i].Name, w)
		}
	}
}

func TestLoad_ValidationError_ReturnsDefaults(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(path, []byte(`
[timeout]
fetch = "-1s"
`), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(path)
	if err == nil {
		t.Fatal("expected validation error")
	}
	defaults := DefaultConfig()
	if cfg.Timeout.Fetch != defaults.Timeout.Fetch {
		t.Errorf("Timeout.Fetch = %s, want default %s", cfg.Timeout.Fetch, defaults.Timeout.Fetch)
	}
}

func TestLoad_SourcesSection(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(path, []byte(`
[sources.zoxide]
limit = 5
`), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Sources["zoxide"].Limit != 5 {
		t.Errorf("Sources[zoxide].Limit = %d, want 5", cfg.Sources["zoxide"].Limit)
	}
}

func TestLoad_MinScoreFromTOML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(path, []byte(`
[sources.zoxide]
min_score = 2.5
`), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Sources["zoxide"].MinScore != 2.5 {
		t.Errorf("Sources[zoxide].MinScore = %f, want 2.5", cfg.Sources["zoxide"].MinScore)
	}
}

func TestValidate_SuspiciouslySmallTimeout(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Timeout.Fetch = 500 * time.Nanosecond
	if err := cfg.Validate(); err == nil {
		t.Error("expected error for sub-millisecond timeout")
	}
}

func TestLoad_OtherSourcePreservesZoxideDefault(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(path, []byte(`
[sources.fish]
limit = 10
`), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Sources["fish"].Limit != 10 {
		t.Errorf("Sources[fish].Limit = %d, want 10", cfg.Sources["fish"].Limit)
	}
	if cfg.Sources["zoxide"].Limit != 0 {
		t.Errorf("Sources[zoxide].Limit = %d, want 0 (default backfilled)", cfg.Sources["zoxide"].Limit)
	}
}

func TestDefaultPath_XDGOverride(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "/tmp/xdg-test")
	got := DefaultPath()
	want := "/tmp/xdg-test/cmdk/config.toml"
	if got != want {
		t.Errorf("DefaultPath() = %q, want %q", got, want)
	}
}

func TestDefaultPath_Fallback(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "")
	got := DefaultPath()
	home, _ := os.UserHomeDir()
	want := filepath.Join(home, ".config", "cmdk", "config.toml")
	if got != want {
		t.Errorf("DefaultPath() = %q, want %q", got, want)
	}
}

func TestLoad_DisplayRules(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(path, []byte(`
[display.rules]
"github.palantir.build" = "gpb"
"~/Code" = "~/dev"
`), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cfg.Display.Rules) != 2 {
		t.Fatalf("got %d rules, want 2", len(cfg.Display.Rules))
	}
	if cfg.Display.Rules["github.palantir.build"] != "gpb" {
		t.Errorf("rule = %q, want %q", cfg.Display.Rules["github.palantir.build"], "gpb")
	}
	if *cfg.Display.ShortenHome != "~" {
		t.Errorf("ShortenHome = %q, want default %q", *cfg.Display.ShortenHome, "~")
	}
}

func TestLoad_ShortenHomeDisabled(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(path, []byte(`
[display]
shorten_home = ""
`), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Display.ShortenHome == nil {
		t.Fatal("ShortenHome should not be nil")
	}
	if *cfg.Display.ShortenHome != "" {
		t.Errorf("ShortenHome = %q, want empty string", *cfg.Display.ShortenHome)
	}
}

func TestValidate_EmptyDisplayRuleKey(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Display.Rules = map[string]string{"": "bad"}
	if err := cfg.Validate(); err == nil {
		t.Error("expected error for empty rule key")
	}
}

func TestValidate_ValidDirActions(t *testing.T) {
	cfg := DefaultConfig()
	cfg.DirActions = []Command{
		{Name: "Yazi", Cmd: "tmux split-window -h yazi"},
	}
	if err := cfg.Validate(); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidate_EmptyDirActionName(t *testing.T) {
	cfg := DefaultConfig()
	cfg.DirActions = []Command{{Name: "", Cmd: "yazi"}}
	if err := cfg.Validate(); err == nil {
		t.Error("expected error for empty dir_action name")
	}
}

func TestValidate_EmptyDirActionCmd(t *testing.T) {
	cfg := DefaultConfig()
	cfg.DirActions = []Command{{Name: "Yazi", Cmd: ""}}
	if err := cfg.Validate(); err == nil {
		t.Error("expected error for empty dir_action cmd")
	}
}

func TestLoad_DirActions(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(path, []byte(`
[[dir_actions]]
name = "Yazi"
cmd = "tmux split-window -h yazi"

[[dir_actions]]
name = "New pane"
cmd = "tmux split-window -v"
`), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cfg.DirActions) != 2 {
		t.Fatalf("got %d dir_actions, want 2", len(cfg.DirActions))
	}
	if cfg.DirActions[0].Name != "Yazi" {
		t.Errorf("dir_actions[0].Name = %q, want %q", cfg.DirActions[0].Name, "Yazi")
	}
	if cfg.DirActions[1].Cmd != "tmux split-window -v" {
		t.Errorf("dir_actions[1].Cmd = %q", cfg.DirActions[1].Cmd)
	}
}

func TestLoad_DirActionsPreservesOrder(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(path, []byte(`
[[dir_actions]]
name = "alpha"
cmd = "echo alpha"

[[dir_actions]]
name = "beta"
cmd = "echo beta"

[[dir_actions]]
name = "gamma"
cmd = "echo gamma"
`), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cfg.DirActions) != 3 {
		t.Fatalf("got %d dir_actions, want 3", len(cfg.DirActions))
	}
	want := []string{"alpha", "beta", "gamma"}
	for i, w := range want {
		if cfg.DirActions[i].Name != w {
			t.Errorf("dir_actions[%d].Name = %q, want %q", i, cfg.DirActions[i].Name, w)
		}
	}
}

func TestLoad_NoDirActions(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(path, []byte(`
[[commands]]
name = "htop"
cmd = "htop"
`), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cfg.DirActions) != 0 {
		t.Errorf("got %d dir_actions, want 0", len(cfg.DirActions))
	}
}

func TestDefaultConfig_NoDirActions(t *testing.T) {
	cfg := DefaultConfig()
	if len(cfg.DirActions) != 0 {
		t.Errorf("DirActions = %d, want 0", len(cfg.DirActions))
	}
}

func TestValidate_ValidIconAlias(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Commands = []Command{{Name: "GitHub", Cmd: "open gh", Icon: ":nf-dev-github:"}}
	if err := cfg.Validate(); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidate_InvalidIconAlias(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Commands = []Command{{Name: "test", Cmd: "test", Icon: ":nf-fake-thing:"}}
	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected error for invalid icon alias")
	}
	if !strings.Contains(err.Error(), "commands[0].icon") {
		t.Errorf("error = %q, want prefix commands[0].icon", err.Error())
	}
}

func TestValidate_ValidIconRawUnicode(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Commands = []Command{{Name: "test", Cmd: "test", Icon: "\ue709"}}
	if err := cfg.Validate(); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidate_InvalidIconMultiChar(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Commands = []Command{{Name: "test", Cmd: "test", Icon: "ab"}}
	if err := cfg.Validate(); err == nil {
		t.Error("expected error for multi-character icon")
	}
}

func TestValidate_EmptyIconOK(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Commands = []Command{{Name: "test", Cmd: "test", Icon: ""}}
	if err := cfg.Validate(); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidate_DirActionIconAlias(t *testing.T) {
	cfg := DefaultConfig()
	cfg.DirActions = []Command{{Name: "Yazi", Cmd: "yazi", Icon: ":nf-md-folder:"}}
	if err := cfg.Validate(); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestLoad_CommandWithIconAlias(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(path, []byte(`
[[commands]]
name = "GitHub"
cmd = "open https://github.com"
icon = ":nf-dev-github:"
`), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Commands[0].Icon != "\ue709" {
		t.Errorf("Icon = %q, want resolved unicode \\ue709", cfg.Commands[0].Icon)
	}
}

func TestLoad_CommandWithUnicodeIcon(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(path, []byte("[[commands]]\nname = \"test\"\ncmd = \"test\"\nicon = \"\ue709\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Commands[0].Icon != "\ue709" {
		t.Errorf("Icon = %q, want \\ue709", cfg.Commands[0].Icon)
	}
}

func TestLoad_DirActionWithIcon(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(path, []byte(`
[[dir_actions]]
name = "Browse"
cmd = "yazi"
icon = ":nf-md-folder_open:"
`), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.DirActions[0].Icon != "\U000f0770" {
		t.Errorf("Icon = %q, want resolved unicode \\U000f0770", cfg.DirActions[0].Icon)
	}
}
