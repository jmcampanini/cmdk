package generator

import (
	"fmt"

	"github.com/jmcampanini/cmdk/internal/config"
	"github.com/jmcampanini/cmdk/internal/item"
)

type GeneratorFunc func(accumulated []item.Item, ctx Context) []item.Item

type Context struct {
	PaneID string
	Config *config.Config
}

type Registry struct {
	generators map[string]GeneratorFunc
	typeMap    map[string]string
}

func NewRegistry() *Registry {
	return &Registry{
		generators: make(map[string]GeneratorFunc),
		typeMap:    make(map[string]string),
	}
}

func (r *Registry) Register(name string, fn GeneratorFunc) {
	r.generators[name] = fn
}

func (r *Registry) MapType(itemType string, generatorName string) {
	r.typeMap[itemType] = generatorName
}

func (r *Registry) Get(name string) (GeneratorFunc, error) {
	fn, ok := r.generators[name]
	if !ok {
		return nil, fmt.Errorf("generator %q not found", name)
	}
	return fn, nil
}

func (r *Registry) Resolve(accumulated []item.Item) (GeneratorFunc, error) {
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
