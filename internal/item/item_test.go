package item

import (
	"testing"

	"charm.land/bubbles/v2/list"
)

var _ list.Item = Item{}

func TestNewItem_DataNotNil(t *testing.T) {
	i := NewItem()
	if i.Data == nil {
		t.Fatal("NewItem().Data should not be nil")
	}
}

func TestItem_FilterValue(t *testing.T) {
	i := Item{Display: "main:1 zsh", Type: "window"}

	if got := i.FilterValue(); got != "main:1 zsh" {
		t.Errorf("FilterValue() = %q, want %q", got, "main:1 zsh")
	}
}
