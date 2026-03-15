package generator

import (
	"testing"

	"github.com/jmcampanini/cmdk/internal/item"
)

func TestRegisterAndGet(t *testing.T) {
	reg := NewRegistry()
	called := false
	reg.Register("test", func(accumulated []item.Item, ctx Context) []item.Item {
		called = true
		return nil
	})

	fn, err := reg.Get("test")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	fn(nil, Context{})
	if !called {
		t.Error("expected generator to be called")
	}
}

func TestGetUnknown(t *testing.T) {
	reg := NewRegistry()
	_, err := reg.Get("nope")
	if err == nil {
		t.Error("expected error for unknown generator")
	}
}

func TestResolveEmptyAccumulated(t *testing.T) {
	reg := NewRegistry()
	reg.Register("root", func(accumulated []item.Item, ctx Context) []item.Item {
		return []item.Item{{Display: "from-root"}}
	})
	reg.MapType("", "root")

	fn, err := reg.Resolve(nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	items := fn(nil, Context{})
	if len(items) != 1 || items[0].Display != "from-root" {
		t.Errorf("unexpected items: %v", items)
	}
}

func TestResolveUnknownType(t *testing.T) {
	reg := NewRegistry()
	accumulated := []item.Item{{Type: "unknown"}}
	_, err := reg.Resolve(accumulated)
	if err == nil {
		t.Error("expected error for unmapped type")
	}
}
