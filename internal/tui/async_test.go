package tui

import (
	"context"
	"errors"
	"testing"
	"time"

	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"

	"github.com/jmcampanini/cmdk/internal/config"
	"github.com/jmcampanini/cmdk/internal/generator"
	"github.com/jmcampanini/cmdk/internal/item"
	"github.com/jmcampanini/cmdk/internal/theme"
)

func asyncTestConfig() config.Config {
	return config.DefaultConfig()
}

func newAsyncTestModel(syncItems []item.Item, asyncSources []AsyncSource) Model {
	t := theme.Light()
	reg := generator.NewRegistry()
	reg.Register("root", func([]item.Item, generator.Context) []item.Item { return syncItems })
	reg.MapType("", "root")

	var initialAll []item.Item
	initialAll = append(initialAll, syncItems...)
	for _, src := range asyncSources {
		initialAll = append(initialAll, generator.LoadingItem(generator.Source{Name: src.Name, Type: src.Type}))
	}
	listItems := item.GroupAndOrder(initialAll, false)

	return NewModel(listItems, "%1", nil, reg, generator.Context{Config: asyncTestConfig()}, t, asyncSources, syncItems)
}

func sizeModel(m Model, width, height int) Model {
	result, _ := m.Update(tea.WindowSizeMsg{Width: width, Height: height})
	return result.(Model)
}

func itemDisplays(m Model) []string {
	var displays []string
	for _, li := range m.list.Items() {
		if it, ok := li.(item.Item); ok {
			displays = append(displays, it.Display)
		}
	}
	return displays
}

func contains(ss []string, target string) bool {
	for _, s := range ss {
		if s == target {
			return true
		}
	}
	return false
}

func TestInit_NoAsyncSources_ReturnsNil(t *testing.T) {
	m := newAsyncTestModel(
		[]item.Item{{Type: "action", Display: "test", Action: item.ActionExecute}},
		nil,
	)
	if cmd := m.Init(); cmd != nil {
		t.Error("Init() should return nil when no async sources")
	}
}

func TestInit_WithAsyncSources_ReturnsBatch(t *testing.T) {
	m := newAsyncTestModel(
		[]item.Item{{Type: "action", Display: "test", Action: item.ActionExecute}},
		[]AsyncSource{
			{Name: "windows", Type: "window", Timeout: time.Second, Fetch: func(context.Context) ([]item.Item, error) {
				return nil, nil
			}},
		},
	)
	if cmd := m.Init(); cmd == nil {
		t.Error("Init() should return a non-nil cmd when async sources are present")
	}
}

func TestSourceResult_ReplacesPlaceholder(t *testing.T) {
	m := newAsyncTestModel(
		[]item.Item{{Type: "action", Display: "do stuff", Action: item.ActionExecute}},
		[]AsyncSource{
			{Name: "windows", Type: "window", Timeout: time.Second},
		},
	)
	m = sizeModel(m, 80, 24)

	displays := itemDisplays(m)
	if !contains(displays, "Loading windows\u2026") {
		t.Fatalf("expected loading placeholder, got %v", displays)
	}

	result, _ := m.Update(sourceResultMsg{
		Name: "windows",
		Items: []item.Item{
			{Type: "window", Display: "main:1 zsh", Action: item.ActionExecute},
			{Type: "window", Display: "dev:1 node", Action: item.ActionExecute},
		},
	})
	m = result.(Model)

	displays = itemDisplays(m)
	if contains(displays, "Loading windows\u2026") {
		t.Error("loading placeholder should be gone after result")
	}
	if !contains(displays, "main:1 zsh") || !contains(displays, "dev:1 node") {
		t.Errorf("expected real items, got %v", displays)
	}
}

func TestSourceResult_ErrorReplacesPlaceholder(t *testing.T) {
	m := newAsyncTestModel(
		[]item.Item{{Type: "action", Display: "do stuff", Action: item.ActionExecute}},
		[]AsyncSource{
			{Name: "zoxide", Type: "dir", Timeout: time.Second},
		},
	)
	m = sizeModel(m, 80, 24)

	result, _ := m.Update(sourceResultMsg{
		Name: "zoxide",
		Err:  errors.New("not installed"),
	})
	m = result.(Model)

	displays := itemDisplays(m)
	if contains(displays, "Loading zoxide\u2026") {
		t.Error("loading placeholder should be gone")
	}
	if !contains(displays, "zoxide error: not installed") {
		t.Errorf("expected error item, got %v", displays)
	}
}

