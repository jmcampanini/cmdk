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

func bellWindowItem(display string) Item {
	it := NewItem()
	it.Type = "window"
	it.Display = display
	it.Data["bell"] = "1"
	return it
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
	want := []string{"window", "window", "dir", "dir", "action"}

	if !slices.Equal(got, want) {
		t.Errorf("types = %v, want %v", got, want)
	}
}

func TestGroupAndOrder_SessionsAfterWindows(t *testing.T) {
	items := []Item{
		{Type: "session", Display: "tmux: work"},
		{Type: "window", Display: "tmux: work:1 zsh"},
		{Type: "action", Display: "htop"},
		{Type: "dir", Display: "~/foo"},
	}

	got := types(GroupAndOrder(items, false))
	want := []string{"window", "session", "dir", "action"}

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
	bellWindow := bellWindowItem("tmux: main:1 zsh")

	items := []Item{
		{Type: "action", Display: "htop"},
		{Type: "dir", Display: "~/foo"},
		bellWindow,
		{Type: "window", Display: "tmux: main:2 vim"},
	}

	got := displays(GroupAndOrder(items, true))
	want := []string{"tmux: main:1 zsh", "tmux: main:2 vim", "~/foo", "htop"}

	if !slices.Equal(got, want) {
		t.Errorf("displays = %v, want %v", got, want)
	}
}

func TestGroupAndOrder_BellToTopDisabled(t *testing.T) {
	bellWindow := bellWindowItem("tmux: main:1 zsh")

	items := []Item{
		{Type: "action", Display: "htop"},
		bellWindow,
		{Type: "window", Display: "tmux: main:2 vim"},
	}

	got := types(GroupAndOrder(items, false))
	want := []string{"window", "window", "action"}

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

func TestGroupAndOrder_ErrorsBeforeLoading(t *testing.T) {
	items := []Item{
		{Type: "loading", Display: "Loading windows\u2026"},
		{Type: "error", Display: "zoxide error: command not found"},
	}

	got := displays(GroupAndOrder(items, false))
	want := []string{"zoxide error: command not found", "Loading windows\u2026"}

	if !slices.Equal(got, want) {
		t.Errorf("displays = %v, want %v", got, want)
	}
}

func TestGroupAndOrder_LoadingBeforeBellAndNormalItems(t *testing.T) {
	bellWindow := bellWindowItem("tmux: main:1 zsh")

	items := []Item{
		{Type: "action", Display: "deploy"},
		bellWindow,
		{Type: "loading", Display: "Loading windows\u2026"},
		{Type: "dir", Display: "~/foo"},
	}

	got := displays(GroupAndOrder(items, true))
	want := []string{"Loading windows\u2026", "tmux: main:1 zsh", "~/foo", "deploy"}

	if !slices.Equal(got, want) {
		t.Errorf("displays = %v, want %v", got, want)
	}
}

func TestGroupAndOrder_ErrorsAndLoadingBeforeBellWindows(t *testing.T) {
	bellWindow := bellWindowItem("tmux: main:1 zsh")

	items := []Item{
		bellWindow,
		{Type: "loading", Display: "Loading windows\u2026"},
		{Type: "error", Display: "zoxide error: command not found"},
	}

	got := displays(GroupAndOrder(items, true))
	want := []string{"zoxide error: command not found", "Loading windows\u2026", "tmux: main:1 zsh"}

	if !slices.Equal(got, want) {
		t.Errorf("displays = %v, want %v", got, want)
	}
}

func TestGroupAndOrder_KnownTypesAfterStatusItems(t *testing.T) {
	items := []Item{
		{Type: "window", Display: "main:1 zsh"},
		{Type: "dir", Display: "~/foo"},
		{Type: "loading", Display: "Loading windows\u2026"},
		{Type: "action", Display: "deploy"},
		{Type: "error", Display: "zoxide error: command not found"},
	}

	got := displays(GroupAndOrder(items, false))
	want := []string{"zoxide error: command not found", "Loading windows\u2026", "main:1 zsh", "~/foo", "deploy"}

	if !slices.Equal(got, want) {
		t.Errorf("displays = %v, want %v", got, want)
	}
}

func TestGroupAndOrder_UnknownTypesAfterStatusAndKnownTypes(t *testing.T) {
	items := []Item{
		{Type: "custom", Display: "custom 1"},
		{Type: "loading", Display: "Loading windows\u2026"},
		{Type: "window", Display: "main:1 zsh"},
		{Type: "alien", Display: "alien 1"},
		{Type: "custom", Display: "custom 2"},
		{Type: "error", Display: "zoxide error: command not found"},
	}

	got := displays(GroupAndOrder(items, false))
	want := []string{"zoxide error: command not found", "Loading windows\u2026", "main:1 zsh", "custom 1", "custom 2", "alien 1"}

	if !slices.Equal(got, want) {
		t.Errorf("displays = %v, want %v", got, want)
	}
}

func TestGroupAndOrder_StableOrderWithinStatusBuckets(t *testing.T) {
	items := []Item{
		{Type: "loading", Display: "Loading windows\u2026"},
		{Type: "error", Display: "zoxide error: command not found"},
		{Type: "loading", Display: "Loading zoxide\u2026"},
		{Type: "error", Display: "config error: bad toml"},
	}

	got := displays(GroupAndOrder(items, false))
	want := []string{"zoxide error: command not found", "config error: bad toml", "Loading windows\u2026", "Loading zoxide\u2026"}

	if !slices.Equal(got, want) {
		t.Errorf("displays = %v, want %v", got, want)
	}
}
