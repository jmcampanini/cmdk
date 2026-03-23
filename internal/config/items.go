package config

import (
	"context"

	"github.com/jmcampanini/cmdk/internal/item"
)

func CommandItems(cfg *Config) func(context.Context) ([]item.Item, error) {
	return func(_ context.Context) ([]item.Item, error) {
		items := make([]item.Item, len(cfg.Commands))
		for i, cmd := range cfg.Commands {
			it := item.NewItem()
			it.Type = "cmd"
			it.Source = "config"
			it.Action = item.ActionExecute
			it.Cmd = cmd.Cmd
			it.Display = cmd.Name
			it.Icon = cmd.Icon
			items[i] = it
		}
		return items, nil
	}
}
