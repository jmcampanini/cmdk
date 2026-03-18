package tui

import (
	"testing"

	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"

	"github.com/jmcampanini/cmdk/internal/generator"
	"github.com/jmcampanini/cmdk/internal/item"
	"github.com/jmcampanini/cmdk/internal/theme"
)

func testRegistry() *generator.Registry {
	reg := generator.NewRegistry()
	reg.Register("root", func(accumulated []item.Item, ctx generator.Context) []item.Item {
		return []item.Item{
			{Type: "window", Display: "main:1 zsh", Action: item.ActionExecute},
			{Type: "window", Display: "dev:1 node", Action: item.ActionExecute},
			{Type: "dir", Display: "~/projects/foo", Action: item.ActionNextList, Data: map[string]string{"path": "~/projects/foo"}},
		}
	})
	reg.Register("dir-actions", func(accumulated []item.Item, ctx generator.Context) []item.Item {
		return []item.Item{
			{Type: "cmd", Display: "New window", Action: item.ActionExecute, Cmd: "tmux new-window -c {{sq .path}}"},
		}
	})
	reg.MapType("", "root")
	reg.MapType("dir", "dir-actions")
	return reg
}

func testItems() []list.Item {
	return []list.Item{
		item.Item{Type: "window", Display: "main:1 zsh", Action: item.ActionExecute},
		item.Item{Type: "window", Display: "dev:1 node", Action: item.ActionExecute},
		item.Item{Type: "dir", Display: "~/projects/foo", Action: item.ActionNextList, Data: map[string]string{"path": "~/projects/foo"}},
	}
}

func TestNewModel_ItemCount(t *testing.T) {
	m := NewModel(testItems(), "%1", nil, testRegistry(), generator.Context{}, theme.Light())
	if got := len(m.list.Items()); got != 3 {
		t.Errorf("item count = %d, want 3", got)
	}
}

func TestNewModel_InitReturnsNil(t *testing.T) {
	m := NewModel(testItems(), "%1", nil, testRegistry(), generator.Context{}, theme.Light())
	if cmd := m.Init(); cmd != nil {
		t.Error("Init() should return nil")
	}
}

func TestSelectedReturnsNilByDefault(t *testing.T) {
	m := NewModel(testItems(), "%1", nil, testRegistry(), generator.Context{}, theme.Light())
	if m.Selected() != nil {
		t.Error("Selected() should be nil before any selection")
	}
}

