package item

import (
	"testing"

	"charm.land/bubbles/v2/list"
)

var _ list.DefaultItem = Item{}

func TestNewItem_DataNotNil(t *testing.T) {
	i := NewItem()
	if i.Data == nil {
		t.Fatal("NewItem().Data should not be nil")
	}
}

func TestItem_ListMethods(t *testing.T) {
	i := Item{Display: "main:1 zsh", Type: "window"}

	if got := i.FilterValue(); got != "main:1 zsh" {
		t.Errorf("FilterValue() = %q, want %q", got, "main:1 zsh")
	}
	if got := i.Title(); got != "main:1 zsh" {
		t.Errorf("Title() = %q, want %q", got, "main:1 zsh")
	}
	if got := i.Description(); got != "window" {
		t.Errorf("Description() = %q, want %q", got, "window")
	}
}
