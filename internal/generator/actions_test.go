package generator

import (
	"testing"

	"github.com/jmcampanini/cmdk/internal/config"
	"github.com/jmcampanini/cmdk/internal/item"
)

func TestActionsGenerator_ProducesNewWindow(t *testing.T) {
	items := runActions(dirAccumulated("/home/user/projects"), Context{})
	if len(items) != 1 {
		t.Fatalf("got %d items, want 1", len(items))
	}

	it := items[0]
	if it.Display != "New window" {
		t.Errorf("Display = %q, want %q", it.Display, "New window")
	}
	if it.Type != "action" {
		t.Errorf("Type = %q, want %q", it.Type, "action")
	}
	if it.Source != "builtin" {
		t.Errorf("Source = %q, want %q", it.Source, "builtin")
	}
	if it.Action != item.ActionExecute {
		t.Errorf("Action = %q, want %q", it.Action, item.ActionExecute)
	}
	if it.Cmd != "tmux new-window -c {{sq .path}}" {
		t.Errorf("Cmd = %q, want template with path", it.Cmd)
	}
}

func TestActionsGenerator_EmptyAccumulated(t *testing.T) {
	items := runActions(nil, Context{})
	if items != nil {
		t.Errorf("expected nil for empty accumulated, got %v", items)
	}
}

func TestActionsGenerator_NoPathData(t *testing.T) {
	accumulated := []item.Item{
		{Type: "dir", Data: map[string]string{}},
	}

	items := runActions(accumulated, Context{})
	if items != nil {
		t.Errorf("expected nil when no path data, got %v", items)
	}
}

func TestActionsGenerator_EmptyPathString(t *testing.T) {
	accumulated := []item.Item{
		{Type: "dir", Data: map[string]string{"path": ""}},
	}
	items := runActions(accumulated, Context{})
	if items != nil {
		t.Errorf("expected nil when path is empty string, got %v", items)
	}
}

func TestActionsGenerator_UsesLastItem(t *testing.T) {
	accumulated := []item.Item{
		{Type: "window", Data: map[string]string{"session": "main"}},
		{Type: "dir", Data: map[string]string{"path": "/tmp"}},
	}

	items := runActions(accumulated, Context{})
	if len(items) != 1 {
		t.Fatalf("got %d items, want 1", len(items))
	}
	if items[0].Display != "New window" {
		t.Errorf("Display = %q, want %q", items[0].Display, "New window")
	}
}

func dirAccumulated(path string) []item.Item {
	return []item.Item{
		{Type: "dir", Data: map[string]string{"path": path}},
	}
}

func runActions(accumulated []item.Item, ctx Context) []item.Item {
	return NewActionsGenerator()(accumulated, ctx)
}

func TestActionsGenerator_WithConfigActions(t *testing.T) {
	cfg := config.Config{
		Actions: []config.Action{
			{Name: "Yazi", Cmd: "tmux split-window -h yazi", Matches: "dir"},
			{Name: "New pane", Cmd: "tmux split-window -v", Matches: "dir"},
		},
	}

	items := runActions(dirAccumulated("/home/user/projects"), Context{PaneID: "%5", Config: cfg})
	if len(items) != 3 {
		t.Fatalf("got %d items, want 3", len(items))
	}
	if items[0].Display != "New window" {
		t.Errorf("items[0].Display = %q, want %q", items[0].Display, "New window")
	}
	if items[1].Display != "Yazi" {
		t.Errorf("items[1].Display = %q, want %q", items[1].Display, "Yazi")
	}
	if items[2].Display != "New pane" {
		t.Errorf("items[2].Display = %q, want %q", items[2].Display, "New pane")
	}
	for i, it := range items {
		if it.Action != item.ActionExecute {
			t.Errorf("items[%d].Action = %q, want %q", i, it.Action, item.ActionExecute)
		}
	}
	if items[1].Cmd != "tmux split-window -h yazi" {
		t.Errorf("items[1].Cmd = %q, want config cmd passed through", items[1].Cmd)
	}
}

func TestActionsGenerator_NewWindowAlwaysFirst(t *testing.T) {
	cfg := config.Config{
		Actions: []config.Action{
			{Name: "Alpha", Cmd: "echo alpha", Matches: "dir"},
		},
	}

	items := runActions(dirAccumulated("/tmp"), Context{Config: cfg})
	if items[0].Display != "New window" {
		t.Errorf("first item should be 'New window', got %q", items[0].Display)
	}
	if items[0].Source != "builtin" {
		t.Errorf("first item Source = %q, want %q", items[0].Source, "builtin")
	}
}

func TestActionsGenerator_ConfigItemsHavePathAndPaneID(t *testing.T) {
	cfg := config.Config{
		Actions: []config.Action{
			{Name: "Yazi", Cmd: "yazi", Matches: "dir"},
		},
	}

	items := runActions(dirAccumulated("/home/user"), Context{PaneID: "%3", Config: cfg})
	for i, it := range items {
		if it.Data["path"] != "/home/user" {
			t.Errorf("items[%d].Data[path] = %q, want /home/user", i, it.Data["path"])
		}
		if it.Data["pane_id"] != "%3" {
			t.Errorf("items[%d].Data[pane_id] = %q, want %%3", i, it.Data["pane_id"])
		}
	}
}

