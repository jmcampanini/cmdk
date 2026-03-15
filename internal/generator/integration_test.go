package generator

import (
	"testing"

	"github.com/jmcampanini/cmdk/internal/execute"
	"github.com/jmcampanini/cmdk/internal/item"
)

func setupRegistry() *Registry {
	reg := NewRegistry()

	windows := func() ([]item.Item, error) {
		return []item.Item{
			{Type: "window", Display: "main:1 zsh", Action: item.ActionExecute,
				Cmd: "tmux switch-client -t '{{.session}}:{{.window_index}}'",
				Data: map[string]string{"session": "main", "window_index": "1"}},
		}, nil
	}
	dirs := func() ([]item.Item, error) {
		return []item.Item{
			{Type: "dir", Display: "/home/user/projects", Action: item.ActionNextList,
				Data: map[string]string{"path": "/home/user/projects"}},
		}, nil
	}

	reg.Register("root", NewRootGenerator(windows, dirs))
	reg.Register("dir-actions", NewDirActionsGenerator())
	reg.MapType("", "root")
	reg.MapType("dir", "dir-actions")
	return reg
}

func TestIntegration_TwoLevelChain(t *testing.T) {
	reg := setupRegistry()
	ctx := Context{}

	gen, err := reg.Resolve(nil)
	if err != nil {
		t.Fatalf("resolve root: %v", err)
	}
	rootItems := gen(nil, ctx)

	if len(rootItems) != 2 {
		t.Fatalf("root items = %d, want 2", len(rootItems))
	}

	dirItem := rootItems[1]
	if dirItem.Type != "dir" {
		t.Fatalf("expected dir item, got %q", dirItem.Type)
	}

	accumulated := []item.Item{dirItem}
	gen, err = reg.Resolve(accumulated)
	if err != nil {
		t.Fatalf("resolve dir-actions: %v", err)
	}
	actionItems := gen(accumulated, ctx)

	if len(actionItems) != 1 {
		t.Fatalf("action items = %d, want 1", len(actionItems))
	}
	if actionItems[0].Display != "New window" {
		t.Errorf("action Display = %q, want %q", actionItems[0].Display, "New window")
	}

	allAccumulated := append(accumulated, actionItems[0])
	data := execute.FlattenData(allAccumulated)
	rendered, err := execute.RenderCmd(actionItems[0].Cmd, data)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if rendered != "tmux new-window -c '/home/user/projects'" {
		t.Errorf("rendered = %q", rendered)
	}
}

func TestIntegration_BackNavigation(t *testing.T) {
	reg := setupRegistry()
	ctx := Context{}

	gen, err := reg.Resolve(nil)
	if err != nil {
		t.Fatalf("resolve root: %v", err)
	}
	rootItems := gen(nil, ctx)

	dirItem := rootItems[1]
	accumulated := []item.Item{dirItem}

	gen, err = reg.Resolve(accumulated)
	if err != nil {
		t.Fatalf("resolve dir-actions: %v", err)
	}
	actionItems := gen(accumulated, ctx)
	if len(actionItems) != 1 {
		t.Fatalf("action items = %d, want 1", len(actionItems))
	}

	accumulated = accumulated[:0]
	gen, err = reg.Resolve(accumulated)
	if err != nil {
		t.Fatalf("resolve root after back: %v", err)
	}
	backItems := gen(accumulated, ctx)
	if len(backItems) != 2 {
		t.Fatalf("back items = %d, want 2", len(backItems))
	}
	if backItems[0].Type != "window" {
		t.Errorf("backItems[0].Type = %q, want window", backItems[0].Type)
	}
	if backItems[1].Type != "dir" {
		t.Errorf("backItems[1].Type = %q, want dir", backItems[1].Type)
	}
}

func TestIntegration_DataFlattening(t *testing.T) {
	dirItem := item.Item{
		Type: "dir",
		Data: map[string]string{"path": "/home/user"},
	}
	cmdItem := item.Item{
		Type: "cmd",
		Cmd:  "tmux new-window -c {{sq .path}}",
		Data: map[string]string{},
	}

	all := []item.Item{dirItem, cmdItem}
	data := execute.FlattenData(all)

	if data["path"] != "/home/user" {
		t.Errorf("path = %q, want /home/user", data["path"])
	}

	rendered, err := execute.RenderCmd(cmdItem.Cmd, data)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if rendered != "tmux new-window -c '/home/user'" {
		t.Errorf("rendered = %q", rendered)
	}
}
