package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoad_ValidTOML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(path, []byte(`
[timeout]
fetch_ms = 1500

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
	if cfg.Timeout.FetchMs != 1500 {
		t.Errorf("timeout.fetch_ms = %d, want 1500", cfg.Timeout.FetchMs)
	}
}

func TestLoad_MissingFile(t *testing.T) {
	cfg, err := Load("/nonexistent/path/config.toml")
	if err != nil {
		t.Fatalf("expected nil error, got: %v", err)
	}
	if cfg == nil {
		t.Fatal("expected non-nil config")
	}
	if len(cfg.Commands) != 0 {
		t.Errorf("got %d commands, want 0", len(cfg.Commands))
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
	if len(cfg.Commands) != 0 {
		t.Errorf("got %d commands, want 0", len(cfg.Commands))
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

func TestFetchTimeout_DefaultsToTwoSeconds(t *testing.T) {
	cfg := &Config{}
	if got := cfg.FetchTimeout(); got != 2*time.Second {
		t.Errorf("zero timeout FetchTimeout() = %s, want %s", got, 2*time.Second)
	}
}

func TestFetchTimeout_UsesConfiguredMilliseconds(t *testing.T) {
	cfg := &Config{Timeout: Timeout{FetchMs: 750}}

	if got := cfg.FetchTimeout(); got != 750*time.Millisecond {
		t.Fatalf("FetchTimeout() = %s, want %s", got, 750*time.Millisecond)
	}
}

func TestSourceLimit_DefaultForZoxide(t *testing.T) {
	cfg := &Config{}
	if got := cfg.SourceLimit("zoxide"); got != 20 {
		t.Errorf("SourceLimit(zoxide) = %d, want 20", got)
	}
}

func TestSourceLimit_ConfiguredValue(t *testing.T) {
	cfg := &Config{
		Sources: map[string]SourceConfig{
			"zoxide": {Limit: 10},
		},
	}
	if got := cfg.SourceLimit("zoxide"); got != 10 {
		t.Errorf("SourceLimit(zoxide) = %d, want 10", got)
	}
}

func TestSourceLimit_ZeroMeansUnlimited(t *testing.T) {
	cfg := &Config{
		Sources: map[string]SourceConfig{
			"zoxide": {Limit: 0},
		},
	}
	if got := cfg.SourceLimit("zoxide"); got != 0 {
		t.Errorf("SourceLimit(zoxide) = %d, want 0 (unlimited)", got)
	}
}

func TestSourceLimit_UnknownSourceReturnsZero(t *testing.T) {
	cfg := &Config{}
	if got := cfg.SourceLimit("windows"); got != 0 {
		t.Errorf("SourceLimit(windows) = %d, want 0", got)
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
	if got := cfg.SourceLimit("zoxide"); got != 5 {
		t.Errorf("SourceLimit(zoxide) = %d, want 5", got)
	}
}
