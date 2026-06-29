package tui

import (
	"context"
	"fmt"
	"image/color"
	"reflect"
	"strings"
	"testing"

	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/jmcampanini/cmdk/internal/config"
	"github.com/jmcampanini/cmdk/internal/execute"
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
			{Type: "action", Display: "New window", Action: item.ActionExecute, Cmd: "tmux new-window -c {{sq .path}}"},
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
	return newTestModelWithTheme(items, reg, theme.Default())
}

func testConfig() config.Config {
	return config.DefaultConfig()
}

func newTestModelWithTheme(items []list.Item, reg *generator.Registry, t theme.Theme) Model {
	return NewModel(items, "%1", nil, reg, generator.Context{Config: testConfig()}, t, nil, nil)
}

func newTestModelWithConfig(items []list.Item, reg *generator.Registry, cfg config.Config) Model {
	return NewModel(items, "%1", nil, reg, generator.Context{Config: cfg}, theme.Default(), nil, nil)
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

func TestNewModel_AutoThemeDetectionRequestsBackgroundColor(t *testing.T) {
	m := newTestModel(testItems(), testRegistry()).WithAutoThemeDetection()
	cmd := m.Init()
	if cmd == nil {
		t.Fatal("Init() should request background color when auto theme detection is enabled")
	}
	if got := reflect.TypeOf(cmd()).String(); got != "tea.backgroundColorMsg" {
		t.Fatalf("Init() command returned %s, want tea.backgroundColorMsg", got)
	}
}

func TestAutoThemeDetection_SwitchesToLightBackground(t *testing.T) {
	light := theme.Light()
	m := newTestModel(testItems(), testRegistry()).WithAutoThemeDetection()

	result, cmd := m.Update(tea.BackgroundColorMsg{Color: color.White})
	if cmd != nil {
		t.Fatalf("BackgroundColorMsg update command = %T, want nil", cmd())
	}
	m = result.(Model)

	if m.theme.Name != light.Name {
		t.Fatalf("theme = %q, want %q", m.theme.Name, light.Name)
	}
	if !reflect.DeepEqual(m.filterStyle.GetBackground(), light.TextboxBg) {
		t.Errorf("filterStyle background = %v, want %v", m.filterStyle.GetBackground(), light.TextboxBg)
	}
	if !reflect.DeepEqual(m.list.Styles.Filter.Focused.Text.GetBackground(), light.TextboxBg) {
		t.Errorf("filter text background = %v, want %v", m.list.Styles.Filter.Focused.Text.GetBackground(), light.TextboxBg)
	}
	if !reflect.DeepEqual(m.list.Styles.Title.GetBackground(), light.Accent) {
		t.Errorf("title background = %v, want %v", m.list.Styles.Title.GetBackground(), light.Accent)
	}
}

func TestAutoThemeDetection_IgnoresBackgroundColorWhenDisabled(t *testing.T) {
	m := newTestModel(testItems(), testRegistry())

	result, _ := m.Update(tea.BackgroundColorMsg{Color: color.White})
	m = result.(Model)

	wantTheme := theme.Default().Name
	if m.theme.Name != wantTheme {
		t.Fatalf("theme = %q, want default %q", m.theme.Name, wantTheme)
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

func TestEnterDuringFiltering_ZeroItems_NoSelection(t *testing.T) {
	m := newTestModel(nil, testRegistry())
	m.list.SetSize(80, 40)

	if m.list.FilterState() != list.Filtering {
		t.Skip("zero-item list cannot enter filtering state")
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
		t.Fatal("could not enter filtering state")
	}

	// Type a filter term so blank-enter-exits doesn't trigger.
	result, _ := m.Update(tea.KeyPressMsg{Code: rune('o'), Text: "o"})
	m = result.(Model)

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
		t.Fatal("could not enter filtering state")
	}

	// Type a filter term so blank-enter-exits doesn't trigger.
	result, _ := m.Update(tea.KeyPressMsg{Code: rune('f'), Text: "f"})
	m = result.(Model)

	if got := len(m.list.VisibleItems()); got != 1 {
		t.Fatalf("VisibleItems() = %d, want 1", got)
	}

	result, _ = m.Update(enterMsg)
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

	// First Escape exits filter mode (drill-down re-enters filter).
	m = exitFilterMode(t, m)

	// Second Escape pops back to the root list.
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

	// Drill-down re-enters filter mode; exit before selecting.
	m = exitFilterMode(t, m)

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

func TestEnterOnErrorItem_OpensDetails(t *testing.T) {
	errorText := "zoxide error: command not found with long diagnostic details"
	items := []list.Item{
		item.Item{Type: "window", Display: "main:1 zsh", Action: item.ActionExecute},
		item.Item{Type: "error", Source: "zoxide", Display: errorText},
	}
	reg := generator.NewRegistry()
	reg.Register("root", func(accumulated []item.Item, ctx generator.Context) []item.Item { return nil })
	reg.MapType("", "root")

	m := newTestModel(items, reg)
	m = setWindowSize(t, m, 80, 40)
	m = exitFilterMode(t, m)

	result, _ := m.Update(downMsg)
	m = result.(Model)

	result, cmd := m.Update(enterMsg)
	m = result.(Model)

	if cmd != nil {
		t.Error("Enter on error item should not quit")
	}
	if m.Selected() != nil {
		t.Error("Selected() should be nil — error item is non-executable")
	}
	if m.mode != viewErrorDetails {
		t.Errorf("mode = %d, want viewErrorDetails (%d)", m.mode, viewErrorDetails)
	}
	if m.errorReturnMode != viewList {
		t.Errorf("errorReturnMode = %d, want viewList (%d)", m.errorReturnMode, viewList)
	}
	content := ansi.Strip(m.View().Content)
	if !strings.Contains(content, "Error details") || !strings.Contains(content, errorText) {
		t.Errorf("details view should contain heading and full error text, got:\n%s", content)
	}
	if !strings.Contains(content, "Source: zoxide") {
		t.Errorf("details view should contain source metadata, got:\n%s", content)
	}
	if len(m.list.Items()) != 2 {
		t.Errorf("list should still have 2 items, got %d", len(m.list.Items()))
	}
}

func TestEnterOnErrorItemDuringFiltering_OpensDetails(t *testing.T) {
	errorText := "config error: invalid table"
	items := []list.Item{
		item.Item{Type: "error", Display: errorText},
	}
	m := newTestModel(items, testRegistry())
	m = setWindowSize(t, m, 80, 40)

	if m.list.FilterState() != list.Filtering {
		t.Fatal("expected root list to start in filtering mode")
	}

	result, cmd := m.Update(enterMsg)
	m = result.(Model)

	if cmd != nil {
		t.Error("Enter on error item should not quit")
	}
	if m.mode != viewErrorDetails {
		t.Errorf("mode = %d, want viewErrorDetails (%d)", m.mode, viewErrorDetails)
	}
	content := ansi.Strip(m.errorDetailsView())
	if !strings.Contains(content, errorText) {
		t.Errorf("details view should contain %q, got %q", errorText, content)
	}
}

func TestErrorDetailsEscReturnsToRootList(t *testing.T) {
	items := []list.Item{
		item.Item{Type: "window", Display: "main:1 zsh", Action: item.ActionExecute},
		item.Item{Type: "error", Display: "zoxide error: command not found"},
	}
	m := newTestModel(items, testRegistry())
	m = setWindowSize(t, m, 80, 40)
	m = exitFilterMode(t, m)

	result, _ := m.Update(downMsg)
	m = result.(Model)
	result, _ = m.Update(enterMsg)
	m = result.(Model)
	if m.mode != viewErrorDetails {
		t.Fatal("expected error details mode")
	}

	result, cmd := m.Update(escMsg)
	m = result.(Model)

	if cmd != nil {
		t.Error("Esc from error details should not quit")
	}
	if m.mode != viewList {
		t.Errorf("mode = %d, want viewList (%d)", m.mode, viewList)
	}
	if m.list.Index() != 1 {
		t.Errorf("list index = %d, want highlighted error index 1", m.list.Index())
	}
	if got := m.list.SelectedItem().(item.Item).Type; got != "error" {
		t.Errorf("selected item type = %q, want error", got)
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

func TestErrorDetailsWrapsWithoutTruncation(t *testing.T) {
	msg := "alpha beta gamma delta epsilon zeta eta theta iota kappa lambda mu nu xi omicron tail-marker"
	m := newTestModel(nil, testRegistry())
	m = setWindowSize(t, m, 24, 10)
	m = m.openErrorDetails(item.Item{Type: "error", Display: msg})

	lines := m.errorDetailsLines()
	if len(lines) < 3 {
		t.Fatalf("wrapped line count = %d, want at least 3: %#v", len(lines), lines)
	}
	contentWidth := m.errorDetailsContentWidth()
	for _, line := range lines {
		if width := ansi.StringWidth(line); width > contentWidth {
			t.Errorf("wrapped line %q width = %d, want <= %d", line, width, contentWidth)
		}
	}

	content := ansi.Strip(m.errorDetailsView())
	if strings.Contains(content, "…") {
		t.Errorf("details view should not truncate with ellipsis, got:\n%s", content)
	}
	if !strings.Contains(content, "alpha beta") {
		t.Errorf("details view should contain the start of the full error, got:\n%s", content)
	}
}

func TestErrorDetailsScrollingClamps(t *testing.T) {
	var b strings.Builder
	for i := 1; i <= 20; i++ {
		if i > 1 {
			b.WriteByte('\n')
		}
		fmt.Fprintf(&b, "ERR %03d short line", i)
	}

	m := newTestModel(nil, testRegistry())
	m = setWindowSize(t, m, 80, 8)
	m = m.openErrorDetails(item.Item{Type: "error", Display: b.String()})

	content := ansi.Strip(m.errorDetailsView())
	if !strings.Contains(content, "ERR 001") {
		t.Fatalf("initial details view should show first line, got:\n%s", content)
	}
	if strings.Contains(content, "ERR 020") {
		t.Fatalf("initial details view should not show last line before scrolling, got:\n%s", content)
	}

	result, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyPgDown})
	m = result.(Model)
	if m.errorDetailScroll <= 0 {
		t.Fatalf("PageDown should scroll down, got scroll=%d", m.errorDetailScroll)
	}

	result, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnd})
	m = result.(Model)
	maxScroll := m.maxErrorDetailScroll()
	if m.errorDetailScroll != maxScroll {
		t.Fatalf("End scroll = %d, want max %d", m.errorDetailScroll, maxScroll)
	}
	content = ansi.Strip(m.errorDetailsView())
	if !strings.Contains(content, "ERR 020") {
		t.Fatalf("End should reveal last line, got:\n%s", content)
	}

	result, _ = m.Update(downMsg)
	m = result.(Model)
	if m.errorDetailScroll != maxScroll {
		t.Errorf("Down at bottom scroll = %d, want clamped max %d", m.errorDetailScroll, maxScroll)
	}

	result, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyHome})
	m = result.(Model)
	if m.errorDetailScroll != 0 {
		t.Fatalf("Home scroll = %d, want 0", m.errorDetailScroll)
	}
	content = ansi.Strip(m.errorDetailsView())
	if !strings.Contains(content, "ERR 001") {
		t.Fatalf("Home should reveal first line, got:\n%s", content)
	}

	result, _ = m.Update(upMsg)
	m = result.(Model)
	if m.errorDetailScroll != 0 {
		t.Errorf("Up at top scroll = %d, want clamped 0", m.errorDetailScroll)
	}
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

	// First Escape exits filter mode (drill-down re-enters filter).
	m = exitFilterMode(t, m)

	// Second Escape pops back.
	result, _ := m.Update(escMsg)
	m = result.(Model)

	if got := m.stackView(); got != "" {
		t.Errorf("stackView() after back should be empty, got %q", got)
	}
	if len(m.Accumulated()) != 0 {
		t.Errorf("Accumulated() should be empty after back, got %d", len(m.Accumulated()))
	}
}

