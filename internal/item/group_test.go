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

	got := types(GroupAndOrder(items))
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

	got := types(GroupAndOrder(items))
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

	got := displays(GroupAndOrder(items))
	want := []string{"a", "b", "c"}

	if !slices.Equal(got, want) {
		t.Errorf("displays = %v, want %v", got, want)
	}
}

func TestGroupAndOrder_Empty(t *testing.T) {
	got := GroupAndOrder(nil)
	if len(got) != 0 {
		t.Errorf("len = %d, want 0", len(got))
	}
}

func TestGroupAndOrder_UnknownTypesAtEnd(t *testing.T) {
	items := []Item{
		{Type: "custom", Display: "x"},
		{Type: "window", Display: "w"},
		{Type: "alien", Display: "y"},
	}

	got := types(GroupAndOrder(items))
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

	got := types(GroupAndOrder(items))
	want := []string{"action", "cmd"}

	if !slices.Equal(got, want) {
		t.Errorf("types = %v, want %v", got, want)
	}
}
