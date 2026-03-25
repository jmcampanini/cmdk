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
		{Type: "cmd", Display: "htop"},
		{Type: "window", Display: "dev:1 node"},
		{Type: "dir", Display: "~/bar"},
	}

	got := types(GroupAndOrder(items, false))
	want := []string{"cmd", "dir", "dir", "window", "window"}

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
