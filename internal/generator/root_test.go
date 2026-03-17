package generator

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jmcampanini/cmdk/internal/item"
)

const rootTestFetchTimeout = time.Second

func newRootTestGenerator(sources ...Source) GeneratorFunc {
	return NewRootGenerator(rootTestFetchTimeout, sources...)
}

func TestRootGenerator_ReturnsWindows(t *testing.T) {
	src := Source{Name: "windows", Type: "window", Fetch: func(context.Context) ([]item.Item, error) {
		return []item.Item{
			{Type: "window", Display: "a:1 zsh"},
			{Type: "window", Display: "b:1 vim"},
		}, nil
	}}

	gen := newRootTestGenerator(src)
	items := gen(nil, Context{})

	if len(items) != 2 {
		t.Fatalf("got %d items, want 2", len(items))
	}
	if items[0].Display != "a:1 zsh" {
		t.Errorf("items[0].Display = %q, want %q", items[0].Display, "a:1 zsh")
	}
	if items[1].Display != "b:1 vim" {
		t.Errorf("items[1].Display = %q, want %q", items[1].Display, "b:1 vim")
	}
}

func TestRootGenerator_ErrorProducesErrorItem(t *testing.T) {
	src := Source{Name: "zoxide", Type: "dir", Fetch: func(context.Context) ([]item.Item, error) {
		return nil, errors.New("not installed")
	}}

	gen := newRootTestGenerator(src)
	items := gen(nil, Context{})

	if len(items) != 1 {
		t.Fatalf("got %d items, want 1", len(items))
	}
	errItem := items[0]
	if errItem.Type != "dir" {
		t.Errorf("Type = %q, want dir", errItem.Type)
	}
	if errItem.Source != "zoxide" {
		t.Errorf("Source = %q, want zoxide", errItem.Source)
	}
	if errItem.Display != "zoxide error: not installed" {
		t.Errorf("Display = %q", errItem.Display)
	}
	if errItem.Action != "" {
		t.Errorf("Action = %q, want empty (non-selectable)", errItem.Action)
	}
	if errItem.Cmd != "" {
		t.Errorf("Cmd = %q, want empty", errItem.Cmd)
	}
}

func TestRootGenerator_MultipleSources(t *testing.T) {
	windows := Source{Name: "windows", Type: "window", Fetch: func(context.Context) ([]item.Item, error) {
		return []item.Item{
			{Type: "window", Display: "main:1 zsh"},
		}, nil
	}}
	dirs := Source{Name: "zoxide", Type: "dir", Fetch: func(context.Context) ([]item.Item, error) {
		return []item.Item{
			{Type: "dir", Display: "/home/user"},
			{Type: "dir", Display: "/tmp"},
		}, nil
	}}

	gen := newRootTestGenerator(windows, dirs)
	items := gen(nil, Context{})

	if len(items) != 3 {
		t.Fatalf("got %d items, want 3", len(items))
	}
	if items[0].Type != "window" {
		t.Errorf("items[0].Type = %q, want %q", items[0].Type, "window")
	}
	if items[1].Type != "dir" {
		t.Errorf("items[1].Type = %q, want %q", items[1].Type, "dir")
	}
}

func TestRootGenerator_OneSourceErrors(t *testing.T) {
	good := Source{Name: "windows", Type: "window", Fetch: func(context.Context) ([]item.Item, error) {
		return []item.Item{
			{Type: "window", Display: "main:1 zsh"},
		}, nil
	}}
	bad := Source{Name: "zoxide", Type: "dir", Fetch: func(context.Context) ([]item.Item, error) {
		return nil, errors.New("zoxide not installed")
	}}

	gen := newRootTestGenerator(good, bad)
	items := gen(nil, Context{})

	if len(items) != 2 {
		t.Fatalf("got %d items, want 2", len(items))
	}
	if items[0].Display != "main:1 zsh" {
		t.Errorf("items[0].Display = %q, want %q", items[0].Display, "main:1 zsh")
	}
	if items[1].Type != "dir" {
		t.Errorf("items[1].Type = %q, want dir (error item keeps source type)", items[1].Type)
	}
	if items[1].Action != "" {
		t.Errorf("items[1].Action = %q, want empty", items[1].Action)
	}
}

