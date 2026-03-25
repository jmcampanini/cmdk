package generator

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/jmcampanini/cmdk/internal/config"
	"github.com/jmcampanini/cmdk/internal/execute"
	"github.com/jmcampanini/cmdk/internal/item"
)

const integrationTestFetchTimeout = time.Second

func newIntegrationRootGenerator(sources ...Source) GeneratorFunc {
	return NewRootGenerator(integrationTestFetchTimeout, sources...)
}

func setupRegistry() *Registry {
	reg := NewRegistry()

	windows := Source{Name: "windows", Type: "window", Fetch: func(context.Context) ([]item.Item, error) {
		return []item.Item{
			{Type: "window", Display: "main:1 zsh", Action: item.ActionExecute,
				Cmd:  "tmux switch-client -t '{{.session}}:{{.window_index}}'",
				Data: map[string]string{"session": "main", "window_index": "1"}},
		}, nil
	}}
	dirs := Source{Name: "zoxide", Type: "dir", Fetch: func(context.Context) ([]item.Item, error) {
		return []item.Item{
			{Type: "dir", Display: "/home/user/projects", Action: item.ActionNextList,
				Data: map[string]string{"path": "/home/user/projects"}},
		}, nil
	}}

	reg.Register("root", newIntegrationRootGenerator(windows, dirs))
	reg.Register("dir-actions", NewActionsGenerator())
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

	allAccumulated := slices.Concat(accumulated, []item.Item{actionItems[0]})
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
	actionItem := item.Item{
		Type: "action",
		Cmd:  "tmux new-window -c {{sq .path}}",
		Data: map[string]string{},
	}

	all := []item.Item{dirItem, actionItem}
	data := execute.FlattenData(all)

	if data["path"] != "/home/user" {
		t.Errorf("path = %q, want /home/user", data["path"])
	}

	rendered, err := execute.RenderCmd(actionItem.Cmd, data)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if rendered != "tmux new-window -c '/home/user'" {
		t.Errorf("rendered = %q", rendered)
	}
}

func TestIntegration_ThreeSourceTypes(t *testing.T) {
	windows := Source{Name: "windows", Type: "window", Fetch: func(context.Context) ([]item.Item, error) {
		return []item.Item{
			{Type: "window", Display: "main:1 zsh"},
		}, nil
	}}
	dirs := Source{Name: "zoxide", Type: "dir", Fetch: func(context.Context) ([]item.Item, error) {
		return []item.Item{
			{Type: "dir", Source: "zoxide", Display: "/projects"},
		}, nil
	}}
	cfg := &config.Config{
		Actions: []config.Action{
			{Name: "htop", Cmd: "htop", Matches: "root"},
		},
	}

	gen := newIntegrationRootGenerator(windows, dirs, Source{Name: "actions", Type: "action", Fetch: config.MatchingActions(cfg, "root")})
	items := gen(nil, Context{})

	if len(items) != 3 {
		t.Fatalf("got %d items, want 3", len(items))
	}
	wantTypes := []string{"window", "dir", "action"}
	for i, wt := range wantTypes {
		if items[i].Type != wt {
			t.Errorf("items[%d].Type = %q, want %q", i, items[i].Type, wt)
		}
	}
}

