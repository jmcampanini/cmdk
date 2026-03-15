package tui

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/bubbles/v2/list"

	"github.com/jmcampanini/cmdk/internal/item"
)

func testItems() []list.Item {
	return []list.Item{
		item.Item{Type: "window", Display: "main:1 zsh", Action: item.ActionExecute},
		item.Item{Type: "window", Display: "dev:1 node", Action: item.ActionExecute},
		item.Item{Type: "dir", Display: "~/projects/foo", Action: item.ActionNextList},
	}
}

func TestNewModel_ItemCount(t *testing.T) {
	m := NewModel(testItems(), "%1", nil)
	if got := len(m.list.Items()); got != 3 {
		t.Errorf("item count = %d, want 3", got)
	}
}

func TestNewModel_InitReturnsNil(t *testing.T) {
	m := NewModel(testItems(), "%1", nil)
	if cmd := m.Init(); cmd != nil {
		t.Error("Init() should return nil")
	}
}

func TestSelectedReturnsNilByDefault(t *testing.T) {
	m := NewModel(testItems(), "%1", nil)
	if m.Selected() != nil {
		t.Error("Selected() should be nil before any selection")
	}
}

func TestEnterOnExecuteItem_SetsSelectedAndQuits(t *testing.T) {
	m := NewModel(testItems(), "%1", nil)
	m.list.SetSize(80, 40)

	msg := tea.KeyPressMsg{Code: tea.KeyEnter}
	result, cmd := m.Update(msg)

	model := result.(Model)
	if model.Selected() == nil {
		t.Fatal("Selected() should be set after Enter on execute item")
	}
	if model.Selected().Display != "main:1 zsh" {
		t.Errorf("Selected().Display = %q, want %q", model.Selected().Display, "main:1 zsh")
	}
	if cmd == nil {
		t.Error("expected Quit command")
	}
}

func TestEnterDuringFiltering_NotIntercepted(t *testing.T) {
	m := NewModel(testItems(), "%1", nil)
	m.list.SetSize(80, 40)

	slash := tea.KeyPressMsg{Code: rune('/')}
	result, _ := m.Update(slash)
	m = result.(Model)

	if m.list.FilterState() != list.Filtering {
		t.Skip("could not enter filtering state")
	}

	enter := tea.KeyPressMsg{Code: tea.KeyEnter}
	result, _ = m.Update(enter)
	model := result.(Model)

	if model.Selected() != nil {
		t.Error("Selected() should be nil during filtering — Enter should be forwarded to list")
	}
}