func TestDownDuringEmptyFilter_NavigatesVisibleResults(t *testing.T) {
	m := newTestModel(testItems(), testRegistry())
	m.list.SetSize(80, 40)

	if m.list.FilterState() != list.Filtering {
		t.Fatal("could not enter filtering state")
	}

	result, _ := m.Update(downMsg)
	m = result.(Model)

	if m.list.FilterState() != list.Filtering {
		t.Errorf("FilterState() = %v, want %v", m.list.FilterState(), list.Filtering)
	}
	if m.list.Index() != 1 {
		t.Errorf("Index() = %d, want 1", m.list.Index())
	}
}

func TestEnterDuringEmptyFilter_SelectsHighlightedItem(t *testing.T) {
	m := newTestModel(testItems(), testRegistry())
	m.list.SetSize(80, 40)

	if m.list.FilterState() != list.Filtering {
		t.Fatal("could not enter filtering state")
	}

	result, cmd := m.Update(enterMsg)
	m = result.(Model)

	if m.Selected() == nil {
		t.Fatal("Selected() should be set after Enter on highlighted item")
	}
	if m.Selected().Display != "main:1 zsh" {
		t.Errorf("Selected().Display = %q, want %q", m.Selected().Display, "main:1 zsh")
	}
	if cmd == nil {
		t.Error("expected Quit command")
	}
}

func TestUpDuringEmptyFilter_NavigatesVisibleResults(t *testing.T) {
	m := newTestModel(testItems(), testRegistry())
	m.list.SetSize(80, 40)

	if m.list.FilterState() != list.Filtering {
		t.Fatal("could not enter filtering state")
	}

	result, _ := m.Update(upMsg)
	m = result.(Model)

	want := len(m.list.VisibleItems()) - 1
	if m.list.FilterState() != list.Filtering {
		t.Errorf("FilterState() = %v, want %v", m.list.FilterState(), list.Filtering)
	}
	if m.list.Index() != want {
		t.Errorf("Index() = %d, want %d", m.list.Index(), want)
	}
}

func TestDownDuringNonEmptyFilter_StaysInFilterMode(t *testing.T) {
	m := newTestModel(testItems(), testRegistry())
	m.list.SetSize(80, 40)

	if m.list.FilterState() != list.Filtering {
		t.Fatal("could not enter filtering state")
	}

	result, _ := m.Update(tea.KeyPressMsg{Code: rune('m'), Text: "m"})
	m = result.(Model)

	if m.list.FilterInput.Value() == "" {
		t.Fatal("expected non-empty filter input after typing")
	}

	result, _ = m.Update(downMsg)
	m = result.(Model)

	if m.list.FilterState() != list.Filtering {
		t.Errorf("FilterState() = %v, want %v", m.list.FilterState(), list.Filtering)
	}
	if header := ansi.Strip(m.headerView()); !strings.Contains(header, "m") {
		t.Errorf("headerView() = %q, want visible filter text %q", header, "m")
	}
}

func filterNavigationItems() []list.Item {
	return []list.Item{
		item.Item{Type: "window", Display: "alpha", Action: item.ActionExecute},
		item.Item{Type: "window", Display: "alpine", Action: item.ActionExecute},
		item.Item{Type: "window", Display: "bravo", Action: item.ActionExecute},
	}
}

func typeFilterQuery(t *testing.T, m Model, text string) Model {
	t.Helper()
	for _, ch := range text {
		result, cmd := m.Update(tea.KeyPressMsg{Code: ch, Text: string(ch)})
		m = result.(Model)
		m = runTestCmd(t, m, cmd)
	}
	return m
}

func runTestCmd(t *testing.T, m Model, cmd tea.Cmd) Model {
	t.Helper()
	if cmd == nil {
		return m
	}
	msg := cmd()
	switch msg := msg.(type) {
	case nil:
		return m
	case tea.BatchMsg:
		for _, c := range msg {
			m = runTestCmd(t, m, c)
		}
		return m
	default:
		result, _ := m.Update(msg)
		model, ok := result.(Model)
		if !ok {
			t.Fatalf("Update(%T) returned %T, want tui.Model", msg, result)
		}
		return model
	}
}

func TestDownDuringNonEmptyFilter_MovesThroughVisibleResults(t *testing.T) {
	m := newTestModel(filterNavigationItems(), testRegistry())
	m.list.SetSize(80, 40)
	m = typeFilterQuery(t, m, "al")

	visible := m.list.VisibleItems()
	if len(visible) < 2 {
		t.Fatalf("VisibleItems() = %d, want at least 2", len(visible))
	}

	want := visible[1].(item.Item).Display

	result, _ := m.Update(downMsg)
	m = result.(Model)

	if m.list.FilterState() != list.Filtering {
		t.Errorf("FilterState() = %v, want %v", m.list.FilterState(), list.Filtering)
	}
	if m.list.Index() != 1 {
		t.Fatalf("Index() = %d, want 1", m.list.Index())
	}
	got := m.list.SelectedItem().(item.Item).Display
	if got != want {
		t.Errorf("SelectedItem().Display = %q, want %q", got, want)
	}
}

func TestUpDuringNonEmptyFilter_MovesThroughVisibleResults(t *testing.T) {
	m := newTestModel(filterNavigationItems(), testRegistry())
	m.list.SetSize(80, 40)
	m = typeFilterQuery(t, m, "al")

	visible := m.list.VisibleItems()
	if len(visible) < 2 {
		t.Fatalf("VisibleItems() = %d, want at least 2", len(visible))
	}

	wantIndex := len(visible) - 1
	want := visible[wantIndex].(item.Item).Display

	result, _ := m.Update(upMsg)
	m = result.(Model)

	if m.list.FilterState() != list.Filtering {
		t.Errorf("FilterState() = %v, want %v", m.list.FilterState(), list.Filtering)
	}
	if m.list.Index() != wantIndex {
		t.Fatalf("Index() = %d, want %d", m.list.Index(), wantIndex)
	}
	got := m.list.SelectedItem().(item.Item).Display
	if got != want {
		t.Errorf("SelectedItem().Display = %q, want %q", got, want)
	}
}

func TestEnterDuringNonEmptyFilter_MultipleItemsActivatesHighlighted(t *testing.T) {
	m := newTestModel(filterNavigationItems(), testRegistry())
	m.list.SetSize(80, 40)
	m = typeFilterQuery(t, m, "al")

	result, _ := m.Update(downMsg)
	m = result.(Model)
	want := m.list.SelectedItem().(item.Item).Display

	result, cmd := m.Update(enterMsg)
	m = result.(Model)

	if m.Selected() == nil {
		t.Fatal("Selected() should be set after Enter on highlighted filtered item")
	}
	if m.Selected().Display != want {
		t.Errorf("Selected().Display = %q, want %q", m.Selected().Display, want)
	}
	if cmd == nil {
		t.Error("expected Quit command")
	}
}

