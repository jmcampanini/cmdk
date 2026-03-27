package generator

import (
	"context"
	"fmt"
	"runtime/debug"
	"sync"
	"time"

	log "charm.land/log/v2"
	"github.com/jmcampanini/cmdk/internal/item"
)

type Source struct {
	Name  string
	Type  string
	Limit int
	Async bool
	Fetch func(context.Context) ([]item.Item, error)
}

func NewRootGenerator(timeout time.Duration, sources ...Source) GeneratorFunc {
	if timeout <= 0 {
		timeout = 2 * time.Second
	}
	return func(_ []item.Item, _ Context) []item.Item {
		results := make([][]item.Item, len(sources))
		errs := make([]error, len(sources))
		var wg sync.WaitGroup

		for i, src := range sources {
			if src.Fetch == nil {
				errs[i] = fmt.Errorf("no fetch function")
				continue
			}

			wg.Go(func() {
				defer func() {
					if r := recover(); r != nil {
						log.Error("source panicked", "source", src.Name, "panic", r, "stack", string(debug.Stack()))
						errs[i] = fmt.Errorf("panic: %v", r)
					}
				}()

				ctx, cancel := context.WithTimeout(context.Background(), timeout)
				defer cancel()

				results[i], errs[i] = src.Fetch(ctx)
				if src.Limit > 0 && len(results[i]) > src.Limit {
					results[i] = results[i][:src.Limit]
				}
			})
		}

		wg.Wait()

		var all []item.Item
		for i, src := range sources {
			if errs[i] != nil {
				all = append(all, ErrorItem(src, errs[i]))
				continue
			}
			all = append(all, results[i]...)
		}
		return all
	}
}

func ErrorItem(src Source, err error) item.Item {
	errItem := item.NewItem()
	errItem.Type = src.Type
	errItem.Source = src.Name
	errItem.Display = fmt.Sprintf("%s error: %s", src.Name, err)
	return errItem
}

func LoadingItem(src Source) item.Item {
	it := item.NewItem()
	it.Type = "loading"
	it.Source = src.Name
	it.Display = fmt.Sprintf("Loading %s\u2026", src.Name)
	it.Data["source_type"] = src.Type
	return it
}