func TestSourceResult_PreservesFilterText(t *testing.T) {
	m := newAsyncTestModel(
		[]item.Item{{Type: "action", Display: "do stuff", Action: item.ActionExecute}},
		[]AsyncSource{
			{Name: "windows", Type: "window", Timeout: time.Second},
		},
	)
	m = sizeModel(m, 80, 24)

	if m.list.FilterState() != list.Filtering {
		t.Fatal("model should start in filter mode")
	}

	for _, r := range "main" {
		result, cmd := m.Update(tea.KeyPressMsg{Code: r, Text: string(r)})
		m = result.(Model)
		if cmd != nil {
			result, _ = m.Update(cmd())
			m = result.(Model)
		}
	}

	filterBefore := m.list.FilterInput.Value()
	if filterBefore != "main" {
		t.Fatalf("expected filter 'main', got %q", filterBefore)
	}

	result, cmd := m.Update(sourceResultMsg{
		Name: "windows",
		Items: []item.Item{
			{Type: "window", Display: "main:1 zsh", Action: item.ActionExecute},
		},
	})
	m = result.(Model)
	if cmd != nil {
		result, _ = m.Update(cmd())
		m = result.(Model)
	}

	filterAfter := m.list.FilterInput.Value()
	if filterAfter != "main" {
		t.Errorf("filter text changed from %q to %q", filterBefore, filterAfter)
	}
}

func TestSourceResult_IncrementalArrival(t *testing.T) {
	m := newAsyncTestModel(
		[]item.Item{{Type: "action", Display: "do stuff", Action: item.ActionExecute}},
		[]AsyncSource{
			{Name: "windows", Type: "window", Timeout: time.Second},
			{Name: "zoxide", Type: "dir", Timeout: time.Second},
		},
	)
	m = sizeModel(m, 80, 24)

	displays := itemDisplays(m)
	if !contains(displays, "Loading windows\u2026") || !contains(displays, "Loading zoxide\u2026") {
		t.Fatalf("expected two loading placeholders, got %v", displays)
	}

	result, _ := m.Update(sourceResultMsg{
		Name:  "windows",
		Items: []item.Item{{Type: "window", Display: "main:1 zsh", Action: item.ActionExecute}},
	})
	m = result.(Model)

	displays = itemDisplays(m)
	if contains(displays, "Loading windows\u2026") {
		t.Error("windows placeholder should be gone")
	}
	if !contains(displays, "Loading zoxide\u2026") {
		t.Error("zoxide placeholder should still be present")
	}
	if !contains(displays, "main:1 zsh") {
		t.Error("windows items should be present")
	}

	result, _ = m.Update(sourceResultMsg{
		Name:  "zoxide",
		Items: []item.Item{{Type: "dir", Display: "~/projects", Action: item.ActionNextList, Data: map[string]string{"path": "~/projects"}}},
	})
	m = result.(Model)

	displays = itemDisplays(m)
	if contains(displays, "Loading zoxide\u2026") {
		t.Error("zoxide placeholder should be gone")
	}
	if !contains(displays, "~/projects") {
		t.Error("zoxide items should be present")
	}
}

func TestSourceResult_BaseItemsPreserved(t *testing.T) {
	syncItems := []item.Item{
		{Type: "action", Display: "sync action 1", Action: item.ActionExecute},
		{Type: "action", Display: "sync action 2", Action: item.ActionExecute},
	}
	m := newAsyncTestModel(syncItems, []AsyncSource{
		{Name: "windows", Type: "window", Timeout: time.Second},
	})
	m = sizeModel(m, 80, 24)

	result, _ := m.Update(sourceResultMsg{
		Name:  "windows",
		Items: []item.Item{{Type: "window", Display: "main:1 zsh", Action: item.ActionExecute}},
	})
	m = result.(Model)

	displays := itemDisplays(m)
	if !contains(displays, "sync action 1") || !contains(displays, "sync action 2") {
		t.Errorf("base items should be preserved, got %v", displays)
	}
}

func TestSourceResult_GroupAndOrderApplied(t *testing.T) {
	syncItems := []item.Item{
		{Type: "action", Display: "my action", Action: item.ActionExecute},
	}
	m := newAsyncTestModel(syncItems, []AsyncSource{
		{Name: "windows", Type: "window", Timeout: time.Second},
		{Name: "zoxide", Type: "dir", Timeout: time.Second},
	})
	m = sizeModel(m, 80, 24)

	result, _ := m.Update(sourceResultMsg{
		Name:  "windows",
		Items: []item.Item{{Type: "window", Display: "main:1 zsh", Action: item.ActionExecute}},
	})
	m = result.(Model)
	result, _ = m.Update(sourceResultMsg{
		Name:  "zoxide",
		Items: []item.Item{{Type: "dir", Display: "~/code", Action: item.ActionNextList, Data: map[string]string{"path": "~/code"}}},
	})
	m = result.(Model)

	displays := itemDisplays(m)
	actionIdx, dirIdx, windowIdx := -1, -1, -1
	for i, d := range displays {
		switch d {
		case "my action":
			actionIdx = i
		case "~/code":
			dirIdx = i
		case "main:1 zsh":
			windowIdx = i
		}
	}
	if actionIdx >= dirIdx || dirIdx >= windowIdx {
		t.Errorf("expected order action < dir < window, got action=%d dir=%d window=%d in %v",
			actionIdx, dirIdx, windowIdx, displays)
	}
}

