// Package generator selects launcher items from configured and external sources.
package generator

import (
	"fmt"

	"github.com/jmcampanini/cmdk/internal/config"
	"github.com/jmcampanini/cmdk/internal/item"
)

// Func produces the next list from the selection stack and launcher context.
type Func func(accumulated []item.Item, ctx Context) []item.Item

// Context supplies the invoking pane and effective configuration to generators.
type Context struct {
	PaneID string
	Config config.Config
}

// Registry maps generator names and selected item types to list producers.
type Registry struct {
	generators map[string]Func
	typeMap    map[string]string
}

// NewRegistry creates an empty registry ready for generator registration.
func NewRegistry() *Registry {
	return &Registry{
		generators: make(map[string]Func),
		typeMap:    make(map[string]string),
	}
}

// Register binds a generator name, replacing any existing binding.
func (r *Registry) Register(name string, fn Func) {
	r.generators[name] = fn
}

// MapType chooses the generator used after selecting an item of the given type.
func (r *Registry) MapType(itemType string, generatorName string) {
	r.typeMap[itemType] = generatorName
}

// Get returns a registered generator or an error for an unknown name.
func (r *Registry) Get(name string) (Func, error) {
	fn, ok := r.generators[name]
	if !ok {
		return nil, fmt.Errorf("generator %q not found", name)
	}
	return fn, nil
}

// Resolve selects the generator for the last item, or the empty type at the root.
func (r *Registry) Resolve(accumulated []item.Item) (Func, error) {
	itemType := ""
	if len(accumulated) > 0 {
		itemType = accumulated[len(accumulated)-1].Type
	}

	name, ok := r.typeMap[itemType]
	if !ok {
		return nil, fmt.Errorf("no generator mapped for item type %q", itemType)
	}

	return r.Get(name)
}
