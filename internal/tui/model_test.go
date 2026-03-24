package tui

import (
	"image/color"
	"reflect"
	"strings"
	"testing"

	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
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
	upMsg    = tea.KeyPressMsg{Code: tea.KeyUp}
)

func newTestModel(items []list.Item, reg *generator.Registry) Model {
	return newTestModelWithTheme(items, reg, theme.Light())
}

func newTestModelWithTheme(items []list.Item, reg *generator.Registry, t theme.Theme) Model {
	return NewModel(items, "%1", nil, reg, generator.Context{}, t)
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

func TestNewModel_UsesTextboxThemeColorForFilterInput(t *testing.T) {
	dark := theme.Dark()
	m := newTestModelWithTheme(testItems(), testRegistry(), dark)

	checks := []struct {
		name string
		got  color.Color
	}{
		{"Focused.Text background", m.list.Styles.Filter.Focused.Text.GetBackground()},
		{"Blurred.Text background", m.list.Styles.Filter.Blurred.Text.GetBackground()},
		{"Focused.Placeholder background", m.list.Styles.Filter.Focused.Placeholder.GetBackground()},
		{"Blurred.Placeholder background", m.list.Styles.Filter.Blurred.Placeholder.GetBackground()},
		{"Focused.Suggestion background", m.list.Styles.Filter.Focused.Suggestion.GetBackground()},
		{"Blurred.Suggestion background", m.list.Styles.Filter.Blurred.Suggestion.GetBackground()},
		{"header filter background", m.filterStyle.GetBackground()},
	}
	for _, c := range checks {
		if !reflect.DeepEqual(c.got, dark.TextboxBg) {
			t.Errorf("%s = %v, want %v", c.name, c.got, dark.TextboxBg)
		}
	}

	wantSeparator := lipgloss.NewStyle().
		Inline(true).
		Background(dark.TextboxBg).
		Render(" ")
	if strings.Contains(m.list.FilterInput.Prompt, wantSeparator) {
		t.Fatalf("FilterInput.Prompt should leave the separator unstyled, got %q", m.list.FilterInput.Prompt)
	}
	if !strings.HasSuffix(m.list.FilterInput.Prompt, " ") {
		t.Fatalf("FilterInput.Prompt should still end with a plain separator space, got %q", m.list.FilterInput.Prompt)
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

func TestEnterDuringFiltering_MultipleItems_NotIntercepted(t *testing.T) {
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

func TestEnterDuringFiltering_ZeroItems_NoSelection(t *testing.T) {
	m := newTestModel(nil, testRegistry())
	m.list.SetSize(80, 40)

	if m.list.FilterState() != list.Filtering {
		t.Skip("could not enter filtering state")
	}
	if got := len(m.list.VisibleItems()); got != 0 {
		t.Fatalf("VisibleItems() = %d, want 0", got)
	}

	result, cmd := m.Update(enterMsg)
	model := result.(Model)

	if model.Selected() != nil {
		t.Error("Selected() should be nil when no items are visible")
	}
	if cmd != nil {
		t.Error("should not quit when no items are visible")
	}
}

func TestEnterDuringFiltering_SingleExecuteItem_AutoSelects(t *testing.T) {
	items := []list.Item{
		item.Item{Type: "window", Display: "only-window", Action: item.ActionExecute},
	}
	m := newTestModel(items, testRegistry())
	m.list.SetSize(80, 40)

	if m.list.FilterState() != list.Filtering {
		t.Skip("could not enter filtering state")
	}
	if got := len(m.list.VisibleItems()); got != 1 {
		t.Fatalf("VisibleItems() = %d, want 1", got)
	}

	result, cmd := m.Update(enterMsg)
	model := result.(Model)

	if model.Selected() == nil {
		t.Fatal("Selected() should be set when Enter pressed with single filtered item")
	}
	if model.Selected().Display != "only-window" {
		t.Errorf("Selected().Display = %q, want %q", model.Selected().Display, "only-window")
	}
	if cmd == nil {
		t.Error("expected Quit command")
	}
}

func TestEnterDuringFiltering_SingleNextListItem_DrillsDown(t *testing.T) {
	items := []list.Item{
		item.Item{Type: "dir", Display: "~/projects/foo", Action: item.ActionNextList, Data: map[string]string{"path": "~/projects/foo"}},
	}
	m := newTestModel(items, testRegistry())
	m.list.SetSize(80, 40)

	if m.list.FilterState() != list.Filtering {
		t.Skip("could not enter filtering state")
	}
	if got := len(m.list.VisibleItems()); got != 1 {
		t.Fatalf("VisibleItems() = %d, want 1", got)
	}

	result, _ := m.Update(enterMsg)
	model := result.(Model)

	if len(model.Accumulated()) != 1 {
		t.Fatalf("Accumulated() len = %d, want 1", len(model.Accumulated()))
	}
	if model.Accumulated()[0].Display != "~/projects/foo" {
		t.Errorf("Accumulated()[0].Display = %q, want %q", model.Accumulated()[0].Display, "~/projects/foo")
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

func TestWindowSizeMsg_RootListHeight(t *testing.T) {
	m := newTestModel(testItems(), testRegistry())
	winH := 30
	m = setWindowSize(t, m, 80, winH)

	want := winH - m.overheadHeight()
	if got := m.list.Height(); got != want {
		t.Errorf("list height at root = %d, want %d", got, want)
	}
}

func TestWindowSizeMsg_TinyTerminalClampsToOne(t *testing.T) {
	m := newTestModel(testItems(), testRegistry())
	m = setWindowSize(t, m, 80, 40)
	m = drillDownToDirItem(t, m)

	m = setWindowSize(t, m, 80, 3)
	if got := m.list.Height(); got != 1 {
		t.Errorf("list height in tiny terminal = %d, want 1", got)
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

func TestDownDuringEmptyFilter_ExitsFilterAndNavigates(t *testing.T) {
	m := newTestModel(testItems(), testRegistry())
	m.list.SetSize(80, 40)

	if m.list.FilterState() != list.Filtering {
		t.Skip("could not enter filtering state")
	}

	result, _ := m.Update(downMsg)
	m = result.(Model)

	if m.list.FilterState() != list.Unfiltered {
		t.Errorf("FilterState() = %v, want %v", m.list.FilterState(), list.Unfiltered)
	}
	if m.list.Index() != 1 {
		t.Errorf("Index() = %d, want 1", m.list.Index())
	}
}

func TestUpDuringEmptyFilter_ExitsFilter(t *testing.T) {
	m := newTestModel(testItems(), testRegistry())
	m.list.SetSize(80, 40)

	if m.list.FilterState() != list.Filtering {
		t.Skip("could not enter filtering state")
	}

	result, _ := m.Update(upMsg)
	m = result.(Model)

	if m.list.FilterState() != list.Unfiltered {
		t.Errorf("FilterState() = %v, want %v", m.list.FilterState(), list.Unfiltered)
	}
}

func TestDownDuringNonEmptyFilter_StaysInFilterMode(t *testing.T) {
	m := newTestModel(testItems(), testRegistry())
	m.list.SetSize(80, 40)

	if m.list.FilterState() != list.Filtering {
		t.Skip("could not enter filtering state")
	}

	result, _ := m.Update(tea.KeyPressMsg{Code: rune('m'), Text: "m"})
	m = result.(Model)

	if m.list.FilterInput.Value() == "" {
		t.Fatal("expected non-empty filter input after typing")
	}

	result, _ = m.Update(downMsg)
	m = result.(Model)

	if m.list.FilterState() == list.Unfiltered {
		t.Error("FilterState() should not be Unfiltered when filter has text")
	}
}

// --- Text input mode tests ---

func textInputItems() []list.Item {
	return []list.Item{
		item.Item{Type: "cmd", Display: "Claude", Action: item.ActionTextInput, Prompt: "Enter worktree name", Cmd: "claude --worktree {{sq .prompt}}", Data: map[string]string{"path": "/tmp"}},
	}
}

func textInputRegistry() *generator.Registry {
	reg := generator.NewRegistry()
	reg.Register("root", func(accumulated []item.Item, ctx generator.Context) []item.Item {
		return nil
	})
	reg.MapType("", "root")
	return reg
}

func enterTextInput(t *testing.T, m Model) Model {
	t.Helper()
	m = exitFilterMode(t, m)
	result, cmd := m.Update(enterMsg)
	m = result.(Model)
	if cmd != nil {
		t.Fatal("entering text input mode should not quit")
	}
	if m.textInputItem == nil {
		t.Fatal("textInputItem should be set")
	}
	return m
}

func TestEnterOnTextInputItem_EntersTextInputMode(t *testing.T) {
	m := newTestModel(textInputItems(), textInputRegistry())
	m.list.SetSize(80, 40)

	m = enterTextInput(t, m)

	if m.Selected() != nil {
		t.Error("Selected() should be nil in text input mode")
	}
}

func TestEnterOnTextInputItem_DuringFiltering_SingleMatch(t *testing.T) {
	m := newTestModel(textInputItems(), textInputRegistry())
	m.list.SetSize(80, 40)

	if m.list.FilterState() != list.Filtering {
		t.Skip("could not enter filtering state")
	}
	if got := len(m.list.VisibleItems()); got != 1 {
		t.Fatalf("VisibleItems() = %d, want 1", got)
	}

	result, cmd := m.Update(enterMsg)
	m = result.(Model)

	if cmd != nil {
		t.Error("should not quit when entering text input mode")
	}
	if m.textInputItem == nil {
		t.Fatal("textInputItem should be set for single filtered ActionTextInput item")
	}
	if m.Selected() != nil {
		t.Error("Selected() should be nil in text input mode")
	}
}

func TestTextInput_SubmitNonEmpty_SetsSelectedAndQuits(t *testing.T) {
	m := newTestModel(textInputItems(), textInputRegistry())
	m.list.SetSize(80, 40)
	m = enterTextInput(t, m)

	result, _ := m.Update(tea.KeyPressMsg{Code: rune('h'), Text: "h"})
	m = result.(Model)
	result, _ = m.Update(tea.KeyPressMsg{Code: rune('i'), Text: "i"})
	m = result.(Model)

	result, cmd := m.Update(enterMsg)
	m = result.(Model)

	if m.Selected() == nil {
		t.Fatal("Selected() should be set after non-empty submit")
	}
	if m.Selected().Data["prompt"] != "hi" {
		t.Errorf("Data[prompt] = %q, want %q", m.Selected().Data["prompt"], "hi")
	}
	if cmd == nil {
		t.Error("expected Quit command")
	}
}

func TestTextInput_SubmitPreservesExistingData(t *testing.T) {
	m := newTestModel(textInputItems(), textInputRegistry())
	m.list.SetSize(80, 40)
	m = enterTextInput(t, m)

	result, _ := m.Update(tea.KeyPressMsg{Code: rune('x'), Text: "x"})
	m = result.(Model)
	result, _ = m.Update(enterMsg)
	m = result.(Model)

	if m.Selected() == nil {
		t.Fatal("Selected() should be set")
	}
	if m.Selected().Data["path"] != "/tmp" {
		t.Errorf("Data[path] = %q, want /tmp", m.Selected().Data["path"])
	}
	if m.Selected().Data["prompt"] != "x" {
		t.Errorf("Data[prompt] = %q, want x", m.Selected().Data["prompt"])
	}
}

func TestTextInput_SubmitEmpty_ShowsError(t *testing.T) {
	m := newTestModel(textInputItems(), textInputRegistry())
	m.list.SetSize(80, 40)
	m = enterTextInput(t, m)

	result, cmd := m.Update(enterMsg)
	m = result.(Model)

	if !m.showInputErr {
		t.Error("showInputErr should be true after empty submit")
	}
	if m.Selected() != nil {
		t.Error("Selected() should be nil after empty submit")
	}
	if cmd != nil {
		t.Error("should not quit after empty submit")
	}
}

func TestTextInput_SubmitWhitespace_ShowsError(t *testing.T) {
	m := newTestModel(textInputItems(), textInputRegistry())
	m.list.SetSize(80, 40)
	m = enterTextInput(t, m)

	result, _ := m.Update(tea.KeyPressMsg{Code: rune(' '), Text: " "})
	m = result.(Model)
	result, _ = m.Update(tea.KeyPressMsg{Code: rune(' '), Text: " "})
	m = result.(Model)

	result, cmd := m.Update(enterMsg)
	m = result.(Model)

	if !m.showInputErr {
		t.Error("showInputErr should be true for whitespace-only submit")
	}
	if cmd != nil {
		t.Error("should not quit for whitespace-only submit")
	}
}

func TestTextInput_ErrorClearsOnKeystroke(t *testing.T) {
	m := newTestModel(textInputItems(), textInputRegistry())
	m.list.SetSize(80, 40)
	m = enterTextInput(t, m)

	result, _ := m.Update(enterMsg)
	m = result.(Model)
	if !m.showInputErr {
		t.Fatal("showInputErr should be true after empty submit")
	}

	result, _ = m.Update(tea.KeyPressMsg{Code: rune('a'), Text: "a"})
	m = result.(Model)

	if m.showInputErr {
		t.Error("showInputErr should be cleared after keystroke")
	}
}

func TestTextInput_Escape_ReturnsToList(t *testing.T) {
	m := newTestModel(textInputItems(), textInputRegistry())
	m.list.SetSize(80, 40)
	m = enterTextInput(t, m)

	result, cmd := m.Update(escMsg)
	m = result.(Model)

	if m.textInputItem != nil {
		t.Error("textInputItem should be nil after Escape")
	}
	if cmd != nil {
		t.Error("should not quit on Escape from text input")
	}
	if len(m.list.Items()) != 1 {
		t.Errorf("list should still have items, got %d", len(m.list.Items()))
	}
}

func TestTextInput_Escape_PreservesAccumulated(t *testing.T) {
	reg := generator.NewRegistry()
	reg.Register("root", func(accumulated []item.Item, ctx generator.Context) []item.Item {
		return []item.Item{
			{Type: "dir", Display: "~/projects/foo", Action: item.ActionNextList, Data: map[string]string{"path": "~/projects/foo"}},
		}
	})
	reg.Register("dir-actions", func(accumulated []item.Item, ctx generator.Context) []item.Item {
		return []item.Item{
			{Type: "cmd", Display: "Claude", Action: item.ActionTextInput, Prompt: "Enter name", Cmd: "claude", Data: map[string]string{"path": "~/projects/foo"}},
		}
	})
	reg.MapType("", "root")
	reg.MapType("dir", "dir-actions")

	items := []list.Item{
		item.Item{Type: "dir", Display: "~/projects/foo", Action: item.ActionNextList, Data: map[string]string{"path": "~/projects/foo"}},
	}
	m := newTestModel(items, reg)
	m.list.SetSize(80, 40)

	m = exitFilterMode(t, m)
	result, _ := m.Update(enterMsg)
	m = result.(Model)
	if len(m.Accumulated()) != 1 {
		t.Fatalf("should have 1 accumulated item, got %d", len(m.Accumulated()))
	}

	// After drill-down, navigateTo resets the filter to Unfiltered.
	// Press Enter directly to select the single text input item.
	result, _ = m.Update(enterMsg)
	m = result.(Model)
	if m.textInputItem == nil {
		t.Fatal("should be in text input mode")
	}

	result, _ = m.Update(escMsg)
	m = result.(Model)

	if len(m.Accumulated()) != 1 {
		t.Errorf("accumulated should still have 1 item after Escape, got %d", len(m.Accumulated()))
	}
}

func TestTextInput_SubmitTrimsInput(t *testing.T) {
	m := newTestModel(textInputItems(), textInputRegistry())
	m.list.SetSize(80, 40)
	m = enterTextInput(t, m)

	for _, ch := range " hello " {
		result, _ := m.Update(tea.KeyPressMsg{Code: rune(ch), Text: string(ch)})
		m = result.(Model)
	}

	result, _ := m.Update(enterMsg)
	m = result.(Model)

	if m.Selected() == nil {
		t.Fatal("Selected() should be set")
	}
	if m.Selected().Data["prompt"] != "hello" {
		t.Errorf("Data[prompt] = %q, want %q", m.Selected().Data["prompt"], "hello")
	}
}

func TestTextInput_WindowSizeMsg_DoesNotCrash(t *testing.T) {
	m := newTestModel(textInputItems(), textInputRegistry())
	m.list.SetSize(80, 40)
	m = enterTextInput(t, m)

	result, cmd := m.Update(tea.WindowSizeMsg{Width: 60, Height: 20})
	m = result.(Model)

	if cmd != nil {
		t.Error("WindowSizeMsg should not quit")
	}
	if m.winWidth != 60 || m.winHeight != 20 {
		t.Errorf("dimensions = %dx%d, want 60x20", m.winWidth, m.winHeight)
	}
}

func TestTextInput_CtrlC_Quits(t *testing.T) {
	m := newTestModel(textInputItems(), textInputRegistry())
	m.list.SetSize(80, 40)
	m = enterTextInput(t, m)

	_, cmd := m.Update(tea.KeyPressMsg{Code: rune('c'), Mod: tea.ModCtrl})
	if cmd == nil {
		t.Error("Ctrl+C should produce Quit command")
	}
}

func TestTextInput_NilDataMap_DoesNotPanic(t *testing.T) {
	items := []list.Item{
		item.Item{Type: "cmd", Display: "Claude", Action: item.ActionTextInput, Prompt: "Name", Cmd: "echo {{.prompt}}"},
	}
	m := newTestModel(items, textInputRegistry())
	m.list.SetSize(80, 40)
	m = enterTextInput(t, m)

	result, _ := m.Update(tea.KeyPressMsg{Code: rune('x'), Text: "x"})
	m = result.(Model)
	result, _ = m.Update(enterMsg)
	m = result.(Model)

	if m.Selected() == nil {
		t.Fatal("Selected() should be set")
	}
	if m.Selected().Data["prompt"] != "x" {
		t.Errorf("Data[prompt] = %q, want x", m.Selected().Data["prompt"])
	}
}

func TestTextInput_ViewContainsPromptAndHints(t *testing.T) {
	m := newTestModel(textInputItems(), textInputRegistry())
	m = setWindowSize(t, m, 80, 40)
	m = enterTextInput(t, m)

	content := m.View().Content
	if !strings.Contains(content, "enter submit") {
		t.Error("View should contain hint text")
	}
	if !strings.Contains(content, "esc back") {
		t.Error("View should contain esc hint")
	}
}

func TestTextInput_ViewContainsErrorWhenShown(t *testing.T) {
	m := newTestModel(textInputItems(), textInputRegistry())
	m = setWindowSize(t, m, 80, 40)
	m = enterTextInput(t, m)

	result, _ := m.Update(enterMsg)
	m = result.(Model)

	content := m.View().Content
	if !strings.Contains(content, "input required") {
		t.Error("View should contain error text when showInputErr is true")
	}
}

func TestTextInput_ViewDoesNotContainListItems(t *testing.T) {
	items := []list.Item{
		item.Item{Type: "cmd", Display: "Claude", Action: item.ActionTextInput, Prompt: "Enter name", Cmd: "claude", Data: map[string]string{}},
		item.Item{Type: "cmd", Display: "htop-visible-marker", Action: item.ActionExecute, Cmd: "htop"},
	}
	m := newTestModel(items, textInputRegistry())
	m = setWindowSize(t, m, 80, 40)
	m = enterTextInput(t, m)

	content := m.View().Content
	if strings.Contains(content, "htop-visible-marker") {
		t.Error("View should not contain list items during text input mode")
	}
}
