package cwd

import (
	"context"
	"os"
	"testing"

	"github.com/jmcampanini/cmdk/internal/item"
	"github.com/jmcampanini/cmdk/internal/pathfmt"
)

const testHome = "/home/testuser"

func TestListCWD_ReturnsOneItem(t *testing.T) {
	items, err := ListCWD(context.Background(), testHome, "~", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("got %d items, want 1", len(items))
	}
	it := items[0]
	if it.Type != "dir" {
		t.Errorf("Type = %q, want %q", it.Type, "dir")
	}
	if it.Source != "cwd" {
		t.Errorf("Source = %q, want %q", it.Source, "cwd")
	}
	if it.Action != item.ActionNextList {
		t.Errorf("Action = %q, want %q", it.Action, item.ActionNextList)
	}
}

func TestListCWD_DataMatchesGetwd(t *testing.T) {
	items, err := ListCWD(context.Background(), testHome, "~", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	wd, _ := os.Getwd()
	it := items[0]
	if it.Data["path"] != wd {
		t.Errorf("Data[path] = %q, want %q", it.Data["path"], wd)
	}
	wantDisplay := pathfmt.DisplayPath(wd, testHome, "~", nil)
	if it.Display != wantDisplay {
		t.Errorf("Display = %q, want %q", it.Display, wantDisplay)
	}
}