func TestEnterOnExecuteItem_SetsSelectedAndQuits(t *testing.T) {
	m := NewModel(testItems(), "%1", nil, testRegistry(), generator.Context{}, theme.Light())
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
	m := NewModel(testItems(), "%1", nil, testRegistry(), generator.Context{}, theme.Light())
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

func drillDownToDirItem(t *testing.T, m Model) Model {
	t.Helper()
	down := tea.KeyPressMsg{Code: tea.KeyDown}
	result, _ := m.Update(down)
	m = result.(Model)
	result, _ = m.Update(down)
	m = result.(Model)

	enter := tea.KeyPressMsg{Code: tea.KeyEnter}
	result, _ = m.Update(enter)
	return result.(Model)
}

func TestEnterOnNextListItem_DrillsDown(t *testing.T) {
	m := NewModel(testItems(), "%1", nil, testRegistry(), generator.Context{}, theme.Light())
	m.list.SetSize(80, 40)

	m = drillDownToDirItem(t, m)

	if len(m.Accumulated()) != 1 {
		t.Fatalf("Accumulated() len = %d, want 1", len(m.Accumulated()))
	}
	if m.Accumulated()[0].Display != "~/projects/foo" {
		t.Errorf("Accumulated()[0].Display = %q, want %q", m.Accumulated()[0].Display, "~/projects/foo")
	}

	items := m.list.Items()
	if len(items) != 1 {
		t.Fatalf("list items = %d, want 1", len(items))
	}
	if it, ok := items[0].(item.Item); ok {
		if it.Display != "New window" {
			t.Errorf("item Display = %q, want %q", it.Display, "New window")
		}
	} else {
		t.Fatal("list item is not item.Item")
	}
}

func TestEscapeFromDrillDown_PopsBack(t *testing.T) {
	m := NewModel(testItems(), "%1", nil, testRegistry(), generator.Context{}, theme.Light())
	m.list.SetSize(80, 40)

	m = drillDownToDirItem(t, m)

	if len(m.Accumulated()) != 1 {
		t.Fatalf("after drill-down: Accumulated() len = %d, want 1", len(m.Accumulated()))
	}

	esc := tea.KeyPressMsg{Code: tea.KeyEscape}
	result, cmd := m.Update(esc)
	m = result.(Model)

	if cmd != nil {
		t.Error("Escape from drill-down should not quit")
	}
	if len(m.Accumulated()) != 0 {
		t.Errorf("after back: Accumulated() len = %d, want 0", len(m.Accumulated()))
	}
	if len(m.list.Items()) != 3 {
		t.Errorf("after back: list items = %d, want 3", len(m.list.Items()))
	}
}

func TestDrillDownThenExecute_SetsSelectedAndQuits(t *testing.T) {
	m := NewModel(testItems(), "%1", nil, testRegistry(), generator.Context{}, theme.Light())
	m.list.SetSize(80, 40)

	m = drillDownToDirItem(t, m)

	enter := tea.KeyPressMsg{Code: tea.KeyEnter}
	result, cmd := m.Update(enter)
	m = result.(Model)

	if m.Selected() == nil {
		t.Fatal("Selected() should be set after Enter on execute item in drill-down")
	}
	if m.Selected().Display != "New window" {
		t.Errorf("Selected().Display = %q, want %q", m.Selected().Display, "New window")
	}
	if cmd == nil {
		t.Error("expected Quit command")
	}
}

func TestNextListWithUnmappedType_StaysOnCurrentList(t *testing.T) {
	reg := generator.NewRegistry()
	reg.Register("root", func(accumulated []item.Item, ctx generator.Context) []item.Item {
		return nil
	})
	reg.MapType("", "root")

	items := []list.Item{
		item.Item{Type: "unknown", Display: "unmapped item", Action: item.ActionNextList, Data: map[string]string{}},
	}
	m := NewModel(items, "%1", nil, reg, generator.Context{}, theme.Light())
	m.list.SetSize(80, 40)

	enter := tea.KeyPressMsg{Code: tea.KeyEnter}
	result, cmd := m.Update(enter)
	m = result.(Model)

	if cmd != nil {
		t.Error("should not quit on resolve failure")
	}
	if m.Selected() != nil {
		t.Error("Selected() should be nil on resolve failure")
	}
	if len(m.Accumulated()) != 0 {
		t.Error("Accumulated() should be empty — resolve failed, no navigation")
	}
	if len(m.list.Items()) != 1 {
		t.Errorf("list should still have 1 item, got %d", len(m.list.Items()))
	}
}

func TestEnterOnErrorItem_NoAction(t *testing.T) {
	items := []list.Item{
		item.Item{Type: "window", Display: "main:1 zsh", Action: item.ActionExecute},
		item.Item{Type: "dir", Display: "zoxide error: command not found"},
	}
	reg := generator.NewRegistry()
	reg.Register("root", func(accumulated []item.Item, ctx generator.Context) []item.Item { return nil })
	reg.MapType("", "root")

	m := NewModel(items, "%1", nil, reg, generator.Context{}, theme.Light())
	m.list.SetSize(80, 40)

	down := tea.KeyPressMsg{Code: tea.KeyDown}
	result, _ := m.Update(down)
	m = result.(Model)

	enter := tea.KeyPressMsg{Code: tea.KeyEnter}
	result, cmd := m.Update(enter)
	m = result.(Model)

	if cmd != nil {
		t.Error("Enter on error item should not quit")
	}
	if m.Selected() != nil {
		t.Error("Selected() should be nil — error item is non-selectable")
	}
	if len(m.list.Items()) != 2 {
		t.Errorf("list should still have 2 items, got %d", len(m.list.Items()))
	}
}

func TestEscapeFromRoot_Quits(t *testing.T) {
	m := NewModel(testItems(), "%1", nil, testRegistry(), generator.Context{}, theme.Light())
	m.list.SetSize(80, 40)

	esc := tea.KeyPressMsg{Code: tea.KeyEscape}
	_, cmd := m.Update(esc)

	if cmd == nil {
		t.Error("Escape from root should quit")
	}
}
