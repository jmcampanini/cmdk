package generator

import (
	"testing"

	"github.com/jmcampanini/cmdk/internal/item"
)

func TestDirActionsGenerator_ProducesNewWindow(t *testing.T) {
	gen := NewDirActionsGenerator()
	accumulated := []item.Item{
		{Type: "dir", Data: map[string]string{"path": "/home/user/projects"}},
	}

	items := gen(accumulated, Context{})
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
	gen := NewDirActionsGenerator()
	items := gen(nil, Context{})
	if items != nil {
		t.Errorf("expected nil for empty accumulated, got %v", items)
	}
}

func TestDirActionsGenerator_NoPathData(t *testing.T) {
	gen := NewDirActionsGenerator()
	accumulated := []item.Item{
		{Type: "dir", Data: map[string]string{}},
	}

	items := gen(accumulated, Context{})
	if items != nil {
		t.Errorf("expected nil when no path data, got %v", items)
	}
}

func TestDirActionsGenerator_UsesLastItem(t *testing.T) {
	gen := NewDirActionsGenerator()
	accumulated := []item.Item{
		{Type: "window", Data: map[string]string{"session": "main"}},
		{Type: "dir", Data: map[string]string{"path": "/tmp"}},
	}

	items := gen(accumulated, Context{})
	if len(items) != 1 {
		t.Fatalf("got %d items, want 1", len(items))
	}
	if items[0].Display != "New window" {
		t.Errorf("Display = %q, want %q", items[0].Display, "New window")
	}
}