func TestIntegration_ConfigActionsInOrder(t *testing.T) {
	cfg := &config.Config{
		Actions: []config.Action{
			{Name: "alpha", Cmd: "echo alpha", Matches: "root"},
			{Name: "beta", Cmd: "echo beta", Matches: "root"},
			{Name: "gamma", Cmd: "echo gamma", Matches: "root"},
		},
	}
	windows := Source{Name: "windows", Type: "window", Fetch: func(context.Context) ([]item.Item, error) {
		return []item.Item{{Type: "window", Display: "main:1 zsh"}}, nil
	}}

	gen := newIntegrationRootGenerator(windows, Source{Name: "actions", Type: "action", Fetch: config.MatchingActions(cfg, "root")})
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

func TestIntegration_EmptyConfig(t *testing.T) {
	windows := Source{Name: "windows", Type: "window", Fetch: func(context.Context) ([]item.Item, error) {
		return []item.Item{{Type: "window", Display: "main:1 zsh"}}, nil
	}}

	gen := newIntegrationRootGenerator(windows, Source{Name: "actions", Type: "action", Fetch: config.MatchingActions(&config.Config{}, "root")})
	items := gen(nil, Context{})

	if len(items) != 1 {
		t.Fatalf("got %d items, want 1", len(items))
	}
	if items[0].Type != "window" {
		t.Errorf("items[0].Type = %q, want window", items[0].Type)
	}
}

func TestIntegration_ExecuteWithEnvVars(t *testing.T) {
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
	selected := actionItems[0]

	all := slices.Concat(accumulated, []item.Item{selected})

	envVars := execute.BuildCMDKEnvVars(all, "%5")
	envMap := envSliceToMap(envVars)

	if envMap["CMDK_PATH"] != "/home/user/projects" {
		t.Errorf("CMDK_PATH = %q, want /home/user/projects", envMap["CMDK_PATH"])
	}
	if envMap["CMDK_PANE_ID"] != "%5" {
		t.Errorf("CMDK_PANE_ID = %q, want %%5", envMap["CMDK_PANE_ID"])
	}
}

func TestIntegration_MalformedConfig(t *testing.T) {
	windows := Source{Name: "windows", Type: "window", Fetch: func(context.Context) ([]item.Item, error) {
		return []item.Item{{Type: "window", Display: "main:1 zsh"}}, nil
	}}
	dirs := Source{Name: "zoxide", Type: "dir", Fetch: func(context.Context) ([]item.Item, error) {
		return []item.Item{{Type: "dir", Display: "/projects"}}, nil
	}}
	cfgErr := errors.New("bad toml")
	badConfig := Source{Name: "config", Type: "action", Fetch: func(context.Context) ([]item.Item, error) {
		return nil, cfgErr
	}}

	gen := newIntegrationRootGenerator(windows, dirs, badConfig, Source{Name: "actions", Type: "action", Fetch: config.MatchingActions(&config.Config{}, "root")})
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

func TestIntegration_OneSourceFailsOthersWork(t *testing.T) {
	windows := Source{Name: "windows", Type: "window", Fetch: func(context.Context) ([]item.Item, error) {
		return []item.Item{{Type: "window", Display: "main:1 zsh"}}, nil
	}}
	badDirs := Source{Name: "zoxide", Type: "dir", Fetch: func(context.Context) ([]item.Item, error) {
		return nil, errors.New("command not found")
	}}
	actions := Source{Name: "actions", Type: "action", Fetch: func(context.Context) ([]item.Item, error) {
		return []item.Item{{Type: "action", Display: "htop", Action: item.ActionExecute}}, nil
	}}

	gen := newIntegrationRootGenerator(windows, badDirs, actions)
	items := gen(nil, Context{})

	if len(items) != 3 {
		t.Fatalf("got %d items, want 3", len(items))
	}
	if items[0].Type != "window" {
		t.Errorf("items[0] = %q, want window", items[0].Type)
	}
	if items[1].Display != "zoxide error: command not found" {
		t.Errorf("items[1].Display = %q", items[1].Display)
	}
	if items[2].Display != "htop" {
		t.Errorf("items[2].Display = %q, want htop", items[2].Display)
	}
}

func TestIntegration_DirActionsWithConfig(t *testing.T) {
	cfg := &config.Config{
		Actions: []config.Action{
			{Name: "Yazi", Cmd: "tmux split-window -h -t {{sq .pane_id}} -c {{sq .path}} yazi", Matches: "dir"},
			{Name: "New pane", Cmd: "tmux split-window -v -c {{sq .path}}", Matches: "dir"},
		},
	}
	reg := setupRegistry()
	ctx := Context{PaneID: "%5", Config: cfg}

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

	if len(actionItems) != 3 {
		t.Fatalf("action items = %d, want 3", len(actionItems))
	}
	if actionItems[0].Display != "New window" {
		t.Errorf("actionItems[0].Display = %q, want New window", actionItems[0].Display)
	}
	if actionItems[1].Display != "Yazi" {
		t.Errorf("actionItems[1].Display = %q, want Yazi", actionItems[1].Display)
	}
	if actionItems[2].Display != "New pane" {
		t.Errorf("actionItems[2].Display = %q, want New pane", actionItems[2].Display)
	}
}

func TestIntegration_DirActionConfigCmdRendersWithPathAndPaneID(t *testing.T) {
	cfg := &config.Config{
		Actions: []config.Action{
			{Name: "Yazi", Cmd: "tmux split-window -h -t {{sq .pane_id}} -c {{sq .path}} yazi", Matches: "dir"},
		},
	}
	reg := setupRegistry()
	ctx := Context{PaneID: "%5", Config: cfg}

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

	yaziItem := actionItems[1]
	allAccumulated := slices.Concat(accumulated, []item.Item{yaziItem})
	data := execute.FlattenData(allAccumulated)
	rendered, err := execute.RenderCmd(yaziItem.Cmd, data)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	want := "tmux split-window -h -t '%5' -c '/home/user/projects' yazi"
	if rendered != want {
		t.Errorf("rendered = %q, want %q", rendered, want)
	}
}

func TestIntegration_DirActionsEmptyConfig(t *testing.T) {
	cfg := &config.Config{Actions: []config.Action{}}
	reg := setupRegistry()
	ctx := Context{PaneID: "%1", Config: cfg}

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
		t.Fatalf("action items = %d, want 1 (only New window)", len(actionItems))
	}
	if actionItems[0].Display != "New window" {
		t.Errorf("Display = %q, want New window", actionItems[0].Display)
	}
}

func envSliceToMap(envs []string) map[string]string {
	m := make(map[string]string)
	for _, e := range envs {
		k, v, ok := strings.Cut(e, "=")
		if ok {
			m[k] = v
		}
	}
	return m
}
