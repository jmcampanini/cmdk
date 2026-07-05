package generator

import (
	"testing"

	"github.com/jmcampanini/cmdk/internal/config"
	"github.com/jmcampanini/cmdk/internal/item"
)

func TestActionsGenerator_ProducesBuiltInDirActions(t *testing.T) {
	items := runActions(dirAccumulated("/home/user/projects"), Context{})
	if len(items) != 1 {
		t.Fatalf("got %d items, want 1", len(items))
	}

	newWindow := items[0]
	if newWindow.Display != "New window" {
		t.Errorf("Display = %q, want %q", newWindow.Display, "New window")
	}
	if newWindow.Type != "action" {
		t.Errorf("Type = %q, want %q", newWindow.Type, "action")
	}
	if newWindow.Source != "builtin" {
		t.Errorf("Source = %q, want %q", newWindow.Source, "builtin")
	}
	if newWindow.Action != item.ActionExecute {
		t.Errorf("Action = %q, want %q", newWindow.Action, item.ActionExecute)
	}
	if newWindow.Cmd != "" {
		t.Errorf("Cmd = %q, want empty for internal new-shell action", newWindow.Cmd)
	}
	if !newWindow.NewShell {
		t.Error("NewShell = false, want true")
	}
	if newWindow.MatchType != "dir" || newWindow.LaunchMode != config.LaunchModeSessionWindow {
		t.Errorf("new window launch metadata = match %q mode %q, want dir/session-window", newWindow.MatchType, newWindow.LaunchMode)
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
		{Type: "window", Data: map[string]string{"session_name": "main"}},
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
	wantDisplays := []string{"New window", "Yazi", "New pane"}
	for i, want := range wantDisplays {
		if items[i].Display != want {
			t.Errorf("items[%d].Display = %q, want %q", i, items[i].Display, want)
		}
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

func TestActionsGenerator_BuiltInsAlwaysFirst(t *testing.T) {
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
		t.Errorf("built-in Source = %q, want builtin", items[0].Source)
	}
	if items[1].Display != "Alpha" || items[1].Source != "config" {
		t.Errorf("second item = %q/%q, want Alpha/config", items[1].Display, items[1].Source)
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
		t.Fatalf("got %d items, want 1 built-in", len(items))
	}
}

func TestActionsGenerator_EmptyActions(t *testing.T) {
	cfg := config.Config{Actions: []config.Action{}}

	items := runActions(dirAccumulated("/tmp"), Context{Config: cfg})
	if len(items) != 1 {
		t.Fatalf("got %d items, want 1 built-in", len(items))
	}
}

func TestActionsGenerator_NoPaneID_NoKeyInData(t *testing.T) {
	items := runActions(dirAccumulated("/tmp"), Context{PaneID: ""})
	for i, it := range items {
		if _, ok := it.Data["pane_id"]; ok {
			t.Errorf("items[%d] should not include pane_id when PaneID is empty", i)
		}
	}
}

func TestActionsGenerator_BuiltInsHaveIcon(t *testing.T) {
	items := runActions(dirAccumulated("/tmp"), Context{})
	for i, it := range items {
		if it.Icon == "" {
			t.Errorf("items[%d] should have a non-empty Icon", i)
		}
		if it.Icon != "\ueb7f" {
			t.Errorf("items[%d].Icon = %q, want \\ueb7f", i, it.Icon)
		}
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
		t.Error("mutating built-in Data should not affect config item")
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
		t.Fatalf("got %d items, want 2 (built-in + Dir action)", len(items))
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
