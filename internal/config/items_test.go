package config

import (
	"context"
	"testing"

	"github.com/jmcampanini/cmdk/internal/item"
)

func TestMatchingActions_EmptyConfig(t *testing.T) {
	fn := MatchingActions(&Config{}, "root")
	items, err := fn(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 0 {
		t.Fatalf("expected 0 items, got %d", len(items))
	}
}

func TestMatchingActions_CorrectFields(t *testing.T) {
	cfg := &Config{
		Actions: []Action{
			{Name: "htop", Cmd: "htop", Matches: "root"},
		},
	}
	fn := MatchingActions(cfg, "root")
	items, err := fn(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("got %d items, want 1", len(items))
	}
	it := items[0]
	if it.Type != "action" {
		t.Errorf("Type = %q, want %q", it.Type, "action")
	}
	if it.Source != "config" {
		t.Errorf("Source = %q, want %q", it.Source, "config")
	}
	if it.Action != item.ActionExecute {
		t.Errorf("Action = %q, want %q", it.Action, item.ActionExecute)
	}
	if it.Cmd != "htop" {
		t.Errorf("Cmd = %q, want %q", it.Cmd, "htop")
	}
	if it.Display != "htop" {
		t.Errorf("Display = %q, want %q", it.Display, "htop")
	}
}

func TestMatchingActions_PreservesOrder(t *testing.T) {
	cfg := &Config{
		Actions: []Action{
			{Name: "alpha", Cmd: "echo alpha", Matches: "root"},
			{Name: "beta", Cmd: "echo beta", Matches: "root"},
			{Name: "gamma", Cmd: "echo gamma", Matches: "root"},
		},
	}
	fn := MatchingActions(cfg, "root")
	items, err := fn(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 3 {
		t.Fatalf("got %d items, want 3", len(items))
	}
	want := []string{"alpha", "beta", "gamma"}
	for i, w := range want {
		if items[i].Display != w {
			t.Errorf("items[%d].Display = %q, want %q", i, items[i].Display, w)
		}
	}
}

func TestMatchingActions_IconPassedThrough(t *testing.T) {
	cfg := &Config{
		Actions: []Action{
			{Name: "GitHub", Cmd: "open gh", Matches: "root", Icon: "\ue709"},
		},
	}
	fn := MatchingActions(cfg, "root")
	items, err := fn(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if items[0].Icon != "\ue709" {
		t.Errorf("Icon = %q, want \\ue709", items[0].Icon)
	}
}

func TestMatchingActions_NoIcon(t *testing.T) {
	cfg := &Config{
		Actions: []Action{
			{Name: "htop", Cmd: "htop", Matches: "root"},
		},
	}
	fn := MatchingActions(cfg, "root")
	items, err := fn(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if items[0].Icon != "" {
		t.Errorf("Icon = %q, want empty", items[0].Icon)
	}
}

func TestMatchingActions_FiltersByMatchType(t *testing.T) {
	cfg := &Config{
		Actions: []Action{
			{Name: "htop", Cmd: "htop", Matches: "root"},
			{Name: "Yazi", Cmd: "yazi", Matches: "dir"},
			{Name: "logs", Cmd: "tail -f", Matches: "root"},
		},
	}

	rootFn := MatchingActions(cfg, "root")
	rootItems, err := rootFn(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(rootItems) != 2 {
		t.Fatalf("root: got %d items, want 2", len(rootItems))
	}
	if rootItems[0].Display != "htop" {
		t.Errorf("root items[0].Display = %q, want htop", rootItems[0].Display)
	}
	if rootItems[1].Display != "logs" {
		t.Errorf("root items[1].Display = %q, want logs", rootItems[1].Display)
	}

	dirFn := MatchingActions(cfg, "dir")
	dirItems, err := dirFn(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(dirItems) != 1 {
		t.Fatalf("dir: got %d items, want 1", len(dirItems))
	}
	if dirItems[0].Display != "Yazi" {
		t.Errorf("dir items[0].Display = %q, want Yazi", dirItems[0].Display)
	}
}

func TestMatchingActions_ActionStagedForActionsWithStages(t *testing.T) {
	cfg := &Config{
		Actions: []Action{
			{
				Name: "New session", Cmd: "tmux new-session", Matches: "root",
				Stages: []StageConfig{
					{Type: "prompt", Key: "name", Text: "Session name"},
				},
			},
		},
	}
	fn := MatchingActions(cfg, "root")
	items, err := fn(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("got %d items, want 1", len(items))
	}
	if items[0].Action != item.ActionStaged {
		t.Errorf("Action = %q, want %q", items[0].Action, item.ActionStaged)
	}
}

func TestMatchingActions_StageConversion(t *testing.T) {
	cfg := &Config{
		Actions: []Action{
			{
				Name: "Multi", Cmd: "echo", Matches: "root",
				Stages: []StageConfig{
					{Type: "prompt", Key: "name", Text: "Enter name", Default: "world"},
					{Type: "picker", Key: "dir", Source: "zoxide"},
				},
			},
		},
	}
	fn := MatchingActions(cfg, "root")
	items, err := fn(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items[0].Stages) != 2 {
		t.Fatalf("got %d stages, want 2", len(items[0].Stages))
	}

	s0 := items[0].Stages[0]
	if s0.Type != item.StagePrompt {
		t.Errorf("stages[0].Type = %q, want prompt", s0.Type)
	}
	if s0.Key != "name" {
		t.Errorf("stages[0].Key = %q, want name", s0.Key)
	}
	if s0.Text != "Enter name" {
		t.Errorf("stages[0].Text = %q, want Enter name", s0.Text)
	}
	if s0.Default != "world" {
		t.Errorf("stages[0].Default = %q, want world", s0.Default)
	}

	s1 := items[0].Stages[1]
	if s1.Type != item.StagePicker {
		t.Errorf("stages[1].Type = %q, want picker", s1.Type)
	}
	if s1.Key != "dir" {
		t.Errorf("stages[1].Key = %q, want dir", s1.Key)
	}
	if s1.Source != "zoxide" {
		t.Errorf("stages[1].Source = %q, want zoxide", s1.Source)
	}
}

func TestMatchingActions_StageConversion_FieldConfig(t *testing.T) {
	cfg := &Config{
		Actions: []Action{
			{
				Name: "Fields", Cmd: "echo", Matches: "root",
				Stages: []StageConfig{
					{Type: "picker", Key: "user", Source: "printf 'a|b'", Delimiter: "|", Display: 1, Pass: 2},
				},
			},
		},
	}
	fn := MatchingActions(cfg, "root")
	items, err := fn(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	s := items[0].Stages[0]
	if s.Delimiter != "|" {
		t.Errorf("Delimiter = %q, want |", s.Delimiter)
	}
	if s.Display != 1 {
		t.Errorf("Display = %d, want 1", s.Display)
	}
	if s.Pass != 2 {
		t.Errorf("Pass = %d, want 2", s.Pass)
	}
}

func TestMatchingActions_NoStagesGivesActionExecute(t *testing.T) {
	cfg := &Config{
		Actions: []Action{
			{Name: "htop", Cmd: "htop", Matches: "root"},
		},
	}
	fn := MatchingActions(cfg, "root")
	items, err := fn(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if items[0].Action != item.ActionExecute {
		t.Errorf("Action = %q, want %q", items[0].Action, item.ActionExecute)
	}
	if len(items[0].Stages) != 0 {
		t.Errorf("Stages len = %d, want 0", len(items[0].Stages))
	}
}
