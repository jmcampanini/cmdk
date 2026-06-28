package generator

import (
	"testing"

	"github.com/jmcampanini/cmdk/internal/config"
	"github.com/jmcampanini/cmdk/internal/item"
)

func TestActionsGenerator_ProducesBuiltInDirActions(t *testing.T) {
	items := runActions(dirAccumulated("/home/user/projects"), Context{})
	if len(items) != 2 {
		t.Fatalf("got %d items, want 2", len(items))
	}

	plain := items[0]
	if plain.Display != "New window" {
		t.Errorf("Display = %q, want %q", plain.Display, "New window")
	}
	if plain.Type != "action" {
		t.Errorf("Type = %q, want %q", plain.Type, "action")
	}
	if plain.Source != "builtin" {
		t.Errorf("Source = %q, want %q", plain.Source, "builtin")
	}
	if plain.Action != item.ActionExecute {
		t.Errorf("Action = %q, want %q", plain.Action, item.ActionExecute)
	}
	if plain.Cmd != "tmux new-window -c {{sq .path}}" {
		t.Errorf("Cmd = %q, want plain tmux new-window template", plain.Cmd)
	}

	sessioned := items[1]
	if sessioned.Display != "New session window" {
		t.Errorf("Display = %q, want %q", sessioned.Display, "New session window")
	}
	if sessioned.Type != "action" {
		t.Errorf("Type = %q, want %q", sessioned.Type, "action")
	}
	if sessioned.Source != "builtin" {
		t.Errorf("Source = %q, want %q", sessioned.Source, "builtin")
	}
	if sessioned.Action != item.ActionExecute {
		t.Errorf("Action = %q, want %q", sessioned.Action, item.ActionExecute)
	}
	if sessioned.Cmd != "cmdk session window {{sq .path}} --new" {
		t.Errorf("Cmd = %q, want session window template", sessioned.Cmd)
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
	if len(items) != 2 {
		t.Fatalf("got %d items, want 2", len(items))
	}
	if items[0].Display != "New window" {
		t.Errorf("Display = %q, want %q", items[0].Display, "New window")
	}
	if items[1].Display != "New session window" {
		t.Errorf("Display = %q, want %q", items[1].Display, "New session window")
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
	if len(items) != 4 {
		t.Fatalf("got %d items, want 4", len(items))
	}
	wantDisplays := []string{"New window", "New session window", "Yazi", "New pane"}
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
	if items[2].Cmd != "tmux split-window -h yazi" {
		t.Errorf("items[2].Cmd = %q, want config cmd passed through", items[2].Cmd)
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
	if items[1].Display != "New session window" {
		t.Errorf("second item should be 'New session window', got %q", items[1].Display)
	}
	if items[0].Source != "builtin" || items[1].Source != "builtin" {
		t.Errorf("built-ins Source = %q/%q, want builtin", items[0].Source, items[1].Source)
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
	if items[1].Source != "builtin" {
		t.Errorf("built-in Source = %q, want builtin", items[1].Source)
	}
	if items[2].Source != "config" {
		t.Errorf("config item Source = %q, want config", items[2].Source)
	}
}

func TestActionsGenerator_ZeroConfig(t *testing.T) {
	items := runActions(dirAccumulated("/tmp"), Context{})
	if len(items) != 2 {
		t.Fatalf("got %d items, want 2 built-ins", len(items))
	}
}

func TestActionsGenerator_EmptyActions(t *testing.T) {
	cfg := config.Config{Actions: []config.Action{}}

	items := runActions(dirAccumulated("/tmp"), Context{Config: cfg})
	if len(items) != 2 {
		t.Fatalf("got %d items, want 2 built-ins", len(items))
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
	for i, it := range items[:2] {
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
	if len(items) != 3 {
		t.Fatalf("got %d items, want 3", len(items))
	}
	if items[2].Icon != "\ue709" {
		t.Errorf("config item Icon = %q, want \\ue709", items[2].Icon)
	}
}

func TestActionsGenerator_ConfigNoIcon(t *testing.T) {
	cfg := config.Config{
		Actions: []config.Action{
			{Name: "Yazi", Cmd: "yazi", Matches: "dir"},
		},
	}
	items := runActions(dirAccumulated("/tmp"), Context{Config: cfg})
	if items[2].Icon != "" {
		t.Errorf("config item Icon = %q, want empty", items[2].Icon)
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
		t.Error("mutating one built-in Data should not affect another")
	}
	if _, ok := items[2].Data["extra"]; ok {
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
	if len(items) != 3 {
		t.Fatalf("got %d items, want 3 (built-ins + Dir action)", len(items))
	}
	if items[2].Display != "Dir action" {
		t.Errorf("items[2].Display = %q, want Dir action", items[2].Display)
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
	if len(items) != 3 {
		t.Fatalf("got %d items, want 3", len(items))
	}
	staged := items[2]
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