func TestTypingDuringNonEmptyFilter_ResetsHighlightToFirstResult(t *testing.T) {
	m := newTestModel(filterNavigationItems(), testRegistry())
	m.list.SetSize(80, 40)
	m = typeFilterQuery(t, m, "al")

	result, _ := m.Update(downMsg)
	m = result.(Model)
	if m.list.Index() != 1 {
		t.Fatalf("after Down: Index() = %d, want 1", m.list.Index())
	}

	m = typeFilterQuery(t, m, "p")

	visible := m.list.VisibleItems()
	if len(visible) < 2 {
		t.Fatalf("VisibleItems() = %d, want at least 2", len(visible))
	}
	if m.list.Index() != 0 {
		t.Fatalf("Index() = %d, want 0 after typing refines the filter", m.list.Index())
	}
	got := m.list.SelectedItem().(item.Item).Display
	want := visible[0].(item.Item).Display
	if got != want {
		t.Errorf("SelectedItem().Display = %q, want first visible item %q", got, want)
	}
}

func TestTypingFilterRunsSynchronously(t *testing.T) {
	m := newTestModel(filterNavigationItems(), testRegistry())
	m.list.SetSize(80, 40)

	result, cmd := m.Update(tea.KeyPressMsg{Code: rune('b'), Text: "b"})
	m = result.(Model)
	if cmd != nil {
		t.Fatal("typing in the filter should not return an async filter command")
	}

	visible := m.list.VisibleItems()
	if len(visible) != 1 {
		t.Fatalf("VisibleItems() = %d, want 1", len(visible))
	}
	if got := visible[0].(item.Item).Display; got != "bravo" {
		t.Fatalf("visible item = %q, want bravo", got)
	}
}

func stagedItems() []list.Item {
	return []list.Item{
		item.Item{
			Type:    "action",
			Display: "New Branch",
			Action:  item.ActionStaged,
			Cmd:     "git checkout -b {{.branch}}",
			Stages: []item.Stage{
				{Type: item.StagePrompt, Key: "branch", Text: "Branch name:"},
			},
		},
	}
}

func stagedItemWithDefault() []list.Item {
	return []list.Item{
		item.Item{
			Type:    "action",
			Display: "New Branch",
			Action:  item.ActionStaged,
			Cmd:     "git checkout -b {{.branch}}",
			Stages: []item.Stage{
				{Type: item.StagePrompt, Key: "branch", Text: "Branch name:", Default: "feature/"},
			},
		},
	}
}

func multiStageItems() []list.Item {
	return []list.Item{
		item.Item{
			Type:    "action",
			Display: "Rename",
			Action:  item.ActionStaged,
			Cmd:     "mv {{.old}} {{.new}}",
			Stages: []item.Stage{
				{Type: item.StagePrompt, Key: "old", Text: "Old name:"},
				{Type: item.StagePrompt, Key: "new", Text: "New name:"},
			},
		},
	}
}

func selectStagedItem(t *testing.T, m Model) Model {
	t.Helper()
	m = exitFilterMode(t, m)
	result, _ := m.Update(enterMsg)
	return result.(Model)
}

func typeInPrompt(t *testing.T, m Model, text string) Model {
	t.Helper()
	for _, ch := range text {
		result, _ := m.Update(tea.KeyPressMsg{Code: ch, Text: string(ch)})
		m = result.(Model)
	}
	return m
}

func TestActionStaged_EntersPromptMode(t *testing.T) {
	m := newTestModel(stagedItems(), testRegistry())
	m.list.SetSize(80, 40)

	m = selectStagedItem(t, m)

	if m.mode != viewPrompt {
		t.Errorf("mode = %d, want viewPrompt (%d)", m.mode, viewPrompt)
	}
	if m.stageLabel != "Branch name:" {
		t.Errorf("stageLabel = %q, want %q", m.stageLabel, "Branch name:")
	}
	if len(m.Accumulated()) != 1 {
		t.Errorf("Accumulated() len = %d, want 1 (action item)", len(m.Accumulated()))
	}
}

func TestPromptEnter_PushesResultAndExecutes(t *testing.T) {
	m := newTestModel(stagedItems(), testRegistry())
	m.list.SetSize(80, 40)

	m = selectStagedItem(t, m)
	m = typeInPrompt(t, m, "feature/auth")

	result, cmd := m.Update(enterMsg)
	m = result.(Model)

	if m.Selected() == nil {
		t.Fatal("Selected() should be set after completing single stage")
	}
	if m.Selected().Cmd != "git checkout -b {{.branch}}" {
		t.Errorf("Selected().Cmd = %q", m.Selected().Cmd)
	}
	if cmd == nil {
		t.Error("expected Quit command")
	}
	found := false
	for _, it := range m.Accumulated() {
		if v, ok := it.Data["branch"]; ok && v == "feature/auth" {
			found = true
		}
	}
	if !found {
		t.Error("Accumulated() should contain stage result with branch=feature/auth")
	}
}

func TestPromptEsc_PopsBackToList(t *testing.T) {
	m := newTestModel(stagedItems(), testRegistry())
	m.list.SetSize(80, 40)

	m = selectStagedItem(t, m)
	if m.mode != viewPrompt {
		t.Fatal("expected viewPrompt mode")
	}

	result, cmd := m.Update(escMsg)
	m = result.(Model)

	if m.mode != viewList {
		t.Errorf("mode = %d, want viewList (%d)", m.mode, viewList)
	}
	if cmd != nil {
		t.Error("Esc from prompt should not quit")
	}
	if len(m.Accumulated()) != 0 {
		t.Errorf("Accumulated() len = %d, want 0 (action popped)", len(m.Accumulated()))
	}
}

func TestPromptWithDefault_PreFilled(t *testing.T) {
	m := newTestModel(stagedItemWithDefault(), testRegistry())
	m.list.SetSize(80, 40)

	m = selectStagedItem(t, m)

	if m.stageInput.Value() != "feature/" {
		t.Errorf("stageInput.Value() = %q, want %q", m.stageInput.Value(), "feature/")
	}
}

func TestMultiPromptChain_FirstStage(t *testing.T) {
	m := newTestModel(multiStageItems(), testRegistry())
	m.list.SetSize(80, 40)

	m = selectStagedItem(t, m)

	if m.mode != viewPrompt {
		t.Fatal("expected viewPrompt mode for first stage")
	}
	if m.stageLabel != "Old name:" {
		t.Errorf("stageLabel = %q, want %q", m.stageLabel, "Old name:")
	}
}

func TestMultiPromptChain_SecondStage(t *testing.T) {
	m := newTestModel(multiStageItems(), testRegistry())
	m.list.SetSize(80, 40)

	m = selectStagedItem(t, m)
	m = typeInPrompt(t, m, "main.go")

	result, cmd := m.Update(enterMsg)
	m = result.(Model)

	if cmd != nil {
		t.Error("should not quit after first stage — second stage remains")
	}
	if m.mode != viewPrompt {
		t.Fatal("expected viewPrompt mode for second stage")
	}
	if m.stageLabel != "New name:" {
		t.Errorf("stageLabel = %q, want %q", m.stageLabel, "New name:")
	}
	if len(m.Accumulated()) != 2 {
		t.Errorf("Accumulated() len = %d, want 2 (action + first result)", len(m.Accumulated()))
	}
}

func TestMultiPromptChain_CompleteBoth(t *testing.T) {
	m := newTestModel(multiStageItems(), testRegistry())
	m.list.SetSize(80, 40)

	m = selectStagedItem(t, m)
	m = typeInPrompt(t, m, "main.go")
	result, _ := m.Update(enterMsg)
	m = result.(Model)

	m = typeInPrompt(t, m, "app.go")
	result, cmd := m.Update(enterMsg)
	m = result.(Model)

	if m.Selected() == nil {
		t.Fatal("Selected() should be set after both stages complete")
	}
	if m.Selected().Cmd != "mv {{.old}} {{.new}}" {
		t.Errorf("Selected().Cmd = %q", m.Selected().Cmd)
	}
	if cmd == nil {
		t.Error("expected Quit command")
	}
	data := execute.FlattenData(m.Accumulated())
	if data["old"] != "main.go" {
		t.Errorf("data[old] = %q, want main.go", data["old"])
	}
	if data["new"] != "app.go" {
		t.Errorf("data[new] = %q, want app.go", data["new"])
	}
}

func TestMultiPromptChain_EscFromSecondStage(t *testing.T) {
	m := newTestModel(multiStageItems(), testRegistry())
	m.list.SetSize(80, 40)

	m = selectStagedItem(t, m)
	m = typeInPrompt(t, m, "main.go")
	result, _ := m.Update(enterMsg)
	m = result.(Model)

	if m.stageLabel != "New name:" {
		t.Fatalf("expected second stage, got label %q", m.stageLabel)
	}

	result, cmd := m.Update(escMsg)
	m = result.(Model)

	if cmd != nil {
		t.Error("Esc from second stage should not quit")
	}
	if m.mode != viewPrompt {
		t.Errorf("mode = %d, want viewPrompt (back to first stage)", m.mode)
	}
	if m.stageLabel != "Old name:" {
		t.Errorf("stageLabel = %q, want %q (first stage re-shown)", m.stageLabel, "Old name:")
	}
	if m.stageInput.Value() != "main.go" {
		t.Errorf("stageInput.Value() = %q, want %q (prior input restored)", m.stageInput.Value(), "main.go")
	}
}

