package tui

import (
	log "charm.land/log/v2"

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
			log.Warn("inline expand: no generator for type, keeping as drill-down", "type", it.Type, "error", err)
			result = append(result, it)
			continue
		}

		children := gen([]item.Item{it}, ctx)
		if len(children) == 0 {
			log.Warn("inline expand: generator returned no children, keeping as drill-down", "type", it.Type, "display", it.Display)
			result = append(result, it)
			continue
		}

		parent := it
		for _, child := range children {
			inline := child
			inline.Display = parent.Display + inlineSeparator + child.Display
			inline.Value = child.Display // original name, restored on selection for stack view
			// Preserve parent type so inline items sort with their directory group in GroupAndOrder.
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
