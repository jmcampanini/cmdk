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
	if len(cfg.Actions) != 0 {
		t.Errorf("Actions = %d, want 0", len(cfg.Actions))
	}
	if cfg.Display.ShortenHome != "~" {
		t.Errorf("Display.ShortenHome = %q, want \"~\"", cfg.Display.ShortenHome)
	}
	if !cfg.Behavior.BellToTop {
		t.Error("Behavior.BellToTop = false, want true")
	}
	if !cfg.Behavior.AutoSelectSingle {
		t.Error("Behavior.AutoSelectSingle = false, want true")
	}
	if !cfg.Behavior.WrapList {
		t.Error("Behavior.WrapList = false, want true")
	}
	if !cfg.Behavior.StartInFilter {
		t.Error("Behavior.StartInFilter = false, want true")
	}
	if cfg.Behavior.InlineActions {
		t.Error("Behavior.InlineActions = true, want false")
	}
}

func TestValidate_Valid(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Actions = []Action{{Name: "htop", Cmd: "htop", Matches: "root"}}
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

func TestValidate_EmptyActionName(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Actions = []Action{{Name: "", Cmd: "htop", Matches: "root"}}
	if err := cfg.Validate(); err == nil {
		t.Error("expected error for empty action name")
	}
}

func TestValidate_EmptyActionCmd(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Actions = []Action{{Name: "htop", Cmd: "", Matches: "root"}}
	if err := cfg.Validate(); err == nil {
		t.Error("expected error for empty action cmd")
	}
}

func TestValidate_EmptyActionMatches(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Actions = []Action{{Name: "htop", Cmd: "htop", Matches: ""}}
	if err := cfg.Validate(); err == nil {
		t.Error("expected error for empty action matches")
	}
}

func TestValidate_InvalidActionMatches(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Actions = []Action{{Name: "htop", Cmd: "htop", Matches: "invalid"}}
	if err := cfg.Validate(); err == nil {
		t.Error("expected error for invalid action matches")
	}
}

func TestValidate_ActionMatchesRoot(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Actions = []Action{{Name: "htop", Cmd: "htop", Matches: "root"}}
	if err := cfg.Validate(); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidate_ActionMatchesDir(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Actions = []Action{{Name: "Yazi", Cmd: "yazi", Matches: "dir"}}
	if err := cfg.Validate(); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidate_StagePromptValid(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Actions = []Action{{
		Name: "test", Cmd: "echo", Matches: "root",
		Stages: []StageConfig{{Type: "prompt", Key: "name", Text: "Enter name"}},
	}}
	if err := cfg.Validate(); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidate_StagePromptWithDefault(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Actions = []Action{{
		Name: "test", Cmd: "echo", Matches: "root",
		Stages: []StageConfig{{Type: "prompt", Key: "name", Text: "Enter name", Default: "world"}},
	}}
	if err := cfg.Validate(); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidate_StagePromptMissingText(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Actions = []Action{{
		Name: "test", Cmd: "echo", Matches: "root",
		Stages: []StageConfig{{Type: "prompt", Key: "name"}},
	}}
	if err := cfg.Validate(); err == nil {
		t.Error("expected error for prompt stage without text")
	}
}

func TestValidate_StagePromptForbidsSource(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Actions = []Action{{
		Name: "test", Cmd: "echo", Matches: "root",
		Stages: []StageConfig{{Type: "prompt", Key: "name", Text: "Enter", Source: "zoxide"}},
	}}
	if err := cfg.Validate(); err == nil {
		t.Error("expected error for prompt stage with source")
	}
}

func TestValidate_StagePickerValid(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Actions = []Action{{
		Name: "test", Cmd: "echo", Matches: "root",
		Stages: []StageConfig{{Type: "picker", Key: "dir", Source: "zoxide"}},
	}}
	if err := cfg.Validate(); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidate_StagePickerMissingSource(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Actions = []Action{{
		Name: "test", Cmd: "echo", Matches: "root",
		Stages: []StageConfig{{Type: "picker", Key: "dir"}},
	}}
	if err := cfg.Validate(); err == nil {
		t.Error("expected error for picker stage without source")
	}
}

func TestValidate_StagePickerForbidsText(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Actions = []Action{{
		Name: "test", Cmd: "echo", Matches: "root",
		Stages: []StageConfig{{Type: "picker", Key: "dir", Source: "zoxide", Text: "nope"}},
	}}
	if err := cfg.Validate(); err == nil {
		t.Error("expected error for picker stage with text")
	}
}

func TestValidate_StagePickerForbidsDefault(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Actions = []Action{{
		Name: "test", Cmd: "echo", Matches: "root",
		Stages: []StageConfig{{Type: "picker", Key: "dir", Source: "zoxide", Default: "nope"}},
	}}
	if err := cfg.Validate(); err == nil {
		t.Error("expected error for picker stage with default")
	}
}

func TestValidate_StagePickerWithFieldConfig(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Actions = []Action{{
		Name: "test", Cmd: "echo", Matches: "root",
		Stages: []StageConfig{{Type: "picker", Key: "dir", Source: "zoxide", Delimiter: "|", Display: 1, Pass: 2}},
	}}
	if err := cfg.Validate(); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidate_StagePickerNegativeDisplay(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Actions = []Action{{
		Name: "test", Cmd: "echo", Matches: "root",
		Stages: []StageConfig{{Type: "picker", Key: "dir", Source: "zoxide", Display: -1}},
	}}
	if err := cfg.Validate(); err == nil {
		t.Error("expected error for negative display")
	}
}

func TestValidate_StagePickerNegativePass(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Actions = []Action{{
		Name: "test", Cmd: "echo", Matches: "root",
		Stages: []StageConfig{{Type: "picker", Key: "dir", Source: "zoxide", Pass: -1}},
	}}
	if err := cfg.Validate(); err == nil {
		t.Error("expected error for negative pass")
	}
}

func TestValidate_StagePromptForbidsDelimiter(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Actions = []Action{{
		Name: "test", Cmd: "echo", Matches: "root",
		Stages: []StageConfig{{Type: "prompt", Key: "x", Text: "Name:", Delimiter: "|"}},
	}}
	if err := cfg.Validate(); err == nil {
		t.Error("expected error for prompt stage with delimiter")
	}
}

func TestValidate_StagePromptForbidsDisplay(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Actions = []Action{{
		Name: "test", Cmd: "echo", Matches: "root",
		Stages: []StageConfig{{Type: "prompt", Key: "x", Text: "Name:", Display: 1}},
	}}
	if err := cfg.Validate(); err == nil {
		t.Error("expected error for prompt stage with display")
	}
}

func TestValidate_StagePromptForbidsPass(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Actions = []Action{{
		Name: "test", Cmd: "echo", Matches: "root",
		Stages: []StageConfig{{Type: "prompt", Key: "x", Text: "Name:", Pass: 1}},
	}}
	if err := cfg.Validate(); err == nil {
		t.Error("expected error for prompt stage with pass")
	}
}

func TestValidate_StageInvalidType(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Actions = []Action{{
		Name: "test", Cmd: "echo", Matches: "root",
		Stages: []StageConfig{{Type: "invalid", Key: "x"}},
	}}
	if err := cfg.Validate(); err == nil {
		t.Error("expected error for invalid stage type")
	}
}

func TestValidate_StageDuplicateKeys(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Actions = []Action{{
		Name: "test", Cmd: "echo", Matches: "root",
		Stages: []StageConfig{
			{Type: "prompt", Key: "name", Text: "Enter name"},
			{Type: "prompt", Key: "name", Text: "Enter name again"},
		},
	}}
	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected error for duplicate stage keys")
	}
	if !strings.Contains(err.Error(), "duplicate") {
		t.Errorf("error = %q, want to contain 'duplicate'", err.Error())
	}
}

func TestValidate_StageReservedKey_Path(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Actions = []Action{{
		Name: "test", Cmd: "echo", Matches: "root",
		Stages: []StageConfig{{Type: "prompt", Key: "path", Text: "Enter"}},
	}}
	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected error for reserved key 'path'")
	}
	if !strings.Contains(err.Error(), "reserved") {
		t.Errorf("error = %q, want to contain 'reserved'", err.Error())
	}
}

func TestValidate_StageReservedKey_PaneID(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Actions = []Action{{
		Name: "test", Cmd: "echo", Matches: "root",
		Stages: []StageConfig{{Type: "prompt", Key: "pane_id", Text: "Enter"}},
	}}
	if err := cfg.Validate(); err == nil {
		t.Error("expected error for reserved key 'pane_id'")
	}
}

func TestValidate_StageReservedKey_Session(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Actions = []Action{{
		Name: "test", Cmd: "echo", Matches: "root",
		Stages: []StageConfig{{Type: "prompt", Key: "session", Text: "Enter"}},
	}}
	if err := cfg.Validate(); err == nil {
		t.Error("expected error for reserved key 'session'")
	}
}

func TestValidate_StageReservedKey_WindowIndex(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Actions = []Action{{
		Name: "test", Cmd: "echo", Matches: "root",
		Stages: []StageConfig{{Type: "prompt", Key: "window_index", Text: "Enter"}},
	}}
	if err := cfg.Validate(); err == nil {
		t.Error("expected error for reserved key 'window_index'")
	}
}

func TestValidate_StageDuplicateKeys_CaseInsensitive(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Actions = []Action{{
		Name: "test", Cmd: "echo", Matches: "root",
		Stages: []StageConfig{
			{Type: "prompt", Key: "name", Text: "Enter name"},
			{Type: "prompt", Key: "Name", Text: "Enter Name"},
		},
	}}
	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected error for case-insensitive duplicate stage keys")
	}
	if !strings.Contains(err.Error(), "duplicate") {
		t.Errorf("error = %q, want to contain 'duplicate'", err.Error())
	}
}

func TestValidate_StageReservedKey_CaseInsensitive(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Actions = []Action{{
		Name: "test", Cmd: "echo", Matches: "root",
		Stages: []StageConfig{{Type: "prompt", Key: "Path", Text: "Enter"}},
	}}
	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected error for reserved key 'Path' (case-insensitive)")
	}
	if !strings.Contains(err.Error(), "reserved") {
		t.Errorf("error = %q, want to contain 'reserved'", err.Error())
	}
}

func TestValidate_StageEmptyKey(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Actions = []Action{{
		Name: "test", Cmd: "echo", Matches: "root",
		Stages: []StageConfig{{Type: "prompt", Key: "", Text: "Enter"}},
	}}
	if err := cfg.Validate(); err == nil {
		t.Error("expected error for empty stage key")
	}
}

func TestValidate_StageKeyWithHyphen(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Actions = []Action{{
		Name: "test", Cmd: "echo", Matches: "root",
		Stages: []StageConfig{{Type: "prompt", Key: "branch-name", Text: "Enter"}},
	}}
	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected error for key with hyphen")
	}
	if !strings.Contains(err.Error(), "valid identifier") {
		t.Errorf("error = %q, want to contain 'valid identifier'", err.Error())
	}
}

func TestValidate_StageKeyStartsWithDigit(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Actions = []Action{{
		Name: "test", Cmd: "echo", Matches: "root",
		Stages: []StageConfig{{Type: "prompt", Key: "1name", Text: "Enter"}},
	}}
	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected error for key starting with digit")
	}
	if !strings.Contains(err.Error(), "valid identifier") {
		t.Errorf("error = %q, want to contain 'valid identifier'", err.Error())
	}
}

func TestLoad_ValidTOML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(path, []byte(`
[timeout]
fetch = "1500ms"

[[actions]]
name = "htop"
cmd = "htop"
matches = "root"

[[actions]]
name = "logs"
cmd = "tail -f /var/log/syslog"
matches = "root"
`), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cfg.Actions) != 2 {
		t.Fatalf("got %d actions, want 2", len(cfg.Actions))
	}
	if cfg.Actions[0].Name != "htop" {
		t.Errorf("actions[0].Name = %q, want %q", cfg.Actions[0].Name, "htop")
	}
	if cfg.Actions[1].Cmd != "tail -f /var/log/syslog" {
		t.Errorf("actions[1].Cmd = %q", cfg.Actions[1].Cmd)
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
	defaults := DefaultConfig()
	if cfg.Timeout.Fetch != defaults.Timeout.Fetch {
		t.Errorf("Timeout.Fetch = %s, want %s (default)", cfg.Timeout.Fetch, defaults.Timeout.Fetch)
	}
	if cfg.Behavior.WrapList != defaults.Behavior.WrapList {
		t.Errorf("Behavior.WrapList = %v, want %v (default)", cfg.Behavior.WrapList, defaults.Behavior.WrapList)
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
	if cfg.Timeout.Fetch != 2*time.Second {
		t.Errorf("Timeout.Fetch = %s, want 2s (default preserved)", cfg.Timeout.Fetch)
	}
}

func TestLoad_PreservesOrder(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(path, []byte(`
[[actions]]
name = "alpha"
cmd = "echo alpha"
matches = "root"

[[actions]]
name = "beta"
cmd = "echo beta"
matches = "root"

[[actions]]
name = "gamma"
cmd = "echo gamma"
matches = "root"
`), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cfg.Actions) != 3 {
		t.Fatalf("got %d actions, want 3", len(cfg.Actions))
	}
	want := []string{"alpha", "beta", "gamma"}
	for i, w := range want {
		if cfg.Actions[i].Name != w {
			t.Errorf("actions[%d].Name = %q, want %q", i, cfg.Actions[i].Name, w)
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

func TestValidate_NegativePickerTimeout(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Timeout.Picker = -1 * time.Second
	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected error for negative picker timeout")
	}
	if !strings.Contains(err.Error(), "timeout.picker") {
		t.Errorf("error = %q, want to contain 'timeout.picker'", err.Error())
	}
}

func TestValidate_SuspiciouslySmallPickerTimeout(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Timeout.Picker = 500 * time.Nanosecond
	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected error for sub-millisecond picker timeout")
	}
	if !strings.Contains(err.Error(), "timeout.picker") {
		t.Errorf("error = %q, want to contain 'timeout.picker'", err.Error())
	}
}

func TestValidate_ZeroPickerTimeout(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Timeout.Picker = 0
	if err := cfg.Validate(); err != nil {
		t.Errorf("zero picker timeout should be valid: %v", err)
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
	if cfg.Display.ShortenHome != "~" {
		t.Errorf("ShortenHome = %q, want default %q", cfg.Display.ShortenHome, "~")
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
	if cfg.Display.ShortenHome != "" {
		t.Errorf("ShortenHome = %q, want empty string", cfg.Display.ShortenHome)
	}
}

func TestValidate_EmptyDisplayRuleKey(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Display.Rules = map[string]string{"": "bad"}
	if err := cfg.Validate(); err == nil {
		t.Error("expected error for empty rule key")
	}
}

func TestLoad_DisplayTruncation(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(path, []byte(`
[display]
truncation_length = 3
truncation_symbol = "…"
`), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Display.TruncationLength != 3 {
		t.Errorf("TruncationLength = %d, want 3", cfg.Display.TruncationLength)
	}
	if cfg.Display.TruncationSymbol != "…" {
		t.Errorf("TruncationSymbol = %q, want %q", cfg.Display.TruncationSymbol, "…")
	}
}

func TestLoad_DisplayTruncationDefaults(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(path, []byte(`
[display]
shorten_home = "~"
`), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Display.TruncationLength != 0 {
		t.Errorf("TruncationLength = %d, want 0 (default)", cfg.Display.TruncationLength)
	}
	if cfg.Display.TruncationSymbol != "" {
		t.Errorf("TruncationSymbol = %q, want empty (default)", cfg.Display.TruncationSymbol)
	}
}

func TestValidate_NegativeTruncationLength(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Display.TruncationLength = -1
	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected error for negative truncation_length")
	}
	if !strings.Contains(err.Error(), "truncation_length") {
		t.Errorf("error = %q, want mention of truncation_length", err.Error())
	}
}

func TestValidate_ValidIconAlias(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Actions = []Action{{Name: "GitHub", Cmd: "open gh", Matches: "root", Icon: ":nf-dev-github:"}}
	if err := cfg.Validate(); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidate_InvalidIconAlias(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Actions = []Action{{Name: "test", Cmd: "test", Matches: "root", Icon: ":nf-fake-thing:"}}
	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected error for invalid icon alias")
	}
	if !strings.Contains(err.Error(), "actions[0].icon") {
		t.Errorf("error = %q, want prefix actions[0].icon", err.Error())
	}
}

func TestValidate_ValidIconRawUnicode(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Actions = []Action{{Name: "test", Cmd: "test", Matches: "root", Icon: "\ue709"}}
	if err := cfg.Validate(); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidate_InvalidIconMultiChar(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Actions = []Action{{Name: "test", Cmd: "test", Matches: "root", Icon: "ab"}}
	if err := cfg.Validate(); err == nil {
		t.Error("expected error for multi-character icon")
	}
}

func TestValidate_EmptyIconOK(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Actions = []Action{{Name: "test", Cmd: "test", Matches: "root", Icon: ""}}
	if err := cfg.Validate(); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidate_DirActionIconAlias(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Actions = []Action{{Name: "Yazi", Cmd: "yazi", Matches: "dir", Icon: ":nf-cod-folder:"}}
	if err := cfg.Validate(); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestLoad_ActionWithIconAlias(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(path, []byte(`
[[actions]]
name = "GitHub"
cmd = "open https://github.com"
matches = "root"
icon = ":nf-dev-github:"
`), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Actions[0].Icon != "\ue709" {
		t.Errorf("Icon = %q, want resolved unicode \\ue709", cfg.Actions[0].Icon)
	}
}

func TestLoad_ActionWithUnicodeIcon(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(path, []byte("[[actions]]\nname = \"test\"\ncmd = \"test\"\nmatches = \"root\"\nicon = \"\ue709\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Actions[0].Icon != "\ue709" {
		t.Errorf("Icon = %q, want \\ue709", cfg.Actions[0].Icon)
	}
}

func TestLoad_BehaviorWrapListDefault(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(path, []byte(`
[[actions]]
name = "htop"
cmd = "htop"
matches = "root"
`), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !cfg.Behavior.WrapList {
		t.Error("Behavior.WrapList = false, want true (default)")
	}
}

func TestLoad_BehaviorWrapListDisabled(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(path, []byte(`
[behavior]
wrap_list = false
`), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Behavior.WrapList {
		t.Error("Behavior.WrapList = true, want false")
	}
}

func TestLoad_BehaviorStartInFilterDisabled(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(path, []byte(`
[behavior]
start_in_filter = false
`), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Behavior.StartInFilter {
		t.Error("Behavior.StartInFilter = true, want false")
	}
}

func TestLoad_BehaviorBellToTopDefault(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(path, []byte(`
[[actions]]
name = "htop"
cmd = "htop"
matches = "root"
`), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !cfg.Behavior.BellToTop {
		t.Error("Behavior.BellToTop = false, want true (default)")
	}
}

func TestLoad_BehaviorBellToTopDisabled(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(path, []byte(`
[behavior]
bell_to_top = false
`), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Behavior.BellToTop {
		t.Error("Behavior.BellToTop = true, want false")
	}
}

func TestLoad_DirActionWithIcon(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(path, []byte(`
[[actions]]
name = "Browse"
cmd = "yazi"
matches = "dir"
icon = ":nf-cod-folder_opened:"
`), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Actions[0].Icon != "\ueaf7" {
		t.Errorf("Icon = %q, want resolved unicode \\ueaf7", cfg.Actions[0].Icon)
	}
}

func TestLoad_Actions(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(path, []byte(`
[[actions]]
name = "Yazi"
cmd = "tmux split-window -h yazi"
matches = "dir"

[[actions]]
name = "New pane"
cmd = "tmux split-window -v"
matches = "dir"
`), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cfg.Actions) != 2 {
		t.Fatalf("got %d actions, want 2", len(cfg.Actions))
	}
	if cfg.Actions[0].Name != "Yazi" {
		t.Errorf("actions[0].Name = %q, want %q", cfg.Actions[0].Name, "Yazi")
	}
	if cfg.Actions[1].Cmd != "tmux split-window -v" {
		t.Errorf("actions[1].Cmd = %q", cfg.Actions[1].Cmd)
	}
}

func TestLoad_NoActions(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(path, []byte(`
[timeout]
fetch = "2s"
`), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cfg.Actions) != 0 {
		t.Errorf("got %d actions, want 0", len(cfg.Actions))
	}
}

func TestLoad_MixedMatchTypes(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(path, []byte(`
[[actions]]
name = "htop"
cmd = "htop"
matches = "root"

[[actions]]
name = "Yazi"
cmd = "yazi"
matches = "dir"
`), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cfg.Actions) != 2 {
		t.Fatalf("got %d actions, want 2", len(cfg.Actions))
	}
	if cfg.Actions[0].Matches != "root" {
		t.Errorf("actions[0].Matches = %q, want root", cfg.Actions[0].Matches)
	}
	if cfg.Actions[1].Matches != "dir" {
		t.Errorf("actions[1].Matches = %q, want dir", cfg.Actions[1].Matches)
	}
}

func TestLoad_BehaviorAutoSelectSingle(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(path, []byte(`
[behavior]
auto_select_single = false
`), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Behavior.AutoSelectSingle {
		t.Error("Behavior.AutoSelectSingle = true, want false")
	}
}

func TestLoad_ActionWithStages(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(path, []byte(`
[[actions]]
name = "New session"
cmd = "tmux new-session -s {{.session_name}}"
matches = "root"

[[actions.stages]]
type = "prompt"
key = "session_name"
text = "Session name"
default = "dev"
`), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cfg.Actions) != 1 {
		t.Fatalf("got %d actions, want 1", len(cfg.Actions))
	}
	if len(cfg.Actions[0].Stages) != 1 {
		t.Fatalf("got %d stages, want 1", len(cfg.Actions[0].Stages))
	}
	s := cfg.Actions[0].Stages[0]
	if s.Type != "prompt" {
		t.Errorf("stage.Type = %q, want prompt", s.Type)
	}
	if s.Key != "session_name" {
		t.Errorf("stage.Key = %q, want session_name", s.Key)
	}
	if s.Text != "Session name" {
		t.Errorf("stage.Text = %q, want Session name", s.Text)
	}
	if s.Default != "dev" {
		t.Errorf("stage.Default = %q, want dev", s.Default)
	}
}
