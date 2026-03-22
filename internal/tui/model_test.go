package tui

import (
	"strings"
	"testing"

	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

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

var (
	escMsg   = tea.KeyPressMsg{Code: tea.KeyEscape}
	enterMsg = tea.KeyPressMsg{Code: tea.KeyEnter}
	downMsg  = tea.KeyPressMsg{Code: tea.KeyDown}
)

func newTestModel(items []list.Item, reg *generator.Registry) Model {
	return NewModel(items, "%1", nil, reg, generator.Context{}, theme.Light())
}

func exitFilterMode(t *testing.T, m Model) Model {
	t.Helper()
	result, _ := m.Update(escMsg)
	m = result.(Model)
	if m.list.FilterState() != list.Unfiltered {
		t.Fatal("expected Unfiltered state after Escape")
	}
	return m
}

func TestNewModel_ItemCount(t *testing.T) {
	m := newTestModel(testItems(), testRegistry())
	if got := len(m.list.Items()); got != 3 {
		t.Errorf("item count = %d, want 3", got)
	}
}

func TestNewModel_InitReturnsNil(t *testing.T) {
	m := newTestModel(testItems(), testRegistry())
	if cmd := m.Init(); cmd != nil {
		t.Error("Init() should return nil")
	}
}

func TestNewModel_StartsInFilterMode(t *testing.T) {
	m := newTestModel(testItems(), testRegistry())
	m.list.SetSize(80, 40)
	if m.list.FilterState() != list.Filtering {
		t.Errorf("FilterState() = %v, want %v", m.list.FilterState(), list.Filtering)
	}
	if got := len(m.list.VisibleItems()); got != 3 {
		t.Errorf("VisibleItems() count = %d, want 3", got)
	}
}

func TestNewModel_UsesSingleColumnFramePadding(t *testing.T) {
	m := newTestModel(testItems(), testRegistry())

	checks := []struct {
		name  string
		left  int
		right int
	}{
		{"TitleBar", m.list.Styles.TitleBar.GetPaddingLeft(), m.list.Styles.TitleBar.GetPaddingRight()},
		{"StatusBar", m.list.Styles.StatusBar.GetPaddingLeft(), m.list.Styles.StatusBar.GetPaddingRight()},
		{"NoItems", m.list.Styles.NoItems.GetPaddingLeft(), m.list.Styles.NoItems.GetPaddingRight()},
		{"PaginationStyle", m.list.Styles.PaginationStyle.GetPaddingLeft(), m.list.Styles.PaginationStyle.GetPaddingRight()},
		{"HelpStyle", m.list.Styles.HelpStyle.GetPaddingLeft(), m.list.Styles.HelpStyle.GetPaddingRight()},
	}
	for _, c := range checks {
		if c.left != horizontalPadding {
			t.Errorf("%s left padding = %d, want %d", c.name, c.left, horizontalPadding)
		}
		if c.right != horizontalPadding {
			t.Errorf("%s right padding = %d, want %d", c.name, c.right, horizontalPadding)
		}
	}
}

func TestSelectedReturnsNilByDefault(t *testing.T) {
	m := newTestModel(testItems(), testRegistry())
	if m.Selected() != nil {
		t.Error("Selected() should be nil before any selection")
	}
}

func TestEnterOnExecuteItem_SetsSelectedAndQuits(t *testing.T) {
	m := newTestModel(testItems(), testRegistry())
	m.list.SetSize(80, 40)

	m = exitFilterMode(t, m)

	result, cmd := m.Update(enterMsg)

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
	m := newTestModel(testItems(), testRegistry())
	m.list.SetSize(80, 40)

	if m.list.FilterState() != list.Filtering {
		t.Skip("could not enter filtering state")
	}

	result, _ := m.Update(enterMsg)
	model := result.(Model)

	if model.Selected() != nil {
		t.Error("Selected() should be nil during filtering — Enter should be forwarded to list")
	}
}

func drillDownToDirItem(t *testing.T, m Model) Model {
	t.Helper()
	m = exitFilterMode(t, m)

	result, _ := m.Update(downMsg)
	m = result.(Model)
	result, _ = m.Update(downMsg)
	m = result.(Model)

	result, _ = m.Update(enterMsg)
	return result.(Model)
}

func TestEnterOnNextListItem_DrillsDown(t *testing.T) {
	m := newTestModel(testItems(), testRegistry())
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
	m := newTestModel(testItems(), testRegistry())
	m.list.SetSize(80, 40)

	m = drillDownToDirItem(t, m)

	if len(m.Accumulated()) != 1 {
		t.Fatalf("after drill-down: Accumulated() len = %d, want 1", len(m.Accumulated()))
	}

	result, cmd := m.Update(escMsg)
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
	m := newTestModel(testItems(), testRegistry())
	m.list.SetSize(80, 40)

	m = drillDownToDirItem(t, m)

	result, cmd := m.Update(enterMsg)
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
	m := newTestModel(items, reg)
	m.list.SetSize(80, 40)

	m = exitFilterMode(t, m)

	result, cmd := m.Update(enterMsg)
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

	m := newTestModel(items, reg)
	m.list.SetSize(80, 40)

	m = exitFilterMode(t, m)

	result, _ := m.Update(downMsg)
	m = result.(Model)

	result, cmd := m.Update(enterMsg)
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
	m := newTestModel(testItems(), testRegistry())
	m.list.SetSize(80, 40)

	// First Escape exits filter mode
	result, cmd := m.Update(escMsg)
	m = result.(Model)
	if cmd != nil {
		t.Error("first Escape should exit filter mode, not quit")
	}

	// Second Escape from root quits
	_, cmd = m.Update(escMsg)
	if cmd == nil {
		t.Error("second Escape from root should quit")
	}
}

func setWindowSize(t *testing.T, m Model, w, h int) Model {
	t.Helper()
	result, _ := m.Update(tea.WindowSizeMsg{Width: w, Height: h})
	return result.(Model)
}

func TestStackView_EmptyAtRoot(t *testing.T) {
	m := newTestModel(testItems(), testRegistry())
	if got := m.stackView(); got != "" {
		t.Errorf("stackView() at root = %q, want empty", got)
	}
}

func TestStackView_SingleEntry(t *testing.T) {
	m := newTestModel(testItems(), testRegistry())
	m = setWindowSize(t, m, 80, 40)
	m = drillDownToDirItem(t, m)

	got := m.stackView()
	stripped := ansi.Strip(got)
	if !strings.Contains(stripped, "~/projects/foo") {
		t.Errorf("stackView() should contain dir display, got %q", stripped)
	}
	if !strings.HasSuffix(got, "\n\n") {
		t.Error("stackView() should end with trailing blank line")
	}
}

func TestStackView_MultipleEntries(t *testing.T) {
	m := newTestModel(testItems(), testRegistry())
	m.accumulated = []item.Item{
		{Display: "entry-one"},
		{Display: "entry-two"},
	}
	m = setWindowSize(t, m, 80, 40)

	got := ansi.Strip(m.stackView())
	lines := strings.Split(strings.TrimSuffix(got, "\n\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 stack lines, got %d: %q", len(lines), got)
	}
	if !strings.Contains(lines[0], "entry-one") {
		t.Errorf("first line should contain entry-one, got %q", lines[0])
	}
	if !strings.Contains(lines[1], "entry-two") {
		t.Errorf("second line should contain entry-two, got %q", lines[1])
	}
}

func TestOverheadHeight_Root(t *testing.T) {
	m := newTestModel(testItems(), testRegistry())
	if got := m.overheadHeight(); got != 2 {
		t.Errorf("overheadHeight() at root = %d, want 2", got)
	}
}

func TestOverheadHeight_WithStack(t *testing.T) {
	m := newTestModel(testItems(), testRegistry())
	m.accumulated = []item.Item{{Display: "a"}, {Display: "b"}}
	want := 2 + 2 + 1
	if got := m.overheadHeight(); got != want {
		t.Errorf("overheadHeight() with 2 entries = %d, want %d", got, want)
	}
}

func TestListHeightReducedForStack(t *testing.T) {
	m := newTestModel(testItems(), testRegistry())
	winH := 40
	m = setWindowSize(t, m, 80, winH)
	m = drillDownToDirItem(t, m)

	want := winH - m.overheadHeight()
	if got := m.list.Height(); got != want {
		t.Errorf("list height = %d, want %d (winH=%d overhead=%d)", got, want, winH, m.overheadHeight())
	}
}

func TestView_StackAppearsAfterDrillDown(t *testing.T) {
	m := newTestModel(testItems(), testRegistry())
	m = setWindowSize(t, m, 80, 40)
	m = drillDownToDirItem(t, m)

	content := ansi.Strip(m.View().Content)
	dirIdx := strings.Index(content, "~/projects/foo")
	newWinIdx := strings.Index(content, "New window")
	if dirIdx < 0 {
		t.Fatal("View should contain stack entry ~/projects/foo")
	}
	if newWinIdx < 0 {
		t.Fatal("View should contain list item New window")
	}
	if dirIdx >= newWinIdx {
		t.Error("stack entry should appear before list items")
	}
}

func TestView_StackDisappearsAfterBack(t *testing.T) {
	m := newTestModel(testItems(), testRegistry())
	m = setWindowSize(t, m, 80, 40)
	m = drillDownToDirItem(t, m)

	result, _ := m.Update(escMsg)
	m = result.(Model)

	if got := m.stackView(); got != "" {
		t.Errorf("stackView() after back should be empty, got %q", got)
	}
	if len(m.Accumulated()) != 0 {
		t.Errorf("Accumulated() should be empty after back, got %d", len(m.Accumulated()))
	}
}
