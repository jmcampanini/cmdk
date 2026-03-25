package config

import (
	"context"

	"github.com/jmcampanini/cmdk/internal/item"
)

func (a Action) ToItem() item.Item {
	stages := make([]item.Stage, len(a.Stages))
	for j, s := range a.Stages {
		stages[j] = item.Stage{
			Type:    item.StageType(s.Type),
			Key:     s.Key,
			Text:    s.Text,
			Default: s.Default,
			Source:  s.Source,
		}
	}

	action := item.ActionExecute
	if len(stages) > 0 {
		action = item.ActionStaged
	}

	it := item.NewItem()
	it.Type = "action"
	it.Source = "config"
	it.Display = a.Name
	it.Action = action
	it.Cmd = a.Cmd
	it.Icon = a.Icon
	it.Stages = stages
	return it
}

func MatchingActions(cfg *Config, matchType string) func(context.Context) ([]item.Item, error) {
	return func(_ context.Context) ([]item.Item, error) {
		var items []item.Item
		for _, a := range cfg.Actions {
			if a.Matches != matchType {
				continue
			}
			items = append(items, a.ToItem())
		}
		return items, nil
	}
}
