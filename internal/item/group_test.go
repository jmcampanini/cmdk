package item

import (
	"slices"
	"testing"

	"charm.land/bubbles/v2/list"
)

func types(items []list.Item) []string {
	out := make([]string, len(items))
	for i, it := range items {
		out[i] = it.(Item).Type
	}
	return out
}

func displays(items []list.Item) []string {
	out := make([]string, len(items))
	for i, it := range items {
		out[i] = it.(Item).Display
	}
	return out
}

func TestGroupAndOrder_MixedTypes(t *testing.T) {
	items := []Item{
		{Type: "dir", Display: "~/foo"},
		{Type: "window", Display: "main:1 zsh"},
		{Type: "action", Display: "htop"},
		{Type: "window", Display: "dev:1 node"},
		{Type: "dir", Display: "~/bar"},
	}

	got := types(GroupAndOrder(items, false))
	want := []string{"action", "dir", "dir", "window", "window"}

	if !slices.Equal(got, want) {
		t.Errorf("types = %v, want %v", got, want)
	}
}

func TestGroupAndOrder_MixedTypesWithCmd(t *testing.T) {
	items := []Item{
		{Type: "dir", Display: "~/foo"},
		{Type: "window", Display: "main:1 zsh"},
		{Type: "cmd", Display: "htop"},
		{Type: "action", Display: "deploy"},
		{Type: "dir", Display: "~/bar"},
	}

	got := types(GroupAndOrder(items, false))
	want := []string{"action", "cmd", "dir", "dir", "window"}

	if !slices.Equal(got, want) {
		t.Errorf("types = %v, want %v", got, want)
	}
}

func TestGroupAndOrder_WithinGroupOrder(t *testing.T) {
	items := []Item{
		{Type: "window", Display: "a"},
		{Type: "window", Display: "b"},
		{Type: "window", Display: "c"},
	}

	got := displays(GroupAndOrder(items, false))
	want := []string{"a", "b", "c"}

	if !slices.Equal(got, want) {
		t.Errorf("displays = %v, want %v", got, want)
	}
}

func TestGroupAndOrder_Empty(t *testing.T) {
	got := GroupAndOrder(nil, false)
	if len(got) != 0 {
		t.Errorf("len = %d, want 0", len(got))
	}
}

func TestGroupAndOrder_BellToTop(t *testing.T) {
	bellWindow := NewItem()
	bellWindow.Type = "window"
	bellWindow.Display = "tmux: main:1 zsh"
	bellWindow.Data["bell"] = "1"

	items := []Item{
		{Type: "cmd", Display: "htop"},
		{Type: "dir", Display: "~/foo"},
		bellWindow,
		{Type: "window", Display: "tmux: main:2 vim"},
	}

	got := displays(GroupAndOrder(items, true))
	want := []string{"tmux: main:1 zsh", "htop", "~/foo", "tmux: main:2 vim"}

	if !slices.Equal(got, want) {
		t.Errorf("displays = %v, want %v", got, want)
	}
}

func TestGroupAndOrder_BellToTopDisabled(t *testing.T) {
	bellWindow := NewItem()
	bellWindow.Type = "window"
	bellWindow.Display = "tmux: main:1 zsh"
	bellWindow.Data["bell"] = "1"

	items := []Item{
		{Type: "cmd", Display: "htop"},
		bellWindow,
		{Type: "window", Display: "tmux: main:2 vim"},
	}

	got := types(GroupAndOrder(items, false))
	want := []string{"cmd", "window", "window"}

	if !slices.Equal(got, want) {
		t.Errorf("types = %v, want %v", got, want)
	}
}

func TestGroupAndOrder_UnknownTypesAtEnd(t *testing.T) {
	items := []Item{
		{Type: "custom", Display: "x"},
		{Type: "window", Display: "w"},
		{Type: "alien", Display: "y"},
	}

	got := types(GroupAndOrder(items, false))
	want := []string{"window", "custom", "alien"}

	if !slices.Equal(got, want) {
		t.Errorf("types = %v, want %v", got, want)
	}
}

func TestGroupAndOrder_ActionBeforeCmd(t *testing.T) {
	items := []Item{
		{Type: "cmd", Display: "legacy"},
		{Type: "action", Display: "new"},
	}

	got := types(GroupAndOrder(items, false))
	want := []string{"action", "cmd"}

	if !slices.Equal(got, want) {
		t.Errorf("types = %v, want %v", got, want)
	}
}

func TestGroupAndOrder_LoadingItemInSourceTypeBucket(t *testing.T) {
	loading := NewItem()
	loading.Type = "loading"
	loading.Display = "Loading windows\u2026"
	loading.Data["source_type"] = "window"

	items := []Item{
		{Type: "action", Display: "deploy"},
		{Type: "dir", Display: "~/foo"},
		loading,
	}

	got := displays(GroupAndOrder(items, false))
	want := []string{"deploy", "~/foo", "Loading windows\u2026"}

	if !slices.Equal(got, want) {
		t.Errorf("displays = %v, want %v", got, want)
	}
}

func TestGroupAndOrder_LoadingItemWithoutSourceType(t *testing.T) {
	loading := NewItem()
	loading.Type = "loading"
	loading.Display = "Loading something\u2026"

	items := []Item{
		{Type: "action", Display: "deploy"},
		loading,
	}

	got := types(GroupAndOrder(items, false))
	if len(got) != 2 {
		t.Fatalf("len = %d, want 2", len(got))
	}
	if got[0] != "action" {
		t.Errorf("got[0] = %q, want action", got[0])
	}
	if got[1] != "loading" {
		t.Errorf("got[1] = %q, want loading", got[1])
	}
}
