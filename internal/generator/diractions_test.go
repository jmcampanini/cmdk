package generator

import (
	"testing"

	"github.com/jmcampanini/cmdk/internal/config"
	"github.com/jmcampanini/cmdk/internal/item"
)

func TestDirActionsGenerator_ProducesNewWindow(t *testing.T) {
	items := runDirActions(dirAccumulated("/home/user/projects"), Context{})
	if len(items) != 1 {
		t.Fatalf("got %d items, want 1", len(items))
	}

	it := items[0]
	if it.Display != "New window" {
		t.Errorf("Display = %q, want %q", it.Display, "New window")
	}
	if it.Type != "cmd" {
		t.Errorf("Type = %q, want %q", it.Type, "cmd")
	}
	if it.Source != "generator" {
		t.Errorf("Source = %q, want %q", it.Source, "generator")
	}
	if it.Action != item.ActionExecute {
		t.Errorf("Action = %q, want %q", it.Action, item.ActionExecute)
	}
	if it.Cmd != "tmux new-window -c {{sq .path}}" {
		t.Errorf("Cmd = %q, want template with path", it.Cmd)
	}
}

func TestDirActionsGenerator_EmptyAccumulated(t *testing.T) {
	items := runDirActions(nil, Context{})
	if items != nil {
		t.Errorf("expected nil for empty accumulated, got %v", items)
	}
}

func TestDirActionsGenerator_NoPathData(t *testing.T) {
	accumulated := []item.Item{
		{Type: "dir", Data: map[string]string{}},
	}

	items := runDirActions(accumulated, Context{})
	if items != nil {
		t.Errorf("expected nil when no path data, got %v", items)
	}
}

func TestDirActionsGenerator_EmptyPathString(t *testing.T) {
	accumulated := []item.Item{
		{Type: "dir", Data: map[string]string{"path": ""}},
	}
	items := runDirActions(accumulated, Context{})
	if items != nil {
		t.Errorf("expected nil when path is empty string, got %v", items)
	}
}

