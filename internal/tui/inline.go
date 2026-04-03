package tui

import (
	"github.com/jmcampanini/cmdk/internal/generator"
	"github.com/jmcampanini/cmdk/internal/item"
)

const inlineSeparator = " » "

func expandInline(items []item.Item, registry *generator.Registry, ctx generator.Context) []item.Item {
	var result []item.Item
	for _, it := range items {
		if it.Action != item.ActionNextList {
			result = append(result, it)
			continue
		}

		gen, err := registry.Resolve([]item.Item{it})
		if err != nil {
			result = append(result, it)
			continue
		}

		children := gen([]item.Item{it}, ctx)
		if len(children) == 0 {
			result = append(result, it)
			continue
		}

		parent := it
		for _, child := range children {
			inline := child
			inline.Display = parent.Display + inlineSeparator + child.Display
			inline.Value = child.Display
			inline.Type = parent.Type
			if inline.Icon == "" {
				inline.Icon = iconCmd
			}
			inline.InlineParent = &parent
			result = append(result, inline)
		}
	}
	return result
}
