package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jmcampanini/go-config-loader/configloader"

	"github.com/jmcampanini/cmdk/internal/theme"
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
	if cfg.Display.TmuxSessionTruncationLength != 2 {
		t.Errorf("Display.TmuxSessionTruncationLength = %d, want 2", cfg.Display.TmuxSessionTruncationLength)
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
	if cfg.Behavior.WindowNameMaxLength != 20 {
		t.Errorf("Behavior.WindowNameMaxLength = %d, want 20", cfg.Behavior.WindowNameMaxLength)
	}
	if cfg.Startup.Path != "" {
		t.Errorf("Startup.Path = %q, want empty", cfg.Startup.Path)
	}
	if len(cfg.Theme) != 0 {
		t.Errorf("Theme = %d overrides, want 0", len(cfg.Theme))
	}
}

func TestValidate_StartupPathAllowsNormalPath(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Startup.Path = "~/Code/project"
	if err := cfg.Validate(); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidate_StartupPathRejectsControlCharacters(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Startup.Path = "/tmp/bad\npath"
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected error for startup path with control character")
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

func TestValidate_MutationTimeout(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Timeout.Mutation = -1 * time.Second
	if err := cfg.Validate(); err == nil {
		t.Error("expected error for negative mutation timeout")
	}

	cfg = DefaultConfig()
	cfg.Timeout.Mutation = 500 * time.Microsecond
	if err := cfg.Validate(); err == nil {
		t.Error("expected error for sub-millisecond mutation timeout")
	}

	cfg = DefaultConfig()
	cfg.Timeout.Mutation = 0
	if err := cfg.Validate(); err != nil {
		t.Errorf("zero mutation timeout should be valid (means default): %v", err)
	}
}

func TestTimeout_EffectiveDefaults(t *testing.T) {
	var zero Timeout
	if got := zero.EffectiveFetch(); got != 2*time.Second {
		t.Errorf("EffectiveFetch() = %s for zero value, want 2s default", got)
	}
	if got := zero.EffectiveMutation(); got != 5*time.Second {
		t.Errorf("EffectiveMutation() = %s for zero value, want 5s default", got)
	}

	set := Timeout{Fetch: 7 * time.Second, Mutation: 9 * time.Second}
	if got := set.EffectiveFetch(); got != 7*time.Second {
		t.Errorf("EffectiveFetch() = %s, want configured 7s", got)
	}
	if got := set.EffectiveMutation(); got != 9*time.Second {
		t.Errorf("EffectiveMutation() = %s, want configured 9s", got)
	}

	negative := Timeout{Fetch: -time.Second, Mutation: -time.Second}
	if got := negative.EffectiveFetch(); got != 2*time.Second {
		t.Errorf("EffectiveFetch() = %s for negative value, want 2s default", got)
	}
	if got := negative.EffectiveMutation(); got != 5*time.Second {
		t.Errorf("EffectiveMutation() = %s for negative value, want 5s default", got)
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

func TestValidate_NegativeWindowNameMaxLength(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Behavior.WindowNameMaxLength = -1
	if err := cfg.Validate(); err == nil {
		t.Error("expected error for negative window_name_max_length")
	}
}

func TestValidate_ZeroWindowNameMaxLength(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Behavior.WindowNameMaxLength = 0
	if err := cfg.Validate(); err != nil {
		t.Errorf("zero window_name_max_length should be valid: %v", err)
	}
}

func TestValidate_ThemeOverrides(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Theme = theme.Config{
		theme.NameDark: {
			Accent: "#ca9ee6",
			Roles:  theme.RoleConfig{SessionIcon: "#81c8be"},
		},
	}
	if err := cfg.Validate(); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidate_InvalidThemeMode(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Theme = theme.Config{"frappe": {Accent: "#ca9ee6"}}
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected error for invalid theme mode")
	}
}

func TestValidate_InvalidThemeColor(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Theme = theme.Config{theme.NameDark: {Accent: "ca9ee6"}}
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected error for invalid theme color")
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

func TestValidate_ActionMatchesSession(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Actions = []Action{{Name: "Rename", Cmd: "tmux rename-session", Matches: "session"}}
	if err := cfg.Validate(); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidate_LaunchModeValues(t *testing.T) {
	tests := []string{"", "detect", "session-window", "shell"}
	for _, mode := range tests {
		cfg := DefaultConfig()
		cfg.Actions = []Action{{Name: "a", Cmd: "echo", Matches: "root", LaunchMode: mode}}
		if err := cfg.Validate(); err != nil {
			t.Errorf("LaunchMode %q unexpected error: %v", mode, err)
		}
	}
}

func TestValidate_InvalidLaunchMode(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Actions = []Action{{Name: "bad", Cmd: "echo", Matches: "root", LaunchMode: "background"}}
	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected error for invalid launch_mode")
	}
	if !strings.Contains(err.Error(), "launch_mode") {
		t.Errorf("error = %q, want launch_mode", err.Error())
	}
}

func TestValidate_LaunchPathAndCmdMutuallyExclusive(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Actions = []Action{{Name: "bad", Cmd: "echo", Matches: "dir", LaunchPath: "~/x", LaunchPathCmd: "pwd"}}
	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected error for launch_path + launch_path_cmd")
	}
	if !strings.Contains(err.Error(), "launch_path") || !strings.Contains(err.Error(), "launch_path_cmd") {
		t.Errorf("error = %q, want both field names", err.Error())
	}
}

func TestValidate_WindowNameRejectedForEffectiveShell(t *testing.T) {
	tests := []Action{
		{Name: "root detect", Cmd: "echo", Matches: "root", WindowName: "x"},
		{Name: "explicit shell", Cmd: "echo", Matches: "dir", LaunchMode: "shell", WindowName: "x"},
	}
	for _, action := range tests {
		cfg := DefaultConfig()
		cfg.Actions = []Action{action}
		err := cfg.Validate()
		if err == nil {
			t.Fatalf("%s: expected error", action.Name)
		}
		if !strings.Contains(err.Error(), "window_name") {
			t.Errorf("%s: error = %q, want window_name", action.Name, err.Error())
		}
	}
}

func TestValidate_WindowNameAllowedForSessionWindow(t *testing.T) {
	tests := []Action{
		{Name: "dir detect", Cmd: "echo", Matches: "dir", WindowName: "x"},
		{Name: "root path", Cmd: "echo", Matches: "root", LaunchPath: "/tmp", WindowName: "x"},
		{Name: "root explicit", Cmd: "echo", Matches: "root", LaunchMode: "session-window", WindowName: "x"},
	}
	for _, action := range tests {
		cfg := DefaultConfig()
		cfg.Actions = []Action{action}
		if err := cfg.Validate(); err != nil {
			t.Errorf("%s: unexpected error: %v", action.Name, err)
		}
	}
}

func TestValidate_StageReservedKey_LaunchPath(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Actions = []Action{{
		Name: "test", Cmd: "echo", Matches: "root",
		Stages: []StageConfig{{Type: "prompt", Key: "launch_path", Text: "Launch path"}},
	}}
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected error for reserved launch_path stage key")
	}
}

func TestValidate_StageReservedKey_LaunchBasename(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Actions = []Action{{
		Name: "test", Cmd: "echo", Matches: "root",
		Stages: []StageConfig{{Type: "prompt", Key: "launch_basename", Text: "Launch basename"}},
	}}
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected error for reserved launch_basename stage key")
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

func TestValidate_StageKey_SessionIsNotReserved(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Actions = []Action{{
		Name: "test", Cmd: "echo", Matches: "root",
		Stages: []StageConfig{{Type: "prompt", Key: "session", Text: "Enter"}},
	}}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("unexpected error for unreserved key 'session': %v", err)
	}
}

func TestValidate_StageReservedKey_SessionName(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Actions = []Action{{
		Name: "test", Cmd: "echo", Matches: "root",
		Stages: []StageConfig{{Type: "prompt", Key: "session_name", Text: "Enter"}},
	}}
	if err := cfg.Validate(); err == nil {
		t.Error("expected error for reserved key 'session_name'")
	}
}

func TestValidate_StageReservedKey_SessionID(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Actions = []Action{{
		Name: "test", Cmd: "echo", Matches: "root",
		Stages: []StageConfig{{Type: "prompt", Key: "session_id", Text: "Enter"}},
	}}
	if err := cfg.Validate(); err == nil {
		t.Error("expected error for reserved key 'session_id'")
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

func TestValidate_StageReservedKey_SessionNameForSessionAction(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Actions = []Action{{
		Name: "test", Cmd: "echo", Matches: "session",
		Stages: []StageConfig{{Type: "prompt", Key: "session_name", Text: "Enter"}},
	}}
	if err := cfg.Validate(); err == nil {
		t.Error("expected error for reserved key 'session_name' on session action")
	}
}

func TestValidate_StageReservedKey_WindowID(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Actions = []Action{{
		Name: "test", Cmd: "echo", Matches: "root",
		Stages: []StageConfig{{Type: "prompt", Key: "window_id", Text: "Enter"}},
	}}
	if err := cfg.Validate(); err == nil {
		t.Error("expected error for reserved key 'window_id'")
	}
}

func TestValidate_StageReservedKey_WindowName(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Actions = []Action{{
		Name: "test", Cmd: "echo", Matches: "root",
		Stages: []StageConfig{{Type: "prompt", Key: "window_name", Text: "Enter"}},
	}}
	if err := cfg.Validate(); err == nil {
		t.Error("expected error for reserved key 'window_name'")
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

func TestValidate_StagePromptAllowEmpty(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Actions = []Action{{
		Name: "test", Cmd: "echo", Matches: "root",
		Stages: []StageConfig{{Type: "prompt", Key: "msg", Text: "Message:", AllowEmpty: true}},
	}}
	if err := cfg.Validate(); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidate_StagePickerForbidsAllowEmpty(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Actions = []Action{{
		Name: "test", Cmd: "echo", Matches: "root",
		Stages: []StageConfig{{Type: "picker", Key: "dir", Source: "zoxide", AllowEmpty: true}},
	}}
	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected error for picker stage with allow_empty")
	}
	if !strings.Contains(err.Error(), "allow_empty") {
		t.Errorf("error = %q, want to contain 'allow_empty'", err.Error())
	}
}

func TestLoad_MutationTimeoutFromTOML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(path, []byte(`
[timeout]
mutation = "7s"
`), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Timeout.Mutation != 7*time.Second {
		t.Errorf("Timeout.Mutation = %s, want configured 7s", cfg.Timeout.Mutation)
	}
	if cfg.Timeout.EffectiveMutation() != 7*time.Second {
		t.Errorf("EffectiveMutation() = %s, want configured 7s", cfg.Timeout.EffectiveMutation())
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

func TestLoad_ThemeOverridesFromTOML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(path, []byte(`
[theme.dark]
accent = "#ca9ee6"
match_bg = "#5b4b8a"

[theme.dark.roles]
session_icon = "#81c8be"
`), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Theme[theme.NameDark].Accent != "#ca9ee6" {
		t.Errorf("theme.dark.accent = %q, want #ca9ee6", cfg.Theme[theme.NameDark].Accent)
	}
	if cfg.Theme[theme.NameDark].Roles.SessionIcon != "#81c8be" {
		t.Errorf("theme.dark.roles.session_icon = %q, want #81c8be", cfg.Theme[theme.NameDark].Roles.SessionIcon)
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

func TestLoad_DirectoryPathReturnsError(t *testing.T) {
	cfg, err := Load(t.TempDir())
	if err == nil {
		t.Fatal("expected error for directory config path")
	}
	defaults := DefaultConfig()
	if cfg.Timeout.Fetch != defaults.Timeout.Fetch {
		t.Errorf("Timeout.Fetch = %s, want default %s", cfg.Timeout.Fetch, defaults.Timeout.Fetch)
	}
}

func TestValidateFile_RejectsNonRegularPath(t *testing.T) {
	path := os.DevNull
	if _, err := os.Stat(path); err != nil {
		t.Skipf("os.DevNull is not available: %v", err)
	}
	if err := ValidateFile(path); err == nil {
		t.Fatal("ValidateFile(os.DevNull) error = nil, want non-regular file error")
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

func TestLoadWithReport_TracksFileProvenance(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(path, []byte(`
[timeout]
fetch = "3s"
`), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, report, err := LoadWithReport(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Timeout.Fetch != 3*time.Second {
		t.Errorf("Timeout.Fetch = %s, want 3s", cfg.Timeout.Fetch)
	}
	absPath, err := filepath.Abs(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := report.Updates["timeout.fetch"]; got != absPath {
		t.Errorf("report.Updates[timeout.fetch] = %q, want %q", got, absPath)
	}
	if len(report.LoadedFiles) != 1 || report.LoadedFiles[0] != absPath {
		t.Errorf("report.LoadedFiles = %v, want [%q]", report.LoadedFiles, absPath)
	}
}

func TestLoadWithReport_IgnoresConfigEnvOverrides(t *testing.T) {
	t.Setenv("CMDK_FETCH_TIMEOUT", "5s")
	t.Setenv("CMDK_WRAP_LIST", "false")

	cfg, report, err := LoadWithReport("/nonexistent/path/config.toml")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defaults := DefaultConfig()
	if cfg.Timeout.Fetch != defaults.Timeout.Fetch {
		t.Errorf("Timeout.Fetch = %s, want default %s", cfg.Timeout.Fetch, defaults.Timeout.Fetch)
	}
	if cfg.Behavior.WrapList != defaults.Behavior.WrapList {
		t.Errorf("Behavior.WrapList = %v, want default %v", cfg.Behavior.WrapList, defaults.Behavior.WrapList)
	}
	if got := report.Updates["timeout.fetch"]; got != configloader.SourceDefault {
		t.Errorf("report.Updates[timeout.fetch] = %q, want %q", got, configloader.SourceDefault)
	}
	if got := report.Updates["behavior.wraplist"]; got != configloader.SourceDefault {
		t.Errorf("report.Updates[behavior.wraplist] = %q, want %q", got, configloader.SourceDefault)
	}
}

func TestValidateFile_MissingFile(t *testing.T) {
	if err := ValidateFile("/nonexistent/path/config.toml"); err == nil {
		t.Fatal("ValidateFile() error = nil, want missing file error")
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

func TestLoad_TmuxSessionDisplayTruncation(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(path, []byte(`
[display]
tmux_session_truncation_length = 3
tmux_session_truncation_symbol = "…"
`), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Display.TmuxSessionTruncationLength != 3 {
		t.Errorf("TmuxSessionTruncationLength = %d, want 3", cfg.Display.TmuxSessionTruncationLength)
	}
	if cfg.Display.TmuxSessionTruncationSymbol != "…" {
		t.Errorf("TmuxSessionTruncationSymbol = %q, want %q", cfg.Display.TmuxSessionTruncationSymbol, "…")
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
	if cfg.Display.TmuxSessionTruncationLength != 2 {
		t.Errorf("TmuxSessionTruncationLength = %d, want 2 (default)", cfg.Display.TmuxSessionTruncationLength)
	}
	if cfg.Display.TmuxSessionTruncationSymbol != "" {
		t.Errorf("TmuxSessionTruncationSymbol = %q, want empty (default)", cfg.Display.TmuxSessionTruncationSymbol)
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

func TestValidate_NegativeTmuxSessionTruncationLength(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Display.TmuxSessionTruncationLength = -1
	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected error for negative tmux_session_truncation_length")
	}
	if !strings.Contains(err.Error(), "tmux_session_truncation_length") {
		t.Errorf("error = %q, want mention of tmux_session_truncation_length", err.Error())
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

func TestLoad_DisplayInlineIconResolution(t *testing.T) {
	tests := []struct {
		name  string
		toml  string
		check func(t *testing.T, cfg Config)
	}{
		{
			name: "shorten_home alias",
			toml: "[display]\nshorten_home = \":nf-oct-home:\"",
			check: func(t *testing.T, cfg Config) {
				if cfg.Display.ShortenHome != "\uf46d" {
					t.Errorf("ShortenHome = %q, want \\uf46d", cfg.Display.ShortenHome)
				}
			},
		},
		{
			name: "truncation_symbol alias",
			toml: "[display]\ntruncation_symbol = \":nf-cod-ellipsis:\"",
			check: func(t *testing.T, cfg Config) {
				if cfg.Display.TruncationSymbol != "\uea7c" {
					t.Errorf("TruncationSymbol = %q, want \\uea7c", cfg.Display.TruncationSymbol)
				}
			},
		},
		{
			name: "tmux_session_truncation_symbol alias",
			toml: "[display]\ntmux_session_truncation_symbol = \":nf-cod-ellipsis:\"",
			check: func(t *testing.T, cfg Config) {
				if cfg.Display.TmuxSessionTruncationSymbol != "\uea7c" {
					t.Errorf("TmuxSessionTruncationSymbol = %q, want \\uea7c", cfg.Display.TmuxSessionTruncationSymbol)
				}
			},
		},
		{
			name: "rule value alias with text",
			toml: "[display.rules]\n\"github.com\" = \":nf-dev-github:gh\"",
			check: func(t *testing.T, cfg Config) {
				if cfg.Display.Rules["github.com"] != "\ue709gh" {
					t.Errorf("rule = %q, want %q", cfg.Display.Rules["github.com"], "\ue709gh")
				}
			},
		},
		{
			name: "rule value alias only",
			toml: "[display.rules]\n\"github.com\" = \":nf-dev-github:\"",
			check: func(t *testing.T, cfg Config) {
				if cfg.Display.Rules["github.com"] != "\ue709" {
					t.Errorf("rule = %q, want \\ue709", cfg.Display.Rules["github.com"])
				}
			},
		},
		{
			name: "rule key not resolved",
			toml: "[display.rules]\n\":nf-dev-github:\" = \"gh\"",
			check: func(t *testing.T, cfg Config) {
				if _, ok := cfg.Display.Rules[":nf-dev-github:"]; !ok {
					t.Error("rule key should remain as literal :nf-dev-github:")
				}
			},
		},
		{
			name: "rule with empty value skipped",
			toml: "[display.rules]\n\"foo\" = \"\"",
			check: func(t *testing.T, cfg Config) {
				if cfg.Display.Rules["foo"] != "" {
					t.Errorf("empty rule value should remain empty, got %q", cfg.Display.Rules["foo"])
				}
			},
		},
		{
			name: "adjacent aliases in rule",
			toml: "[display.rules]\n\"github.com\" = \":nf-dev-github::nf-cod-folder_opened:\"",
			check: func(t *testing.T, cfg Config) {
				want := "\ue709\ueaf7"
				if cfg.Display.Rules["github.com"] != want {
					t.Errorf("rule = %q, want %q", cfg.Display.Rules["github.com"], want)
				}
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "config.toml")
			if err := os.WriteFile(path, []byte(tt.toml), 0o644); err != nil {
				t.Fatal(err)
			}
			cfg, err := Load(path)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			tt.check(t, cfg)
		})
	}
}

func TestLoad_DisplayInvalidInlineIcon(t *testing.T) {
	tests := []struct {
		name      string
		toml      string
		wantInErr string
	}{
		{
			name:      "invalid shorten_home alias",
			toml:      "[display]\nshorten_home = \":nf-fake:\"",
			wantInErr: "display.shorten_home",
		},
		{
			name:      "invalid truncation_symbol alias",
			toml:      "[display]\ntruncation_symbol = \":nf-fake:\"",
			wantInErr: "display.truncation_symbol",
		},
		{
			name:      "invalid tmux_session_truncation_symbol alias",
			toml:      "[display]\ntmux_session_truncation_symbol = \":nf-fake:\"",
			wantInErr: "display.tmux_session_truncation_symbol",
		},
		{
			name:      "invalid rule alias",
			toml:      "[display.rules]\n\"gh\" = \":nf-fake:\"",
			wantInErr: "display.rules",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "config.toml")
			if err := os.WriteFile(path, []byte(tt.toml), 0o644); err != nil {
				t.Fatal(err)
			}
			_, err := Load(path)
			if err == nil {
				t.Fatal("expected error")
			}
			if !strings.Contains(err.Error(), tt.wantInErr) {
				t.Errorf("error = %q, want to contain %q", err.Error(), tt.wantInErr)
			}
		})
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
cmd = "tmux new-session -s {{.new_session_name}}"
matches = "root"

[[actions.stages]]
type = "prompt"
key = "new_session_name"
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
	if s.Key != "new_session_name" {
		t.Errorf("stage.Key = %q, want new_session_name", s.Key)
	}
	if s.Text != "Session name" {
		t.Errorf("stage.Text = %q, want Session name", s.Text)
	}
	if s.Default != "dev" {
		t.Errorf("stage.Default = %q, want dev", s.Default)
	}
}

func TestLoad_ActionWithStages_AllowEmpty(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(path, []byte(`
[[actions]]
name = "Commit"
cmd = "git commit -m {{.msg}}"
matches = "root"

[[actions.stages]]
type = "prompt"
key = "msg"
text = "Message:"
allow_empty = true
`), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	s := cfg.Actions[0].Stages[0]
	if !s.AllowEmpty {
		t.Error("stage.AllowEmpty = false, want true")
	}
}

func TestLoad_ActionWithStages_AllowEmptyDefault(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(path, []byte(`
[[actions]]
name = "Test"
cmd = "echo {{.name}}"
matches = "root"

[[actions.stages]]
type = "prompt"
key = "name"
text = "Name:"
`), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	s := cfg.Actions[0].Stages[0]
	if s.AllowEmpty {
		t.Error("stage.AllowEmpty = true, want false (default)")
	}
}