func TestDirActionsGenerator_UsesLastItem(t *testing.T) {
	accumulated := []item.Item{
		{Type: "window", Data: map[string]string{"session": "main"}},
		{Type: "dir", Data: map[string]string{"path": "/tmp"}},
	}

	items := runDirActions(accumulated, Context{})
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

func runDirActions(accumulated []item.Item, ctx Context) []item.Item {
	return NewDirActionsGenerator()(accumulated, ctx)
}

func TestDirActionsGenerator_WithConfigDirActions(t *testing.T) {
	cfg := &config.Config{
		DirActions: []config.Command{
			{Name: "Yazi", Cmd: "tmux split-window -h yazi"},
			{Name: "New pane", Cmd: "tmux split-window -v"},
		},
	}

	items := runDirActions(dirAccumulated("/home/user/projects"), Context{PaneID: "%5", Config: cfg})
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

func TestDirActionsGenerator_NewWindowAlwaysFirst(t *testing.T) {
	cfg := &config.Config{
		DirActions: []config.Command{
			{Name: "Alpha", Cmd: "echo alpha"},
		},
	}

	items := runDirActions(dirAccumulated("/tmp"), Context{Config: cfg})
	if items[0].Display != "New window" {
		t.Errorf("first item should be 'New window', got %q", items[0].Display)
	}
	if items[0].Source != "generator" {
		t.Errorf("first item Source = %q, want %q", items[0].Source, "generator")
	}
}

func TestDirActionsGenerator_ConfigItemsHavePathAndPaneID(t *testing.T) {
	cfg := &config.Config{
		DirActions: []config.Command{
			{Name: "Yazi", Cmd: "yazi"},
		},
	}

	items := runDirActions(dirAccumulated("/home/user"), Context{PaneID: "%3", Config: cfg})
	for i, it := range items {
		if it.Data["path"] != "/home/user" {
			t.Errorf("items[%d].Data[path] = %q, want /home/user", i, it.Data["path"])
		}
		if it.Data["pane_id"] != "%3" {
			t.Errorf("items[%d].Data[pane_id] = %q, want %%3", i, it.Data["pane_id"])
		}
	}
}

func TestDirActionsGenerator_ConfigItemSource(t *testing.T) {
	cfg := &config.Config{
		DirActions: []config.Command{
			{Name: "Yazi", Cmd: "yazi"},
		},
	}

	items := runDirActions(dirAccumulated("/tmp"), Context{Config: cfg})
	if items[0].Source != "generator" {
		t.Errorf("built-in Source = %q, want generator", items[0].Source)
	}
	if items[1].Source != "config" {
		t.Errorf("config item Source = %q, want config", items[1].Source)
	}
}

func TestDirActionsGenerator_NilConfig(t *testing.T) {
	items := runDirActions(dirAccumulated("/tmp"), Context{Config: nil})
	if len(items) != 1 {
		t.Fatalf("got %d items, want 1 (only New window)", len(items))
	}
}

func TestDirActionsGenerator_EmptyDirActions(t *testing.T) {
	cfg := &config.Config{DirActions: []config.Command{}}

	items := runDirActions(dirAccumulated("/tmp"), Context{Config: cfg})
	if len(items) != 1 {
		t.Fatalf("got %d items, want 1 (only New window)", len(items))
	}
}

func TestDirActionsGenerator_NoPaneID_NoKeyInData(t *testing.T) {
	items := runDirActions(dirAccumulated("/tmp"), Context{PaneID: ""})
	if _, ok := items[0].Data["pane_id"]; ok {
		t.Error("pane_id should not be in Data when PaneID is empty")
	}
}

func TestDirActionsGenerator_NewWindowHasIcon(t *testing.T) {
	items := runDirActions(dirAccumulated("/tmp"), Context{})
	if items[0].Icon == "" {
		t.Error("New window item should have a non-empty Icon")
	}
	if items[0].Icon != "\uf2d0" {
		t.Errorf("New window Icon = %q, want \\uf2d0", items[0].Icon)
	}
}

func TestDirActionsGenerator_ConfigIconPassedThrough(t *testing.T) {
	cfg := &config.Config{
		DirActions: []config.Command{
			{Name: "Browse", Cmd: "yazi", Icon: "\ue709"},
		},
	}
	items := runDirActions(dirAccumulated("/tmp"), Context{Config: cfg})
	if len(items) != 2 {
		t.Fatalf("got %d items, want 2", len(items))
	}
	if items[1].Icon != "\ue709" {
		t.Errorf("config item Icon = %q, want \\ue709", items[1].Icon)
	}
}

func TestDirActionsGenerator_ConfigNoIcon(t *testing.T) {
	cfg := &config.Config{
		DirActions: []config.Command{
			{Name: "Yazi", Cmd: "yazi"},
		},
	}
	items := runDirActions(dirAccumulated("/tmp"), Context{Config: cfg})
	if items[1].Icon != "" {
		t.Errorf("config item Icon = %q, want empty", items[1].Icon)
	}
}

func TestDirActionsGenerator_DataMapsAreIndependent(t *testing.T) {
	cfg := &config.Config{
		DirActions: []config.Command{
			{Name: "A", Cmd: "a"},
		},
	}

	items := runDirActions(dirAccumulated("/tmp"), Context{PaneID: "%1", Config: cfg})
	items[0].Data["extra"] = "mutated"
	if _, ok := items[1].Data["extra"]; ok {
		t.Error("mutating one item's Data should not affect another")
	}
}

func TestDirActionsGenerator_ConfigWithPrompt_SetsActionTextInput(t *testing.T) {
	cfg := &config.Config{
		DirActions: []config.Command{
			{Name: "Claude", Cmd: "claude --worktree {{sq .prompt}}", Prompt: "Enter worktree name"},
		},
	}

	items := runDirActions(dirAccumulated("/tmp"), Context{Config: cfg})
	if len(items) != 2 {
		t.Fatalf("got %d items, want 2", len(items))
	}
	if items[1].Action != item.ActionTextInput {
		t.Errorf("Action = %q, want %q", items[1].Action, item.ActionTextInput)
	}
	if items[1].Prompt != "Enter worktree name" {
		t.Errorf("Prompt = %q, want %q", items[1].Prompt, "Enter worktree name")
	}
}

func TestDirActionsGenerator_ConfigWithoutPrompt_SetsActionExecute(t *testing.T) {
	cfg := &config.Config{
		DirActions: []config.Command{
			{Name: "Yazi", Cmd: "yazi"},
		},
	}

	items := runDirActions(dirAccumulated("/tmp"), Context{Config: cfg})
	if items[1].Action != item.ActionExecute {
		t.Errorf("Action = %q, want %q", items[1].Action, item.ActionExecute)
	}
	if items[1].Prompt != "" {
		t.Errorf("Prompt = %q, want empty", items[1].Prompt)
	}
}

func TestDirActionsGenerator_BuiltInNewWindow_AlwaysExecute(t *testing.T) {
	cfg := &config.Config{
		DirActions: []config.Command{
			{Name: "Claude", Cmd: "claude", Prompt: "Enter name"},
		},
	}

	items := runDirActions(dirAccumulated("/tmp"), Context{Config: cfg})
	if items[0].Action != item.ActionExecute {
		t.Errorf("New window Action = %q, want %q", items[0].Action, item.ActionExecute)
	}
	if items[0].Prompt != "" {
		t.Errorf("New window Prompt = %q, want empty", items[0].Prompt)
	}
}