func TestPromptView_ContainsLabelAndInput(t *testing.T) {
	m := newTestModel(stagedItems(), testRegistry())
	m = setWindowSize(t, m, 80, 40)

	m = selectStagedItem(t, m)

	view := m.View()
	content := ansi.Strip(view.Content)
	if !strings.Contains(content, "Branch name:") {
		t.Error("prompt view should contain the stage label")
	}
}

func allowEmptyPromptItems() []list.Item {
	return []list.Item{
		item.Item{
			Type:    "action",
			Display: "Commit",
			Action:  item.ActionStaged,
			Cmd:     "git commit -m {{.msg}}",
			Stages: []item.Stage{
				{Type: item.StagePrompt, Key: "msg", Text: "Message:", AllowEmpty: true},
			},
		},
	}
}

func TestPromptRequired_EnterOnEmpty_ShowsError(t *testing.T) {
	m := newTestModel(stagedItems(), testRegistry())
	m.list.SetSize(80, 40)

	m = selectStagedItem(t, m)
	result, cmd := m.Update(enterMsg)
	m = result.(Model)

	if m.mode != viewPrompt {
		t.Errorf("mode = %d, want viewPrompt", m.mode)
	}
	if m.stageError != "required" {
		t.Errorf("stageError = %q, want %q", m.stageError, "required")
	}
	if m.Selected() != nil {
		t.Error("Selected() should be nil — submission blocked")
	}
	if cmd != nil {
		t.Error("should not quit on blocked submission")
	}
}

func TestPromptRequired_EnterOnWhitespace_ShowsError(t *testing.T) {
	m := newTestModel(stagedItems(), testRegistry())
	m.list.SetSize(80, 40)

	m = selectStagedItem(t, m)
	m = typeInPrompt(t, m, "   ")
	result, _ := m.Update(enterMsg)
	m = result.(Model)

	if m.stageError != "required" {
		t.Errorf("stageError = %q, want %q", m.stageError, "required")
	}
}

func TestPromptRequired_ErrorClearsOnTyping(t *testing.T) {
	m := newTestModel(stagedItems(), testRegistry())
	m.list.SetSize(80, 40)

	m = selectStagedItem(t, m)
	result, _ := m.Update(enterMsg)
	m = result.(Model)
	if m.stageError != "required" {
		t.Fatal("expected error after empty submit")
	}

	m = typeInPrompt(t, m, "f")
	if m.stageError != "" {
		t.Errorf("stageError = %q, want empty after typing", m.stageError)
	}
}

func TestPromptRequired_SubmitsAfterTyping(t *testing.T) {
	m := newTestModel(stagedItems(), testRegistry())
	m.list.SetSize(80, 40)

	m = selectStagedItem(t, m)
	result, _ := m.Update(enterMsg)
	m = result.(Model)

	m = typeInPrompt(t, m, "feature/auth")
	result, cmd := m.Update(enterMsg)
	m = result.(Model)

	if m.Selected() == nil {
		t.Fatal("Selected() should be set after valid submission")
	}
	if cmd == nil {
		t.Error("expected Quit command")
	}
}

func TestPromptAllowEmpty_EnterOnEmpty_Submits(t *testing.T) {
	m := newTestModel(allowEmptyPromptItems(), testRegistry())
	m.list.SetSize(80, 40)

	m = selectStagedItem(t, m)
	result, cmd := m.Update(enterMsg)
	m = result.(Model)

	if m.Selected() == nil {
		t.Fatal("Selected() should be set — allow_empty permits empty")
	}
	if cmd == nil {
		t.Error("expected Quit command")
	}
	if m.stageError != "" {
		t.Errorf("stageError = %q, want empty", m.stageError)
	}
}

func TestPromptRequired_ErrorAppearsInView(t *testing.T) {
	m := newTestModel(stagedItems(), testRegistry())
	m = setWindowSize(t, m, 80, 40)

	m = selectStagedItem(t, m)
	result, _ := m.Update(enterMsg)
	m = result.(Model)

	view := m.View()
	content := ansi.Strip(view.Content)
	if !strings.Contains(content, "required") {
		t.Error("prompt view should contain error text 'required'")
	}
}

func TestPromptRequired_DefaultValueBypassesError(t *testing.T) {
	m := newTestModel(stagedItemWithDefault(), testRegistry())
	m.list.SetSize(80, 40)

	m = selectStagedItem(t, m)
	result, cmd := m.Update(enterMsg)
	m = result.(Model)

	if m.Selected() == nil {
		t.Fatal("Selected() should be set — default value is non-empty")
	}
	if cmd == nil {
		t.Error("expected Quit command")
	}
}

func TestPromptRequired_EscBackClearsError(t *testing.T) {
	m := newTestModel(multiStageItems(), testRegistry())
	m.list.SetSize(80, 40)

	m = selectStagedItem(t, m)
	m = typeInPrompt(t, m, "main.go")
	result, _ := m.Update(enterMsg)
	m = result.(Model)

	// Now on second stage — press Enter empty to trigger error
	result, _ = m.Update(enterMsg)
	m = result.(Model)
	if m.stageError != "required" {
		t.Fatal("expected error on second stage")
	}

	// Esc back to first stage — error should clear
	result, _ = m.Update(escMsg)
	m = result.(Model)
	if m.stageError != "" {
		t.Errorf("stageError = %q, want empty after Esc back", m.stageError)
	}
}

func TestPromptRequired_MixedAllowEmptyMultiStage(t *testing.T) {
	items := []list.Item{
		item.Item{
			Type:    "action",
			Display: "Mixed",
			Action:  item.ActionStaged,
			Cmd:     "echo {{.first}} {{.second}}",
			Stages: []item.Stage{
				{Type: item.StagePrompt, Key: "first", Text: "Required:"},
				{Type: item.StagePrompt, Key: "second", Text: "Optional:", AllowEmpty: true},
			},
		},
	}
	m := newTestModel(items, testRegistry())
	m.list.SetSize(80, 40)

	m = selectStagedItem(t, m)

	// First stage is required — empty blocked
	result, _ := m.Update(enterMsg)
	m = result.(Model)
	if m.stageError != "required" {
		t.Fatalf("first stage: stageError = %q, want required", m.stageError)
	}

	// Type value and advance
	m = typeInPrompt(t, m, "hello")
	result, _ = m.Update(enterMsg)
	m = result.(Model)

	// Second stage is allow_empty — empty accepted
	result, cmd := m.Update(enterMsg)
	m = result.(Model)
	if m.Selected() == nil {
		t.Fatal("Selected() should be set — second stage allows empty")
	}
	if cmd == nil {
		t.Error("expected Quit command")
	}
}

// --- Picker stage tests ---

func pickerItems() []list.Item {
	return []list.Item{
		item.Item{
			Type:    "action",
			Display: "Pick File",
			Action:  item.ActionStaged,
			Cmd:     "cat {{.file}}",
			Stages: []item.Stage{
				{Type: item.StagePicker, Key: "file", Source: "printf 'alpha\\nbeta\\ngamma'"},
			},
		},
	}
}

func pickerErrorItems() []list.Item {
	return []list.Item{
		item.Item{
			Type:    "action",
			Display: "Bad Picker",
			Action:  item.ActionStaged,
			Cmd:     "echo {{.file}}",
			Stages: []item.Stage{
				{Type: item.StagePicker, Key: "file", Source: "exit 1"},
			},
		},
	}
}

func mixedPickerPromptItems() []list.Item {
	return []list.Item{
		item.Item{
			Type:    "action",
			Display: "Pick Then Name",
			Action:  item.ActionStaged,
			Cmd:     "cp {{.file}} {{.dest}}",
			Stages: []item.Stage{
				{Type: item.StagePicker, Key: "file", Source: "printf 'a.txt\\nb.txt'"},
				{Type: item.StagePrompt, Key: "dest", Text: "Destination:"},
			},
		},
	}
}

func TestPickerStage_EntersPickerMode(t *testing.T) {
	m := newTestModel(pickerItems(), testRegistry())
	m = setWindowSize(t, m, 80, 40)

	m = selectStagedItem(t, m)

	if m.mode != viewPicker {
		t.Errorf("mode = %d, want viewPicker (%d)", m.mode, viewPicker)
	}
	if len(m.pickerList.Items()) != 3 {
		t.Errorf("picker items = %d, want 3", len(m.pickerList.Items()))
	}
}

func TestPickerStage_BlankEnterSelectsHighlightedItem(t *testing.T) {
	m := newTestModel(pickerItems(), testRegistry())
	m = setWindowSize(t, m, 80, 40)

	m = selectStagedItem(t, m)
	if m.mode != viewPicker {
		t.Fatal("expected viewPicker")
	}
	if m.pickerList.FilterState() != list.Filtering {
		t.Fatal("picker should start in filtering state")
	}

	result, cmd := m.Update(enterMsg)
	m = result.(Model)

	if m.Selected() == nil {
		t.Fatal("Selected() should be set after blank Enter on highlighted picker item")
	}
	if cmd == nil {
		t.Error("expected Quit command")
	}
	data := execute.FlattenData(m.Accumulated())
	if data["file"] != "alpha" {
		t.Errorf("data[file] = %q, want alpha", data["file"])
	}
}

