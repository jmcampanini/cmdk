package generator

import (
	"maps"

	"github.com/jmcampanini/cmdk/internal/item"
)

func NewDirActionsGenerator() GeneratorFunc {
	return func(accumulated []item.Item, ctx Context) []item.Item {
		if len(accumulated) == 0 {
			return nil
		}
		last := accumulated[len(accumulated)-1]
		path, ok := last.Data["path"]
		if !ok || path == "" {
			return nil
		}

		data := map[string]string{"path": path}
		if ctx.PaneID != "" {
			data["pane_id"] = ctx.PaneID
		}

		items := []item.Item{{
			Type:    "cmd",
			Source:  "generator",
			Display: "New window",
			Action:  item.ActionExecute,
			Cmd:     "tmux new-window -c {{sq .path}}",
			Data:    maps.Clone(data),
			Icon:    "\uf2d0",
		}}

		if ctx.Config != nil {
			for _, cmd := range ctx.Config.DirActions {
				items = append(items, item.Item{
					Type:    "cmd",
					Source:  "config",
					Display: cmd.Name,
					Action:  item.ActionExecute,
					Cmd:     cmd.Cmd,
					Data:    maps.Clone(data),
					Icon:    cmd.Icon,
				})
			}
		}

		return items
	}
}
