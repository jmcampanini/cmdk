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

func testConfig() *config.Config {
	cfg := config.DefaultConfig()
	return &cfg
}

func newTestModelWithTheme(items []list.Item, reg *generator.Registry, t theme.Theme) Model {
	return NewModel(items, "%1", nil, reg, generator.Context{Config: testConfig()}, t)
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

func TestDownDuringEmptyFilter_ExitsFilterWithoutMoving(t *testing.T) {
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
	if m.list.Index() != 0 {
		t.Errorf("Index() = %d, want 0 (Down should only exit filter, not move cursor)", m.list.Index())
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
	if m.list.Index() != 0 {
		t.Errorf("Index() = %d, want 0 (Up should only exit filter, not move cursor)", m.list.Index())
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

func TestPickerStage_EnterOnErrorItem_NoOp(t *testing.T) {
	m := newTestModel(pickerErrorItems(), testRegistry())
	m = setWindowSize(t, m, 80, 40)

	m = selectStagedItem(t, m)
	if m.mode != viewPicker {
		t.Fatal("expected viewPicker")
	}

	// Exit filter mode, then press Enter on the error item.
	result, _ := m.Update(escMsg)
	m = result.(Model)
	result, cmd := m.Update(enterMsg)
	m = result.(Model)

	if m.Selected() != nil {
		t.Error("Selected() should be nil — error items are not selectable")
	}
	if m.mode != viewPicker {
		t.Errorf("mode = %d, want viewPicker (%d)", m.mode, viewPicker)
	}
	if cmd != nil {
		t.Error("Enter on error item should not produce a quit command")
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