func TestPickerStage_BlankDownNavigatesVisibleResults(t *testing.T) {
	m := newTestModel(pickerItems(), testRegistry())
	m = setWindowSize(t, m, 80, 40)

	m = selectStagedItem(t, m)
	if m.mode != viewPicker {
		t.Fatal("expected viewPicker")
	}

	result, _ := m.Update(downMsg)
	m = result.(Model)

	if m.pickerList.FilterState() != list.Filtering {
		t.Errorf("FilterState() = %v, want %v", m.pickerList.FilterState(), list.Filtering)
	}
	if m.pickerList.Index() != 1 {
		t.Fatalf("Index() = %d, want 1", m.pickerList.Index())
	}
	got := m.pickerList.SelectedItem().(item.Item).Display
	if got != "beta" {
		t.Errorf("SelectedItem().Display = %q, want beta", got)
	}
}

func TestPickerStage_NonEmptyFilterNavigationAndEnter(t *testing.T) {
	m := newTestModel(pickerItems(), testRegistry())
	m = setWindowSize(t, m, 80, 40)

	m = selectStagedItem(t, m)
	if m.mode != viewPicker {
		t.Fatal("expected viewPicker")
	}
	m = typeFilterQuery(t, m, "a")

	if visibleCount := len(m.pickerList.VisibleItems()); visibleCount < 2 {
		t.Fatalf("VisibleItems() = %d, want at least 2", visibleCount)
	}

	result, _ := m.Update(downMsg)
	m = result.(Model)
	if m.pickerList.FilterState() != list.Filtering {
		t.Errorf("FilterState() = %v, want %v", m.pickerList.FilterState(), list.Filtering)
	}
	if m.pickerList.Index() != 1 {
		t.Fatalf("after Down: Index() = %d, want 1", m.pickerList.Index())
	}

	result, _ = m.Update(upMsg)
	m = result.(Model)
	if m.pickerList.FilterState() != list.Filtering {
		t.Errorf("FilterState() = %v, want %v", m.pickerList.FilterState(), list.Filtering)
	}
	if m.pickerList.Index() != 0 {
		t.Fatalf("after Up: Index() = %d, want 0", m.pickerList.Index())
	}

	result, _ = m.Update(downMsg)
	m = result.(Model)
	selected := m.pickerList.SelectedItem().(item.Item)

	result, cmd := m.Update(enterMsg)
	m = result.(Model)

	if cmd == nil {
		t.Error("expected Quit command")
	}
	data := execute.FlattenData(m.Accumulated())
	if data["file"] != selected.Value {
		t.Errorf("data[file] = %q, want %q", data["file"], selected.Value)
	}
}

func TestPickerStage_TypingDuringNonEmptyFilter_ResetsHighlightToFirstResult(t *testing.T) {
	m := newTestModel(pickerItems(), testRegistry())
	m = setWindowSize(t, m, 80, 40)

	m = selectStagedItem(t, m)
	if m.mode != viewPicker {
		t.Fatal("expected viewPicker")
	}
	m = typeFilterQuery(t, m, "a")

	result, _ := m.Update(downMsg)
	m = result.(Model)
	if m.pickerList.Index() != 1 {
		t.Fatalf("after Down: Index() = %d, want 1", m.pickerList.Index())
	}

	m = typeFilterQuery(t, m, "l")

	visible := m.pickerList.VisibleItems()
	if len(visible) == 0 {
		t.Fatal("VisibleItems() = 0, want at least 1")
	}
	if m.pickerList.Index() != 0 {
		t.Fatalf("Index() = %d, want 0 after typing refines the picker filter", m.pickerList.Index())
	}
	got := m.pickerList.SelectedItem().(item.Item).Display
	want := visible[0].(item.Item).Display
	if got != want {
		t.Errorf("SelectedItem().Display = %q, want first visible item %q", got, want)
	}
}

func TestPickerStage_TypingFilterRunsSynchronously(t *testing.T) {
	m := newTestModel(pickerItems(), testRegistry())
	m = setWindowSize(t, m, 80, 40)

	m = selectStagedItem(t, m)
	if m.mode != viewPicker {
		t.Fatal("expected viewPicker")
	}

	result, cmd := m.Update(tea.KeyPressMsg{Code: rune('b'), Text: "b"})
	m = result.(Model)
	if cmd != nil {
		t.Fatal("typing in the picker filter should not return an async filter command")
	}

	visible := m.pickerList.VisibleItems()
	if len(visible) != 1 {
		t.Fatalf("VisibleItems() = %d, want 1", len(visible))
	}
	if got := visible[0].(item.Item).Display; got != "beta" {
		t.Fatalf("visible item = %q, want beta", got)
	}
}

func TestPickerStage_SelectAndExecute(t *testing.T) {
	m := newTestModel(pickerItems(), testRegistry())
	m = setWindowSize(t, m, 80, 40)

	m = selectStagedItem(t, m)

	// Exit filter and select first item.
	result, _ := m.Update(escMsg)
	m = result.(Model)
	result, cmd := m.Update(enterMsg)
	m = result.(Model)

	if m.Selected() == nil {
		t.Fatal("Selected() should be set after picker selection")
	}
	if m.Selected().Cmd != "cat {{.file}}" {
		t.Errorf("Selected().Cmd = %q", m.Selected().Cmd)
	}
	if cmd == nil {
		t.Error("expected Quit command")
	}
	data := execute.FlattenData(m.Accumulated())
	if data["file"] != "alpha" {
		t.Errorf("data[file] = %q, want alpha", data["file"])
	}
}

func TestPickerStage_ErrorShowsInList(t *testing.T) {
	m := newTestModel(pickerErrorItems(), testRegistry())
	m = setWindowSize(t, m, 80, 40)

	m = selectStagedItem(t, m)

	if m.mode != viewPicker {
		t.Errorf("mode = %d, want viewPicker", m.mode)
	}
	if len(m.pickerList.Items()) != 1 {
		t.Fatalf("picker items = %d, want 1 (error item)", len(m.pickerList.Items()))
	}
	it, ok := m.pickerList.Items()[0].(item.Item)
	if !ok {
		t.Fatal("expected item.Item")
	}
	if !strings.Contains(it.Display, "command error") {
		t.Errorf("error item Display = %q, want to contain 'command error'", it.Display)
	}
}

func TestPickerStage_EscPopsBack(t *testing.T) {
	m := newTestModel(pickerItems(), testRegistry())
	m = setWindowSize(t, m, 80, 40)

	m = selectStagedItem(t, m)
	if m.mode != viewPicker {
		t.Fatal("expected viewPicker")
	}

	// First esc exits filter mode.
	result, _ := m.Update(escMsg)
	m = result.(Model)
	// Second esc pops back.
	result, cmd := m.Update(escMsg)
	m = result.(Model)

	if cmd != nil {
		t.Error("Esc from picker should not quit")
	}
	if m.mode != viewList {
		t.Errorf("mode = %d, want viewList", m.mode)
	}
	if len(m.Accumulated()) != 0 {
		t.Errorf("Accumulated() len = %d, want 0", len(m.Accumulated()))
	}
}

func TestMixedPickerPrompt_Chain(t *testing.T) {
	m := newTestModel(mixedPickerPromptItems(), testRegistry())
	m = setWindowSize(t, m, 80, 40)

	m = selectStagedItem(t, m)
	if m.mode != viewPicker {
		t.Fatal("expected viewPicker for first stage")
	}

	// Exit filter, select first item from picker.
	result, _ := m.Update(escMsg)
	m = result.(Model)
	result, cmd := m.Update(enterMsg)
	m = result.(Model)

	if cmd != nil {
		t.Error("should not quit after first stage")
	}
	if m.mode != viewPrompt {
		t.Fatalf("mode = %d, want viewPrompt for second stage", m.mode)
	}
	if m.stageLabel != "Destination:" {
		t.Errorf("stageLabel = %q, want Destination:", m.stageLabel)
	}

	m = typeInPrompt(t, m, "/tmp/out")
	result, cmd = m.Update(enterMsg)
	m = result.(Model)

	if m.Selected() == nil {
		t.Fatal("Selected() should be set after both stages")
	}
	if cmd == nil {
		t.Error("expected Quit command")
	}
	data := execute.FlattenData(m.Accumulated())
	if data["file"] != "a.txt" {
		t.Errorf("data[file] = %q, want a.txt", data["file"])
	}
	if data["dest"] != "/tmp/out" {
		t.Errorf("data[dest] = %q, want /tmp/out", data["dest"])
	}
}

// --- Auto-select tests ---

func autoSelectRegistry() *generator.Registry {
	reg := generator.NewRegistry()
	reg.Register("root", func(_ []item.Item, _ generator.Context) []item.Item {
		return []item.Item{
			{Type: "dir", Display: "~/foo", Action: item.ActionNextList, Data: map[string]string{"path": "~/foo"}},
		}
	})
	reg.Register("dir-actions", func(_ []item.Item, _ generator.Context) []item.Item {
		return []item.Item{
			{Type: "action", Display: "Only Action", Action: item.ActionExecute, Cmd: "echo hello"},
		}
	})
	reg.MapType("", "root")
	reg.MapType("dir", "dir-actions")
	return reg
}

