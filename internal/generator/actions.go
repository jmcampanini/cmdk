package generator

import (
	"maps"

	"github.com/jmcampanini/cmdk/internal/item"
)

func NewActionsGenerator() GeneratorFunc {
	return func(accumulated []item.Item, ctx Context) []item.Item {
		if len(accumulated) == 0 {
			return nil
		}
		last := accumulated[len(accumulated)-1]
		matchType := last.Type

		if matchType == "dir" && last.Data["path"] == "" {
			return nil
		}

		data := make(map[string]string)
		maps.Copy(data, last.Data)
		if ctx.PaneID != "" {
			data["pane_id"] = ctx.PaneID
		}

		var items []item.Item

		if matchType == "dir" {
			items = append(items, item.Item{
				Type:    "action",
				Source:  "builtin",
				Display: "New window",
				Action:  item.ActionExecute,
				Cmd:     "tmux new-window -c {{sq .path}}",
				Data:    maps.Clone(data),
				Icon:    "\uf2d0",
			})
		}

		if ctx.Config != nil {
			for _, a := range ctx.Config.Actions {
				if a.Matches != matchType {
					continue
				}
				it := a.ToItem()
				it.Data = maps.Clone(data)
				items = append(items, it)
			}
		}

		return items
	}
}
