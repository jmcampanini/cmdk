package tui

import (
	"testing"

	"github.com/jmcampanini/cmdk/internal/config"
	"github.com/jmcampanini/cmdk/internal/generator"
	"github.com/jmcampanini/cmdk/internal/item"
)

func testInlineRegistry() *generator.Registry {
	reg := generator.NewRegistry()
	reg.Register("dir-actions", generator.NewActionsGenerator())
	reg.MapType("dir", "dir-actions")
	return reg
}

func TestExpandInline_PassthroughNonNextList(t *testing.T) {
	reg := testInlineRegistry()
	ctx := generator.Context{}

	items := []item.Item{
		{Type: "window", Display: "main:1 zsh", Action: item.ActionExecute},
		{Type: "action", Display: "htop", Action: item.ActionExecute},
	}

	result := expandInline(items, reg, ctx)

	if len(result) != 2 {
		t.Fatalf("got %d items, want 2", len(result))
	}
	if result[0].Display != "main:1 zsh" {
		t.Errorf("result[0].Display = %q, want main:1 zsh", result[0].Display)
	}
	if result[1].Display != "htop" {
		t.Errorf("result[1].Display = %q, want htop", result[1].Display)
	}
}

func TestExpandInline_ExpandsDirItems(t *testing.T) {
	cfg := &config.Config{
		Actions: []config.Action{
			{Name: "Browse", Cmd: "yazi {{sq .path}}", Matches: "dir", Icon: "\uf07c"},
		},
	}
	reg := testInlineRegistry()
	ctx := generator.Context{Config: cfg}

	items := []item.Item{
		{Type: "dir", Display: "~/projects", Action: item.ActionNextList,
			Data: map[string]string{"path": "/home/user/projects"}},
	}

	result := expandInline(items, reg, ctx)

	// "New window" builtin + "Browse" from config
	if len(result) != 2 {
		t.Fatalf("got %d items, want 2", len(result))
	}

	newWindow := result[0]
	if newWindow.Display != "~/projects » New window" {
		t.Errorf("Display = %q, want ~/projects » New window", newWindow.Display)
	}
	if newWindow.Value != "New window" {
		t.Errorf("Value = %q, want New window", newWindow.Value)
	}
	if newWindow.Type != "dir" {
		t.Errorf("Type = %q, want dir", newWindow.Type)
	}
	if newWindow.InlineParent == nil {
		t.Fatal("InlineParent is nil")
	}
	if newWindow.InlineParent.Display != "~/projects" {
		t.Errorf("InlineParent.Display = %q, want ~/projects", newWindow.InlineParent.Display)
	}
	if newWindow.Action != item.ActionExecute {
		t.Errorf("Action = %q, want execute", newWindow.Action)
	}
	if newWindow.Data["path"] != "/home/user/projects" {
		t.Errorf("Data[path] = %q, want /home/user/projects", newWindow.Data["path"])
	}

	browse := result[1]
	if browse.Display != "~/projects » Browse" {
		t.Errorf("Display = %q, want ~/projects » Browse", browse.Display)
	}
	if browse.Icon != "\uf07c" {
		t.Errorf("Icon = %q, want \\uf07c", browse.Icon)
	}
}

func TestExpandInline_IconDefaultsToCmd(t *testing.T) {
	cfg := &config.Config{
		Actions: []config.Action{
			{Name: "NoIcon", Cmd: "echo", Matches: "dir"},
		},
	}
	reg := testInlineRegistry()
	ctx := generator.Context{Config: cfg}

	items := []item.Item{
		{Type: "dir", Display: "~/foo", Action: item.ActionNextList,
			Data: map[string]string{"path": "/home/user/foo"}},
	}

	result := expandInline(items, reg, ctx)

	for _, it := range result {
		if it.Icon == "" {
			t.Errorf("item %q has empty icon, want fallback", it.Display)
		}
	}
}

func TestExpandInline_PreservesOriginalOnNoChildren(t *testing.T) {
	reg := generator.NewRegistry()
	reg.Register("empty-actions", func(_ []item.Item, _ generator.Context) []item.Item {
		return nil
	})
	reg.MapType("custom", "empty-actions")
	ctx := generator.Context{}

	items := []item.Item{
		{Type: "custom", Display: "no children", Action: item.ActionNextList},
	}

	result := expandInline(items, reg, ctx)

	if len(result) != 1 {
		t.Fatalf("got %d items, want 1", len(result))
	}
	if result[0].Display != "no children" {
		t.Errorf("Display = %q, want no children", result[0].Display)
	}
	if result[0].InlineParent != nil {
		t.Error("InlineParent should be nil for preserved item")
	}
}

func TestExpandInline_PreservesOriginalOnUnmappedType(t *testing.T) {
	reg := generator.NewRegistry()
	ctx := generator.Context{}

	items := []item.Item{
		{Type: "unknown", Display: "unmapped", Action: item.ActionNextList},
	}

	result := expandInline(items, reg, ctx)

	if len(result) != 1 {
		t.Fatalf("got %d items, want 1", len(result))
	}
	if result[0].Display != "unmapped" {
		t.Errorf("Display = %q, want unmapped", result[0].Display)
	}
}

func TestExpandInline_MultipleParents(t *testing.T) {
	reg := testInlineRegistry()
	ctx := generator.Context{}

	items := []item.Item{
		{Type: "dir", Display: "~/a", Action: item.ActionNextList,
			Data: map[string]string{"path": "/a"}},
		{Type: "dir", Display: "~/b", Action: item.ActionNextList,
			Data: map[string]string{"path": "/b"}},
	}

	result := expandInline(items, reg, ctx)

	// Each dir gets "New window" builtin
	if len(result) != 2 {
		t.Fatalf("got %d items, want 2", len(result))
	}
	if result[0].Display != "~/a » New window" {
		t.Errorf("result[0].Display = %q", result[0].Display)
	}
	if result[1].Display != "~/b » New window" {
		t.Errorf("result[1].Display = %q", result[1].Display)
	}
	if result[0].Data["path"] != "/a" {
		t.Errorf("result[0] path = %q, want /a", result[0].Data["path"])
	}
	if result[1].Data["path"] != "/b" {
		t.Errorf("result[1] path = %q, want /b", result[1].Data["path"])
	}
}

func TestExpandInline_MixedItems(t *testing.T) {
	reg := testInlineRegistry()
	ctx := generator.Context{}

	items := []item.Item{
		{Type: "action", Display: "htop", Action: item.ActionExecute},
		{Type: "dir", Display: "~/proj", Action: item.ActionNextList,
			Data: map[string]string{"path": "/proj"}},
		{Type: "window", Display: "main:1", Action: item.ActionExecute},
	}

	result := expandInline(items, reg, ctx)

	if len(result) != 3 {
		t.Fatalf("got %d items, want 3", len(result))
	}
	if result[0].Display != "htop" {
		t.Errorf("result[0].Display = %q, want htop", result[0].Display)
	}
	if result[1].Display != "~/proj » New window" {
		t.Errorf("result[1].Display = %q, want ~/proj » New window", result[1].Display)
	}
	if result[2].Display != "main:1" {
		t.Errorf("result[2].Display = %q, want main:1", result[2].Display)
	}
}
