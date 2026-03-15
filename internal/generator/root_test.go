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