func TestLoadingItem_InertOnEnter(t *testing.T) {
	m := newAsyncTestModel(nil, []AsyncSource{
		{Name: "windows", Type: "window", Timeout: time.Second},
	})
	m = sizeModel(m, 80, 24)
	m = exitFilterMode(t, m)

	result, cmd := m.Update(enterMsg)
	m = result.(Model)

	if m.Selected() != nil {
		t.Error("selecting a loading item should not set Selected")
	}
	if cmd != nil {
		if _, isQuit := cmd().(tea.QuitMsg); isQuit {
			t.Error("selecting a loading item should not quit")
		}
	}
}

func TestSourceResult_WhileDrilledDown_DoesNotRebuild(t *testing.T) {
	syncItems := []item.Item{
		{Type: "dir", Display: "~/projects", Action: item.ActionNextList, Data: map[string]string{"path": "~/projects"}},
	}
	asyncSrcs := []AsyncSource{
		{Name: "windows", Type: "window", Timeout: time.Second},
	}
	th := theme.Light()
	reg := generator.NewRegistry()
	reg.Register("root", func([]item.Item, generator.Context) []item.Item { return syncItems })
	reg.Register("dir-actions", func([]item.Item, generator.Context) []item.Item {
		return []item.Item{
			{Type: "action", Display: "New window", Action: item.ActionExecute, Cmd: "tmux new-window"},
		}
	})
	reg.MapType("", "root")
	reg.MapType("dir", "dir-actions")

	var initialAll []item.Item
	initialAll = append(initialAll, syncItems...)
	for _, src := range asyncSrcs {
		initialAll = append(initialAll, generator.LoadingItem(generator.Source{Name: src.Name, Type: src.Type}))
	}
	listItems := item.GroupAndOrder(initialAll, false)

	m := NewModel(listItems, "%1", nil, reg, generator.Context{Config: asyncTestConfig()}, th, asyncSrcs, syncItems)
	m = sizeModel(m, 80, 24)
	m = exitFilterMode(t, m)

	result, _ := m.Update(enterMsg)
	m = result.(Model)

	if len(m.accumulated) == 0 {
		t.Fatal("should have drilled down into dir")
	}
	dirDisplays := itemDisplays(m)
	if !contains(dirDisplays, "New window") {
		t.Fatal("should show dir-actions list")
	}

	result, _ = m.Update(sourceResultMsg{
		Name:  "windows",
		Items: []item.Item{{Type: "window", Display: "main:1 zsh", Action: item.ActionExecute}},
	})
	m = result.(Model)

	afterDisplays := itemDisplays(m)
	if !contains(afterDisplays, "New window") {
		t.Error("dir-actions list should not change when async result arrives while drilled down")
	}
	if contains(afterDisplays, "main:1 zsh") {
		t.Error("async results should not appear in drilled-down view")
	}
}

func TestFetchSourceCmd_PanicRecovery(t *testing.T) {
	src := AsyncSource{
		Name:    "panicky",
		Type:    "window",
		Timeout: time.Second,
		Fetch: func(context.Context) ([]item.Item, error) {
			panic("test panic")
		},
	}

	cmd := fetchSourceCmd(src)
	msg := cmd().(sourceResultMsg)

	if msg.Name != "panicky" {
		t.Errorf("Name = %q, want panicky", msg.Name)
	}
	if msg.Err == nil {
		t.Fatal("expected error from panic, got nil")
	}
	if msg.Items != nil {
		t.Errorf("Items should be nil on panic, got %v", msg.Items)
	}
}

func TestFetchSourceCmd_Limit(t *testing.T) {
	src := AsyncSource{
		Name:    "dirs",
		Type:    "dir",
		Limit:   2,
		Timeout: time.Second,
		Fetch: func(context.Context) ([]item.Item, error) {
			return []item.Item{
				{Type: "dir", Display: "/a"},
				{Type: "dir", Display: "/b"},
				{Type: "dir", Display: "/c"},
			}, nil
		},
	}

	cmd := fetchSourceCmd(src)
	msg := cmd().(sourceResultMsg)

	if len(msg.Items) != 2 {
		t.Fatalf("got %d items, want 2", len(msg.Items))
	}
}
