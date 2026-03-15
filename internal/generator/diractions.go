package generator

import "github.com/jmcampanini/cmdk/internal/item"

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

		return []item.Item{{
			Type:    "cmd",
			Source:  "generator",
			Display: "New window",
			Action:  item.ActionExecute,
			Cmd:     "tmux new-window -c {{sq .path}}",
			Data:    map[string]string{"path": path},
		}}
	}
}
