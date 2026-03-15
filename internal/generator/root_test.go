package generator

import (
	"errors"
	"testing"

	"github.com/jmcampanini/cmdk/internal/item"
)

func TestRootGenerator_ReturnsWindows(t *testing.T) {
	mock := func() ([]item.Item, error) {
		return []item.Item{
			{Type: "window", Display: "a:1 zsh"},
			{Type: "window", Display: "b:1 vim"},
		}, nil
	}

	gen := NewRootGenerator(mock)
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

func TestRootGenerator_ErrorReturnsNil(t *testing.T) {
	mock := func() ([]item.Item, error) {
		return nil, errors.New("tmux not running")
	}

	gen := NewRootGenerator(mock)
	items := gen(nil, Context{})

	if items != nil {
		t.Errorf("expected nil, got %v", items)
	}
}

func TestRootGenerator_MultipleSources(t *testing.T) {
	windows := func() ([]item.Item, error) {
		return []item.Item{
			{Type: "window", Display: "main:1 zsh"},
		}, nil
	}
	dirs := func() ([]item.Item, error) {
		return []item.Item{
			{Type: "dir", Display: "/home/user"},
			{Type: "dir", Display: "/tmp"},
		}, nil
	}

	gen := NewRootGenerator(windows, dirs)
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
	good := func() ([]item.Item, error) {
		return []item.Item{
			{Type: "window", Display: "main:1 zsh"},
		}, nil
	}
	bad := func() ([]item.Item, error) {
		return nil, errors.New("zoxide not installed")
	}

	gen := NewRootGenerator(good, bad)
	items := gen(nil, Context{})

	if len(items) != 1 {
		t.Fatalf("got %d items, want 1", len(items))
	}
	if items[0].Display != "main:1 zsh" {
		t.Errorf("items[0].Display = %q, want %q", items[0].Display, "main:1 zsh")
	}
}

func TestRootGenerator_AllSourcesError(t *testing.T) {
	bad1 := func() ([]item.Item, error) {
		return nil, errors.New("fail 1")
	}
	bad2 := func() ([]item.Item, error) {
		return nil, errors.New("fail 2")
	}

	gen := NewRootGenerator(bad1, bad2)
	items := gen(nil, Context{})

	if items != nil {
		t.Errorf("expected nil, got %v", items)
	}
}
