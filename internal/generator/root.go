package generator

import "github.com/jmcampanini/cmdk/internal/item"

func NewRootGenerator(sources ...func() ([]item.Item, error)) GeneratorFunc {
	return func(accumulated []item.Item, ctx Context) []item.Item {
		var all []item.Item
		for _, src := range sources {
			items, err := src()
			if err != nil {
				continue // TODO(M5): return error items instead of swallowing
			}
			all = append(all, items...)
		}
		return all
	}
}
