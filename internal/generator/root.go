package generator

import (
	"fmt"

	"github.com/jmcampanini/cmdk/internal/item"
)

type Source struct {
	Name  string
	Type  string
	Fetch func() ([]item.Item, error)
}

func NewRootGenerator(sources ...Source) GeneratorFunc {
	return func(accumulated []item.Item, ctx Context) []item.Item {
		var all []item.Item
		for _, src := range sources {
			if src.Fetch == nil {
				errItem := item.NewItem()
				errItem.Type = src.Type
				errItem.Source = src.Name
				errItem.Display = fmt.Sprintf("%s error: no fetch function", src.Name)
				all = append(all, errItem)
				continue
			}
			items, err := src.Fetch()
			if err != nil {
				errItem := item.NewItem()
				errItem.Type = src.Type
				errItem.Source = src.Name
				errItem.Display = fmt.Sprintf("%s error: %s", src.Name, err)
				all = append(all, errItem)
				continue
			}
			all = append(all, items...)
		}
		return all
	}
}
