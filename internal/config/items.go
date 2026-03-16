package config

import (
	"fmt"

	"github.com/jmcampanini/cmdk/internal/item"
)

func CommandItems(cfg *Config) func() ([]item.Item, error) {
	return func() ([]item.Item, error) {
		if cfg == nil {
			return nil, nil
		}
		items := make([]item.Item, len(cfg.Commands))
		for i, cmd := range cfg.Commands {
			it := item.NewItem()
			it.Type = "cmd"
			it.Source = "config"
			it.Action = item.ActionExecute
			it.Cmd = cmd.Cmd
			it.Display = cmd.Name
			items[i] = it
		}
		return items, nil
	}
}

func ErrorSource(err error) func() ([]item.Item, error) {
	it := item.NewItem()
	it.Type = "cmd"
	it.Source = "config"
	it.Display = fmt.Sprintf("config error: %v", err)
	return func() ([]item.Item, error) {
		return []item.Item{it}, nil
	}
}