func TestActionsGenerator_ConfigItemSource(t *testing.T) {
	cfg := config.Config{
		Actions: []config.Action{
			{Name: "Yazi", Cmd: "yazi", Matches: "dir"},
		},
	}

	items := runActions(dirAccumulated("/tmp"), Context{Config: cfg})
	if items[0].Source != "builtin" {
		t.Errorf("built-in Source = %q, want builtin", items[0].Source)
	}
	if items[1].Source != "config" {
		t.Errorf("config item Source = %q, want config", items[1].Source)
	}
}

func TestActionsGenerator_ZeroConfig(t *testing.T) {
	items := runActions(dirAccumulated("/tmp"), Context{})
	if len(items) != 1 {
		t.Fatalf("got %d items, want 1 (only New window)", len(items))
	}
}

func TestActionsGenerator_EmptyActions(t *testing.T) {
	cfg := config.Config{Actions: []config.Action{}}

	items := runActions(dirAccumulated("/tmp"), Context{Config: cfg})
	if len(items) != 1 {
		t.Fatalf("got %d items, want 1 (only New window)", len(items))
	}
}

func TestActionsGenerator_NoPaneID_NoKeyInData(t *testing.T) {
	items := runActions(dirAccumulated("/tmp"), Context{PaneID: ""})
	if _, ok := items[0].Data["pane_id"]; ok {
		t.Error("pane_id should not be in Data when PaneID is empty")
	}
}

func TestActionsGenerator_NewWindowHasIcon(t *testing.T) {
	items := runActions(dirAccumulated("/tmp"), Context{})
	if items[0].Icon == "" {
		t.Error("New window item should have a non-empty Icon")
	}
	if items[0].Icon != "\ueb7f" {
		t.Errorf("New window Icon = %q, want \\ueb7f", items[0].Icon)
	}
}

func TestActionsGenerator_ConfigIconPassedThrough(t *testing.T) {
	cfg := config.Config{
		Actions: []config.Action{
			{Name: "Browse", Cmd: "yazi", Matches: "dir", Icon: "\ue709"},
		},
	}
	items := runActions(dirAccumulated("/tmp"), Context{Config: cfg})
	if len(items) != 2 {
		t.Fatalf("got %d items, want 2", len(items))
	}
	if items[1].Icon != "\ue709" {
		t.Errorf("config item Icon = %q, want \\ue709", items[1].Icon)
	}
}

func TestActionsGenerator_ConfigNoIcon(t *testing.T) {
	cfg := config.Config{
		Actions: []config.Action{
			{Name: "Yazi", Cmd: "yazi", Matches: "dir"},
		},
	}
	items := runActions(dirAccumulated("/tmp"), Context{Config: cfg})
	if items[1].Icon != "" {
		t.Errorf("config item Icon = %q, want empty", items[1].Icon)
	}
}

func TestActionsGenerator_DataMapsAreIndependent(t *testing.T) {
	cfg := config.Config{
		Actions: []config.Action{
			{Name: "A", Cmd: "a", Matches: "dir"},
		},
	}

	items := runActions(dirAccumulated("/tmp"), Context{PaneID: "%1", Config: cfg})
	items[0].Data["extra"] = "mutated"
	if _, ok := items[1].Data["extra"]; ok {
		t.Error("mutating one item's Data should not affect another")
	}
}

func TestActionsGenerator_FiltersNonMatchingActions(t *testing.T) {
	cfg := config.Config{
		Actions: []config.Action{
			{Name: "Root only", Cmd: "echo root", Matches: "root"},
			{Name: "Dir action", Cmd: "echo dir", Matches: "dir"},
		},
	}

	items := runActions(dirAccumulated("/tmp"), Context{Config: cfg})
	if len(items) != 2 {
		t.Fatalf("got %d items, want 2 (New window + Dir action)", len(items))
	}
	if items[1].Display != "Dir action" {
		t.Errorf("items[1].Display = %q, want Dir action", items[1].Display)
	}
}

func TestActionsGenerator_AllItemsHaveTypeAction(t *testing.T) {
	cfg := config.Config{
		Actions: []config.Action{
			{Name: "Yazi", Cmd: "yazi", Matches: "dir"},
		},
	}

	items := runActions(dirAccumulated("/tmp"), Context{Config: cfg})
	for i, it := range items {
		if it.Type != "action" {
			t.Errorf("items[%d].Type = %q, want action", i, it.Type)
		}
	}
}

func TestActionsGenerator_StagedAction(t *testing.T) {
	cfg := config.Config{
		Actions: []config.Action{
			{
				Name: "Staged", Cmd: "echo {{.name}}", Matches: "dir",
				Stages: []config.StageConfig{
					{Type: "prompt", Key: "name", Text: "Enter name"},
				},
			},
		},
	}

	items := runActions(dirAccumulated("/tmp"), Context{Config: cfg})
	if len(items) != 2 {
		t.Fatalf("got %d items, want 2", len(items))
	}
	staged := items[1]
	if staged.Action != item.ActionStaged {
		t.Errorf("Action = %q, want %q", staged.Action, item.ActionStaged)
	}
	if len(staged.Stages) != 1 {
		t.Fatalf("Stages len = %d, want 1", len(staged.Stages))
	}
	if staged.Stages[0].Type != item.StagePrompt {
		t.Errorf("Stage.Type = %q, want prompt", staged.Stages[0].Type)
	}
	if staged.Stages[0].Key != "name" {
		t.Errorf("Stage.Key = %q, want name", staged.Stages[0].Key)
	}
}