func autoSelectRegistryMultiple() *generator.Registry {
	reg := generator.NewRegistry()
	reg.Register("root", func(_ []item.Item, _ generator.Context) []item.Item {
		return []item.Item{
			{Type: "dir", Display: "~/foo", Action: item.ActionNextList, Data: map[string]string{"path": "~/foo"}},
		}
	})
	reg.Register("dir-actions", func(_ []item.Item, _ generator.Context) []item.Item {
		return []item.Item{
			{Type: "action", Display: "Action 1", Action: item.ActionExecute, Cmd: "echo 1"},
			{Type: "action", Display: "Action 2", Action: item.ActionExecute, Cmd: "echo 2"},
		}
	})
	reg.MapType("", "root")
	reg.MapType("dir", "dir-actions")
	return reg
}

func TestAutoSelectSingle_SkipsActionList(t *testing.T) {
	items := []list.Item{
		item.Item{Type: "dir", Display: "~/foo", Action: item.ActionNextList, Data: map[string]string{"path": "~/foo"}},
	}
	m := newTestModel(items, autoSelectRegistry())
	m.autoSelectSingle = true
	m = setWindowSize(t, m, 80, 40)

	m = exitFilterMode(t, m)
	result, cmd := m.Update(enterMsg)
	m = result.(Model)

	if m.Selected() == nil {
		t.Fatal("Selected() should be set — auto-select should have fired")
	}
	if m.Selected().Display != "Only Action" {
		t.Errorf("Selected().Display = %q, want 'Only Action'", m.Selected().Display)
	}
	if cmd == nil {
		t.Error("expected Quit command")
	}
}

func TestAutoSelectSingle_Disabled_ShowsList(t *testing.T) {
	items := []list.Item{
		item.Item{Type: "dir", Display: "~/foo", Action: item.ActionNextList, Data: map[string]string{"path": "~/foo"}},
	}
	m := newTestModel(items, autoSelectRegistry())
	m.autoSelectSingle = false
	m = setWindowSize(t, m, 80, 40)

	m = exitFilterMode(t, m)
	result, _ := m.Update(enterMsg)
	m = result.(Model)

	if m.Selected() != nil {
		t.Error("Selected() should be nil — auto-select disabled, list should be shown")
	}
	if len(m.Accumulated()) != 1 {
		t.Errorf("Accumulated() len = %d, want 1 (dir item)", len(m.Accumulated()))
	}
	if len(m.list.Items()) != 1 {
		t.Errorf("list items = %d, want 1 (action shown)", len(m.list.Items()))
	}
}

func TestAutoSelectSingle_MultipleActions_ShowsList(t *testing.T) {
	items := []list.Item{
		item.Item{Type: "dir", Display: "~/foo", Action: item.ActionNextList, Data: map[string]string{"path": "~/foo"}},
	}
	m := newTestModel(items, autoSelectRegistryMultiple())
	m.autoSelectSingle = true
	m = setWindowSize(t, m, 80, 40)

	m = exitFilterMode(t, m)
	result, _ := m.Update(enterMsg)
	m = result.(Model)

	if m.Selected() != nil {
		t.Error("Selected() should be nil — multiple actions, no auto-select")
	}
	if len(m.list.Items()) != 2 {
		t.Errorf("list items = %d, want 2", len(m.list.Items()))
	}
}

func TestPickerStage_EnterOnErrorItem_OpensDetails(t *testing.T) {
	m := newTestModel(pickerErrorItems(), testRegistry())
	m = setWindowSize(t, m, 80, 40)

	m = selectStagedItem(t, m)
	if m.mode != viewPicker {
		t.Fatal("expected viewPicker")
	}
	if m.pickerList.FilterState() != list.Filtering {
		t.Fatal("picker should start in filtering mode")
	}

	result, cmd := m.Update(enterMsg)
	m = result.(Model)

	if cmd != nil {
		t.Error("Enter on picker error item should not quit")
	}
	if m.Selected() != nil {
		t.Error("Selected() should be nil — error items are non-executable")
	}
	if m.mode != viewErrorDetails {
		t.Errorf("mode = %d, want viewErrorDetails (%d)", m.mode, viewErrorDetails)
	}
	if m.errorReturnMode != viewPicker {
		t.Errorf("errorReturnMode = %d, want viewPicker (%d)", m.errorReturnMode, viewPicker)
	}
	content := ansi.Strip(m.errorDetailsView())
	if !strings.Contains(content, "command error") {
		t.Errorf("details view should contain picker command error, got:\n%s", content)
	}

	result, cmd = m.Update(escMsg)
	m = result.(Model)
	if cmd != nil {
		t.Error("Esc from picker error details should not quit")
	}
	if m.mode != viewPicker {
		t.Errorf("mode after Esc = %d, want viewPicker (%d)", m.mode, viewPicker)
	}
	if len(m.pickerList.Items()) != 1 {
		t.Fatalf("picker list items = %d, want 1 error row", len(m.pickerList.Items()))
	}
	if got := m.pickerList.Items()[0].(item.Item).Type; got != "error" {
		t.Errorf("picker row type = %q, want error", got)
	}
}

func TestAutoSelectSingle_StagedAction_EntersPrompt(t *testing.T) {
	reg := generator.NewRegistry()
	reg.Register("root", func(_ []item.Item, _ generator.Context) []item.Item {
		return []item.Item{
			{Type: "dir", Display: "~/foo", Action: item.ActionNextList, Data: map[string]string{"path": "~/foo"}},
		}
	})
	reg.Register("dir-actions", func(_ []item.Item, _ generator.Context) []item.Item {
		return []item.Item{
			{
				Type:    "action",
				Display: "Staged Action",
				Action:  item.ActionStaged,
				Cmd:     "echo {{.val}}",
				Stages: []item.Stage{
					{Type: item.StagePrompt, Key: "val", Text: "Value:"},
				},
			},
		}
	})
	reg.MapType("", "root")
	reg.MapType("dir", "dir-actions")

	items := []list.Item{
		item.Item{Type: "dir", Display: "~/foo", Action: item.ActionNextList, Data: map[string]string{"path": "~/foo"}},
	}
	m := newTestModel(items, reg)
	m.autoSelectSingle = true
	m = setWindowSize(t, m, 80, 40)

	m = exitFilterMode(t, m)
	result, cmd := m.Update(enterMsg)
	m = result.(Model)

	if cmd != nil {
		t.Error("should not quit — staged action enters prompt")
	}
	if m.mode != viewPrompt {
		t.Errorf("mode = %d, want viewPrompt (%d)", m.mode, viewPrompt)
	}
	if m.stageLabel != "Value:" {
		t.Errorf("stageLabel = %q, want %q", m.stageLabel, "Value:")
	}
}

func TestCompleteStages_RemovesActionFromAccumulated(t *testing.T) {
	m := newTestModel(stagedItems(), testRegistry())
	m.list.SetSize(80, 40)

	m = selectStagedItem(t, m)
	m = typeInPrompt(t, m, "feature/auth")

	result, _ := m.Update(enterMsg)
	m = result.(Model)

	if m.Selected() == nil {
		t.Fatal("Selected() should be set after completing stages")
	}

	for _, it := range m.Accumulated() {
		if it.Action == item.ActionStaged {
			t.Error("Accumulated() should not contain the action item after completion")
		}
	}

	data := execute.FlattenData(m.Accumulated())
	if data["branch"] != "feature/auth" {
		t.Errorf("data[branch] = %q, want feature/auth", data["branch"])
	}
}

func TestFilterWithSpaces_MatchesAcrossSeparators(t *testing.T) {
	items := []list.Item{
		item.Item{Type: "dir", Display: "~/dotfiles/main", Action: item.ActionNextList, Data: map[string]string{"path": "~/dotfiles/main"}},
		item.Item{Type: "dir", Display: "~/projects/foo", Action: item.ActionNextList, Data: map[string]string{"path": "~/projects/foo"}},
		item.Item{Type: "window", Display: "dev:1 node", Action: item.ActionExecute},
	}
	m := newTestModel(items, testRegistry())
	m.list.SetSize(80, 40)

	if m.list.FilterState() != list.Filtering {
		t.Fatal("could not enter filtering state")
	}

	// Type the filter query, processing async filter commands after each keystroke.
	var cmd tea.Cmd
	for _, ch := range "dotfiles main" {
		var result tea.Model
		result, cmd = m.Update(tea.KeyPressMsg{Code: ch, Text: string(ch)})
		m = result.(Model)
	}
	// The last keystroke's cmd runs the filter; feed its result back.
	if cmd != nil {
		msg := cmd()
		result, _ := m.Update(msg)
		m = result.(Model)
	}

	visible := m.list.VisibleItems()
	if len(visible) != 1 {
		t.Fatalf("expected 1 visible item, got %d", len(visible))
	}
	it, ok := visible[0].(item.Item)
	if !ok {
		t.Fatal("expected item.Item")
	}
	if it.Display != "~/dotfiles/main" {
		t.Errorf("visible item = %q, want ~/dotfiles/main", it.Display)
	}
}

