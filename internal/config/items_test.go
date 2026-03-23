package config

import (
	"context"
	"testing"

	"github.com/jmcampanini/cmdk/internal/item"
)

func TestCommandItems_EmptyConfig(t *testing.T) {
	fn := CommandItems(&Config{})
	items, err := fn(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 0 {
		t.Fatalf("expected 0 items, got %d", len(items))
	}
}

func TestCommandItems_CorrectFields(t *testing.T) {
	cfg := &Config{
		Commands: []Command{
			{Name: "htop", Cmd: "htop"},
		},
	}
	fn := CommandItems(cfg)
	items, err := fn(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("got %d items, want 1", len(items))
	}
	it := items[0]
	if it.Type != "cmd" {
		t.Errorf("Type = %q, want %q", it.Type, "cmd")
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

func TestCommandItems_PreservesOrder(t *testing.T) {
	cfg := &Config{
		Commands: []Command{
			{Name: "alpha", Cmd: "echo alpha"},
			{Name: "beta", Cmd: "echo beta"},
			{Name: "gamma", Cmd: "echo gamma"},
		},
	}
	fn := CommandItems(cfg)
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

func TestCommandItems_IconPassedThrough(t *testing.T) {
	cfg := &Config{
		Commands: []Command{
			{Name: "GitHub", Cmd: "open gh", Icon: "\ue709"},
		},
	}
	fn := CommandItems(cfg)
	items, err := fn(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if items[0].Icon != "\ue709" {
		t.Errorf("Icon = %q, want \\ue709", items[0].Icon)
	}
}

func TestCommandItems_NoIcon(t *testing.T) {
	cfg := &Config{
		Commands: []Command{
			{Name: "htop", Cmd: "htop"},
		},
	}
	fn := CommandItems(cfg)
	items, err := fn(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if items[0].Icon != "" {
		t.Errorf("Icon = %q, want empty", items[0].Icon)
	}
}
