package config

import (
	"os"
	"path/filepath"
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