func typeSpaces(t *testing.T, m Model) Model {
	t.Helper()
	for _, ch := range "   " {
		result, _ := m.Update(tea.KeyPressMsg{Code: ch, Text: string(ch)})
		m = result.(Model)
	}
	if m.list.FilterInput.Value() == "" {
		t.Fatal("expected non-empty filter input after typing spaces")
	}
	return m
}

func TestDownDuringWhitespaceOnlyFilter_NavigatesVisibleResults(t *testing.T) {
	m := newTestModel(testItems(), testRegistry())
	m.list.SetSize(80, 40)

	if m.list.FilterState() != list.Filtering {
		t.Fatal("could not enter filtering state")
	}

	m = typeSpaces(t, m)

	result, _ := m.Update(downMsg)
	m = result.(Model)

	if m.list.FilterState() != list.Filtering {
		t.Errorf("FilterState() = %v, want Filtering after down on whitespace-only filter", m.list.FilterState())
	}
	if m.list.Index() != 1 {
		t.Errorf("Index() = %d, want 1", m.list.Index())
	}
}

func TestWrapList_DownPastLastWrapsToFirst(t *testing.T) {
	m := newTestModel(testItems(), testRegistry())
	m.list.SetSize(80, 40)
	m = exitFilterMode(t, m)

	if m.list.Index() != 0 {
		t.Fatalf("expected initial index 0, got %d", m.list.Index())
	}

	// Move to last item (index 2 for 3 items).
	for i := 0; i < len(m.list.Items())-1; i++ {
		result, _ := m.Update(downMsg)
		m = result.(Model)
	}
	if m.list.Index() != 2 {
		t.Fatalf("expected index 2 at bottom, got %d", m.list.Index())
	}

	// One more down should wrap to 0.
	result, _ := m.Update(downMsg)
	m = result.(Model)
	if m.list.Index() != 0 {
		t.Errorf("expected wrap to index 0, got %d", m.list.Index())
	}
}

func TestWrapList_UpPastFirstWrapsToLast(t *testing.T) {
	m := newTestModel(testItems(), testRegistry())
	m.list.SetSize(80, 40)
	m = exitFilterMode(t, m)

	if m.list.Index() != 0 {
		t.Fatalf("expected initial index 0, got %d", m.list.Index())
	}

	// Up from index 0 should wrap to last item.
	result, _ := m.Update(upMsg)
	m = result.(Model)
	lastIdx := len(m.list.Items()) - 1
	if m.list.Index() != lastIdx {
		t.Errorf("expected wrap to index %d, got %d", lastIdx, m.list.Index())
	}
}

func TestWrapListDisabled_DownAtBottomStays(t *testing.T) {
	cfg := testConfig()
	cfg.Behavior.WrapList = false
	m := newTestModelWithConfig(testItems(), testRegistry(), cfg)
	m.list.SetSize(80, 40)
	m = exitFilterMode(t, m)

	// Move to last item.
	for i := 0; i < len(m.list.Items())-1; i++ {
		result, _ := m.Update(downMsg)
		m = result.(Model)
	}
	lastIdx := len(m.list.Items()) - 1
	if m.list.Index() != lastIdx {
		t.Fatalf("expected index %d at bottom, got %d", lastIdx, m.list.Index())
	}

	// Down at bottom should stay at last item (no wrap).
	result, _ := m.Update(downMsg)
	m = result.(Model)
	if m.list.Index() != lastIdx {
		t.Errorf("expected cursor to stay at %d, got %d (wrapped when it should not)", lastIdx, m.list.Index())
	}
}

func TestWrapListDisabled_UpAtTopStays(t *testing.T) {
	cfg := testConfig()
	cfg.Behavior.WrapList = false
	m := newTestModelWithConfig(testItems(), testRegistry(), cfg)
	m.list.SetSize(80, 40)
	m = exitFilterMode(t, m)

	if m.list.Index() != 0 {
		t.Fatalf("expected initial index 0, got %d", m.list.Index())
	}

	// Up at top should stay at first item (no wrap).
	result, _ := m.Update(upMsg)
	m = result.(Model)
	if m.list.Index() != 0 {
		t.Errorf("expected cursor to stay at 0, got %d (wrapped when it should not)", m.list.Index())
	}
}

func TestEnterDuringWhitespaceOnlyFilter_SelectsHighlightedItem(t *testing.T) {
	m := newTestModel(testItems(), testRegistry())
	m.list.SetSize(80, 40)

	if m.list.FilterState() != list.Filtering {
		t.Fatal("could not enter filtering state")
	}

	m = typeSpaces(t, m)

	result, cmd := m.Update(enterMsg)
	m = result.(Model)

	if m.Selected() == nil {
		t.Fatal("Selected() should be set after Enter on whitespace-only filter")
	}
	if m.Selected().Display != "main:1 zsh" {
		t.Errorf("Selected().Display = %q, want %q", m.Selected().Display, "main:1 zsh")
	}
	if cmd == nil {
		t.Error("expected Quit command")
	}
}

func fieldSplitPickerItems() []list.Item {
	return []list.Item{
		item.Item{
			Type:    "action",
			Display: "Pick User",
			Action:  item.ActionStaged,
			Cmd:     "echo {{.user}}",
			Stages: []item.Stage{
				{Type: item.StagePicker, Key: "user", Source: "printf 'Alice|alice@co\\nBob|bob@co'", Delimiter: "|", Display: 1, Pass: 2},
			},
		},
	}
}

func TestPickerStage_FieldSplit_DisplayShowsField(t *testing.T) {
	m := newTestModel(fieldSplitPickerItems(), testRegistry())
	m = setWindowSize(t, m, 80, 40)

	m = selectStagedItem(t, m)

	if m.mode != viewPicker {
		t.Errorf("mode = %d, want viewPicker", m.mode)
	}
	items := m.pickerList.Items()
	if len(items) != 2 {
		t.Fatalf("picker items = %d, want 2", len(items))
	}
	first := items[0].(item.Item)
	if first.Display != "Alice" {
		t.Errorf("Display = %q, want Alice", first.Display)
	}
	if first.Value != "alice@co" {
		t.Errorf("Value = %q, want alice@co", first.Value)
	}
}

func TestPickerStage_FieldSplit_PassValueFlowsToData(t *testing.T) {
	m := newTestModel(fieldSplitPickerItems(), testRegistry())
	m = setWindowSize(t, m, 80, 40)

	m = selectStagedItem(t, m)

	result, _ := m.Update(escMsg)
	m = result.(Model)
	result, cmd := m.Update(enterMsg)
	m = result.(Model)

	if m.Selected() == nil {
		t.Fatal("Selected() should be set after picker selection")
	}
	if cmd == nil {
		t.Error("expected Quit command")
	}
	data := execute.FlattenData(m.Accumulated())
	if data["user"] != "alice@co" {
		t.Errorf("data[user] = %q, want alice@co (pass field, not display)", data["user"])
	}
}

func TestPickerStage_NoFieldConfig_BackwardCompat(t *testing.T) {
	m := newTestModel(pickerItems(), testRegistry())
	m = setWindowSize(t, m, 80, 40)

	m = selectStagedItem(t, m)

	result, _ := m.Update(escMsg)
	m = result.(Model)
	result, _ = m.Update(enterMsg)
	m = result.(Model)

	data := execute.FlattenData(m.Accumulated())
	if data["file"] != "alpha" {
		t.Errorf("data[file] = %q, want alpha (full line, backward compat)", data["file"])
	}
}

func TestStartInFilterFalse_StartsBrowseMode(t *testing.T) {
	cfg := testConfig()
	cfg.Behavior.StartInFilter = false
	m := newTestModelWithConfig(testItems(), testRegistry(), cfg)
	m.list.SetSize(80, 40)

	if m.list.FilterState() != list.Unfiltered {
		t.Errorf("FilterState() = %v, want Unfiltered when start_in_filter = false", m.list.FilterState())
	}
}

func TestStartInFilterFalse_SlashEntersFilterMode(t *testing.T) {
	cfg := testConfig()
	cfg.Behavior.StartInFilter = false
	m := newTestModelWithConfig(testItems(), testRegistry(), cfg)
	m.list.SetSize(80, 40)

	result, _ := m.Update(tea.KeyPressMsg{Code: rune('/')})
	m = result.(Model)

	if m.list.FilterState() != list.Filtering {
		t.Errorf("FilterState() = %v, want Filtering after pressing /", m.list.FilterState())
	}
}

func TestStartInFilterFalse_PickerStartsBrowseMode(t *testing.T) {
	cfg := testConfig()
	cfg.Behavior.StartInFilter = false
	m := newTestModelWithConfig(pickerItems(), testRegistry(), cfg)
	m.list.SetSize(80, 40)

	result, _ := m.Update(enterMsg)
	m = result.(Model)

	if m.mode != viewPicker {
		t.Fatalf("mode = %d, want viewPicker (%d)", m.mode, viewPicker)
	}
	if m.pickerList.FilterState() != list.Unfiltered {
		t.Errorf("picker FilterState() = %v, want Unfiltered when start_in_filter = false", m.pickerList.FilterState())
	}
}

func TestStartInFilterTrue_DrillDownReentersFilterMode(t *testing.T) {
	m := newTestModel(testItems(), testRegistry())
	m.list.SetSize(80, 40)

	m = drillDownToDirItem(t, m)

	if m.list.FilterState() != list.Filtering {
		t.Errorf("FilterState() = %v, want Filtering after drill-down with start_in_filter = true", m.list.FilterState())
	}
}

