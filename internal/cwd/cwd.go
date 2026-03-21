package cwd

import (
	"context"
	"os"

	"github.com/jmcampanini/cmdk/internal/item"
	"github.com/jmcampanini/cmdk/internal/pathfmt"
)

func ListCWD(_ context.Context, home, shortenHome string, rules []pathfmt.Rule) ([]item.Item, error) {
	wd, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	it := item.NewItem()
	it.Type = "dir"
	it.Source = "cwd"
	it.Action = item.ActionNextList
	it.Display = pathfmt.DisplayPath(wd, home, shortenHome, rules)
	it.Data["path"] = wd
	return []item.Item{it}, nil
}