func TestRootGenerator_AllSourcesError(t *testing.T) {
	bad1 := Source{Name: "src1", Type: "window", Fetch: func(context.Context) ([]item.Item, error) {
		return nil, errors.New("fail 1")
	}}
	bad2 := Source{Name: "src2", Type: "dir", Fetch: func(context.Context) ([]item.Item, error) {
		return nil, errors.New("fail 2")
	}}

	gen := newRootTestGenerator(bad1, bad2)
	items := gen(nil, Context{})

	if len(items) != 2 {
		t.Fatalf("got %d items, want 2", len(items))
	}
	if items[0].Type != "window" {
		t.Errorf("items[0].Type = %q, want window", items[0].Type)
	}
	if items[1].Type != "dir" {
		t.Errorf("items[1].Type = %q, want dir", items[1].Type)
	}
}

func TestRootGenerator_ErrorItemInDirGroup(t *testing.T) {
	windows := Source{Name: "windows", Type: "window", Fetch: func(context.Context) ([]item.Item, error) {
		return []item.Item{{Type: "window", Display: "main:1 zsh"}}, nil
	}}
	badDirs := Source{Name: "zoxide", Type: "dir", Fetch: func(context.Context) ([]item.Item, error) {
		return nil, errors.New("command not found")
	}}
	cmds := Source{Name: "commands", Type: "cmd", Fetch: func(context.Context) ([]item.Item, error) {
		return []item.Item{{Type: "cmd", Display: "htop", Action: item.ActionExecute}}, nil
	}}

	gen := newRootTestGenerator(windows, badDirs, cmds)
	items := gen(nil, Context{})

	if len(items) != 3 {
		t.Fatalf("got %d items, want 3", len(items))
	}

	ordered := item.GroupAndOrder(items)
	if len(ordered) != 3 {
		t.Fatalf("ordered = %d, want 3", len(ordered))
	}
	got0 := ordered[0].(item.Item)
	got1 := ordered[1].(item.Item)
	got2 := ordered[2].(item.Item)

	if got0.Type != "window" {
		t.Errorf("ordered[0].Type = %q, want window", got0.Type)
	}
	if got1.Type != "dir" {
		t.Errorf("ordered[1].Type = %q, want dir (error item)", got1.Type)
	}
	if got2.Type != "cmd" {
		t.Errorf("ordered[2].Type = %q, want cmd", got2.Type)
	}
}

func TestRootGenerator_PreservesSourceOrderWhenConcurrent(t *testing.T) {
	firstStarted := make(chan struct{})
	secondDone := make(chan struct{})
	releaseFirst := make(chan struct{})

	first := Source{Name: "windows", Type: "window", Fetch: func(context.Context) ([]item.Item, error) {
		close(firstStarted)
		select {
		case <-releaseFirst:
		case <-time.After(250 * time.Millisecond):
			return nil, errors.New("first source was not released")
		}
		return []item.Item{{Type: "window", Display: "main:1 zsh"}}, nil
	}}
	second := Source{Name: "commands", Type: "cmd", Fetch: func(context.Context) ([]item.Item, error) {
		<-firstStarted
		close(secondDone)
		return []item.Item{{Type: "cmd", Display: "htop"}}, nil
	}}

	go func() {
		<-secondDone
		close(releaseFirst)
	}()

	gen := newRootTestGenerator(first, second)
	items := gen(nil, Context{})

	if len(items) != 2 {
		t.Fatalf("got %d items, want 2", len(items))
	}
	if items[0].Display != "main:1 zsh" {
		t.Errorf("items[0].Display = %q, want %q", items[0].Display, "main:1 zsh")
	}
	if items[1].Display != "htop" {
		t.Errorf("items[1].Display = %q, want %q", items[1].Display, "htop")
	}
}

func TestRootGenerator_TimeoutProducesErrorItem(t *testing.T) {
	slow := Source{Name: "zoxide", Type: "dir", Fetch: func(ctx context.Context) ([]item.Item, error) {
		<-ctx.Done()
		return nil, ctx.Err()
	}}
	fast := Source{Name: "windows", Type: "window", Fetch: func(context.Context) ([]item.Item, error) {
		return []item.Item{{Type: "window", Display: "main:1 zsh"}}, nil
	}}

	gen := NewRootGenerator(10*time.Millisecond, fast, slow)
	items := gen(nil, Context{})

	if len(items) != 2 {
		t.Fatalf("got %d items, want 2", len(items))
	}
	if items[0].Display != "main:1 zsh" {
		t.Errorf("items[0].Display = %q, want %q", items[0].Display, "main:1 zsh")
	}
	if items[1].Type != "dir" {
		t.Errorf("items[1].Type = %q, want dir", items[1].Type)
	}
	if items[1].Display != "zoxide error: context deadline exceeded" {
		t.Errorf("items[1].Display = %q, want %q", items[1].Display, "zoxide error: context deadline exceeded")
	}
}
