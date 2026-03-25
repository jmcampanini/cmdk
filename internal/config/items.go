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

	return item.Item{
		Type:    "action",
		Source:  "config",
		Display: a.Name,
		Action:  action,
		Cmd:     a.Cmd,
		Icon:    a.Icon,
		Stages:  stages,
	}
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
