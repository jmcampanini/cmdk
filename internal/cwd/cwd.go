package cwd

import (
	"os"

	"github.com/jmcampanini/cmdk/internal/item"
)

func ListCWD() ([]item.Item, error) {
	wd, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	it := item.NewItem()
	it.Type = "dir"
	it.Source = "cwd"
	it.Action = item.ActionNextList
	it.Display = wd
	it.Data["path"] = wd
	return []item.Item{it}, nil
}
