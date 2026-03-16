package generator

import (
	"errors"
	"testing"

	"github.com/jmcampanini/cmdk/internal/config"
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

func TestIntegration_FourSourceTypes(t *testing.T) {
	windows := func() ([]item.Item, error) {
		return []item.Item{
			{Type: "window", Display: "main:1 zsh"},
		}, nil
	}
	dirs := func() ([]item.Item, error) {
		return []item.Item{
			{Type: "dir", Source: "zoxide", Display: "/projects"},
		}, nil
	}
	cwdDir := func() ([]item.Item, error) {
		return []item.Item{
			{Type: "dir", Source: "cwd", Display: "/home/user"},
		}, nil
	}
	cfg := &config.Config{
		Commands: []config.Command{
			{Name: "htop", Cmd: "htop"},
		},
	}

	gen := NewRootGenerator(windows, dirs, cwdDir, config.CommandItems(cfg))
	items := gen(nil, Context{})

	if len(items) != 4 {
		t.Fatalf("got %d items, want 4", len(items))
	}
	wantTypes := []string{"window", "dir", "dir", "cmd"}
	for i, wt := range wantTypes {
		if items[i].Type != wt {
			t.Errorf("items[%d].Type = %q, want %q", i, items[i].Type, wt)
		}
	}
}

func TestIntegration_ConfigCommandsInOrder(t *testing.T) {
	cfg := &config.Config{
		Commands: []config.Command{
			{Name: "alpha", Cmd: "echo alpha"},
			{Name: "beta", Cmd: "echo beta"},
			{Name: "gamma", Cmd: "echo gamma"},
		},
	}
	windows := func() ([]item.Item, error) {
		return []item.Item{{Type: "window", Display: "main:1 zsh"}}, nil
	}

	gen := NewRootGenerator(windows, config.CommandItems(cfg))
	items := gen(nil, Context{})

	if len(items) != 4 {
		t.Fatalf("got %d items, want 4", len(items))
	}
	wantNames := []string{"alpha", "beta", "gamma"}
	for i, w := range wantNames {
		got := items[i+1]
		if got.Display != w {
			t.Errorf("items[%d].Display = %q, want %q", i+1, got.Display, w)
		}
		if got.Action != item.ActionExecute {
			t.Errorf("items[%d].Action = %q, want execute", i+1, got.Action)
		}
	}
}

func TestIntegration_NilConfig(t *testing.T) {
	windows := func() ([]item.Item, error) {
		return []item.Item{{Type: "window", Display: "main:1 zsh"}}, nil
	}

	gen := NewRootGenerator(windows, config.CommandItems(nil))
	items := gen(nil, Context{})

	if len(items) != 1 {
		t.Fatalf("got %d items, want 1", len(items))
	}
	if items[0].Type != "window" {
		t.Errorf("items[0].Type = %q, want window", items[0].Type)
	}
}

func TestIntegration_MalformedConfig(t *testing.T) {
	windows := func() ([]item.Item, error) {
		return []item.Item{{Type: "window", Display: "main:1 zsh"}}, nil
	}
	dirs := func() ([]item.Item, error) {
		return []item.Item{{Type: "dir", Display: "/projects"}}, nil
	}

	gen := NewRootGenerator(windows, dirs, config.ErrorSource(errors.New("bad toml")), config.CommandItems(nil))
	items := gen(nil, Context{})

	if len(items) != 3 {
		t.Fatalf("got %d items, want 3", len(items))
	}
	if items[0].Type != "window" {
		t.Errorf("items[0].Type = %q, want window", items[0].Type)
	}
	if items[1].Type != "dir" {
		t.Errorf("items[1].Type = %q, want dir", items[1].Type)
	}
	errItem := items[2]
	if errItem.Source != "config" {
		t.Errorf("errItem.Source = %q, want config", errItem.Source)
	}
	if errItem.Action != "" {
		t.Errorf("errItem.Action = %q, want empty", errItem.Action)
	}
}