func TestStartInFilterFalse_DrillDownStaysBrowseMode(t *testing.T) {
	cfg := testConfig()
	cfg.Behavior.StartInFilter = false
	m := newTestModelWithConfig(testItems(), testRegistry(), cfg)
	m.list.SetSize(80, 40)

	// Navigate to the dir item (index 2) — already in browse mode, no need to exit filter.
	result, _ := m.Update(downMsg)
	m = result.(Model)
	result, _ = m.Update(downMsg)
	m = result.(Model)
	result, _ = m.Update(enterMsg)
	m = result.(Model)

	if m.list.FilterState() != list.Unfiltered {
		t.Errorf("FilterState() = %v, want Unfiltered after drill-down with start_in_filter = false", m.list.FilterState())
	}
}

func inlineTestRegistry() *generator.Registry {
	reg := generator.NewRegistry()
	reg.Register("dir-actions", func(accumulated []item.Item, ctx generator.Context) []item.Item {
		if len(accumulated) == 0 {
			return nil
		}
		last := accumulated[len(accumulated)-1]
		return []item.Item{
			{Type: "action", Display: "New window", Action: item.ActionExecute,
				Cmd: "tmux new-window -c {{sq .path}}", Data: map[string]string{"path": last.Data["path"]},
				Icon: "\ueb7f"},
			{Type: "action", Display: "Browse", Action: item.ActionExecute,
				Cmd: "yazi {{sq .path}}", Data: map[string]string{"path": last.Data["path"]},
				Icon: "\ueaf7"},
		}
	})
	reg.MapType("dir", "dir-actions")
	return reg
}

func newInlineTestModel(t *testing.T) Model {
	t.Helper()
	cfg := testConfig()
	cfg.Behavior.InlineActions = true
	cfg.Behavior.StartInFilter = false

	reg := inlineTestRegistry()

	baseItems := []item.Item{
		{Type: "action", Display: "htop", Action: item.ActionExecute, Cmd: "htop"},
		{Type: "dir", Display: "~/projects/foo", Action: item.ActionNextList, Data: map[string]string{"path": "/home/user/projects/foo"}},
		{Type: "dir", Display: "~/code/bar", Action: item.ActionNextList, Data: map[string]string{"path": "/home/user/code/bar"}},
		{Type: "window", Display: "main:1 zsh", Action: item.ActionExecute},
	}
	listItems := item.GroupAndOrder(baseItems, false)

	m := NewModel(listItems, "%1", nil, reg, generator.Context{Config: cfg}, theme.Default(), nil, baseItems)
	m.list.SetSize(80, 40)
	return m
}

func TestInline_StartupExpandsItems(t *testing.T) {
	m := newInlineTestModel(t)

	// 1 root action (htop) + 2 dirs × 2 actions each + 1 window = 6
	items := m.list.Items()
	if len(items) != 6 {
		var displays []string
		for _, li := range items {
			if it, ok := li.(item.Item); ok {
				displays = append(displays, it.Display)
			}
		}
		t.Fatalf("got %d items %v, want 6", len(items), displays)
	}

	// Check that dir items were expanded, not kept as-is
	for _, li := range items {
		it := li.(item.Item)
		if it.Type == "dir" && it.Action == item.ActionNextList {
			t.Errorf("found unexpanded dir item: %q", it.Display)
		}
	}
}

func TestInline_DisplayFormat(t *testing.T) {
	m := newInlineTestModel(t)

	var found bool
	for _, li := range m.list.Items() {
		it := li.(item.Item)
		if it.Display == "~/projects/foo » New window" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected to find item with display '~/projects/foo » New window'")
	}
}

func TestInline_SelectExecutePushesParent(t *testing.T) {
	m := newInlineTestModel(t)

	// Find the inline item for "~/projects/foo » New window"
	for i, li := range m.list.Items() {
		it := li.(item.Item)
		if it.Display == "~/projects/foo » New window" {
			m.list.Select(i)
			break
		}
	}

	result, cmd := m.Update(enterMsg)
	m = result.(Model)

	if cmd == nil {
		t.Fatal("expected tea.Quit command")
	}
	if m.selected == nil {
		t.Fatal("selected is nil")
	}
	if m.selected.Display != "New window" {
		t.Errorf("selected.Display = %q, want New window", m.selected.Display)
	}
	if len(m.accumulated) != 1 {
		t.Fatalf("accumulated len = %d, want 1", len(m.accumulated))
	}
	if m.accumulated[0].Display != "~/projects/foo" {
		t.Errorf("accumulated[0].Display = %q, want ~/projects/foo", m.accumulated[0].Display)
	}
	if m.accumulated[0].Data["path"] != "/home/user/projects/foo" {
		t.Errorf("accumulated[0].Data[path] = %q", m.accumulated[0].Data["path"])
	}
}

func TestInline_SelectExecuteRendersCmd(t *testing.T) {
	m := newInlineTestModel(t)

	for i, li := range m.list.Items() {
		it := li.(item.Item)
		if it.Display == "~/projects/foo » New window" {
			m.list.Select(i)
			break
		}
	}

	result, _ := m.Update(enterMsg)
	m = result.(Model)

	all := append(m.accumulated, *m.selected)
	data := execute.FlattenData(all)
	rendered, err := execute.RenderCmd(m.selected.Cmd, data)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if rendered != "tmux new-window -c '/home/user/projects/foo'" {
		t.Errorf("rendered = %q", rendered)
	}
}

func TestInline_SelectStagedPushesParent(t *testing.T) {
	reg := generator.NewRegistry()
	reg.Register("dir-actions", func(accumulated []item.Item, ctx generator.Context) []item.Item {
		if len(accumulated) == 0 {
			return nil
		}
		last := accumulated[len(accumulated)-1]
		return []item.Item{
			{Type: "action", Display: "New branch", Action: item.ActionStaged,
				Cmd: "git checkout -b {{.branch}}", Data: map[string]string{"path": last.Data["path"]},
				Stages: []item.Stage{{Type: item.StagePrompt, Key: "branch", Text: "Branch name:"}}},
		}
	})
	reg.MapType("dir", "dir-actions")

	cfg := testConfig()
	cfg.Behavior.InlineActions = true
	cfg.Behavior.StartInFilter = false

	baseItems := []item.Item{
		{Type: "dir", Display: "~/proj", Action: item.ActionNextList, Data: map[string]string{"path": "/proj"}},
	}
	listItems := item.GroupAndOrder(baseItems, false)
	m := NewModel(listItems, "%1", nil, reg, generator.Context{Config: cfg}, theme.Default(), nil, baseItems)
	m.list.SetSize(80, 40)

	result, _ := m.Update(enterMsg)
	m = result.(Model)

	if m.mode != viewPrompt {
		t.Fatalf("mode = %d, want viewPrompt", m.mode)
	}
	// accumulated should have [parent_dir, staged_action]
	if len(m.accumulated) != 2 {
		t.Fatalf("accumulated len = %d, want 2", len(m.accumulated))
	}
	if m.accumulated[0].Display != "~/proj" {
		t.Errorf("accumulated[0].Display = %q, want ~/proj", m.accumulated[0].Display)
	}
	if m.accumulated[1].Display != "New branch" {
		t.Errorf("accumulated[1].Display = %q, want New branch", m.accumulated[1].Display)
	}
}

func TestInline_AsyncRebuildExpands(t *testing.T) {
	cfg := testConfig()
	cfg.Behavior.InlineActions = true
	cfg.Behavior.StartInFilter = false

	reg := inlineTestRegistry()

	syncItems := []item.Item{
		{Type: "action", Display: "htop", Action: item.ActionExecute, Cmd: "htop"},
	}
	asyncSources := []AsyncSource{
		{Name: "zoxide", Fetch: func(_ context.Context) ([]item.Item, error) {
			return []item.Item{
				{Type: "dir", Display: "~/async-dir", Action: item.ActionNextList, Data: map[string]string{"path": "/async"}},
			}, nil
		}},
	}

	listItems := rootItemsWithLoading(syncItems, asyncSources)

	m := NewModel(listItems, "%1", nil, reg, generator.Context{Config: cfg}, theme.Default(), asyncSources, syncItems)
	m.list.SetSize(80, 40)

	// Simulate async result arriving
	asyncItems := []item.Item{
		{Type: "dir", Display: "~/async-dir", Action: item.ActionNextList, Data: map[string]string{"path": "/async"}},
	}
	result, _ := m.Update(sourceResultMsg{Name: "zoxide", Items: asyncItems})
	m = result.(Model)

	// htop (root action) + 2 inline entries (New window + Browse) = 3
	items := m.list.Items()
	if len(items) != 3 {
		var displays []string
		for _, li := range items {
			if it, ok := li.(item.Item); ok {
				displays = append(displays, it.Display)
			}
		}
		t.Fatalf("got %d items %v, want 3", len(items), displays)
	}

	var foundInline bool
	for _, li := range items {
		it := li.(item.Item)
		if it.Display == "~/async-dir » New window" {
			foundInline = true
			break
		}
	}
	if !foundInline {
		t.Error("expected inline item '~/async-dir » New window' after async rebuild")
	}
}
