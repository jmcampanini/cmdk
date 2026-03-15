package tui

import (
	"testing"

	"charm.land/bubbles/v2/list"

	"github.com/jmcampanini/cmdk/internal/item"
)

func testItems() []list.Item {
	return []list.Item{
		item.Item{Type: "window", Display: "main:1 zsh"},
		item.Item{Type: "window", Display: "dev:1 node"},
		item.Item{Type: "dir", Display: "~/projects/foo"},
	}
}

func TestNewModel_ItemCount(t *testing.T) {
	m := NewModel(testItems(), "%1")
	if got := len(m.list.Items()); got != 3 {
		t.Errorf("item count = %d, want 3", got)
	}
}

func TestNewModel_InitReturnsNil(t *testing.T) {
	m := NewModel(testItems(), "%1")
	if cmd := m.Init(); cmd != nil {
		t.Error("Init() should return nil")
	}
}
