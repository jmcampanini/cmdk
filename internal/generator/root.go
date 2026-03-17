package generator

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/jmcampanini/cmdk/internal/item"
)

type Source struct {
	Name  string
	Type  string
	Fetch func(context.Context) ([]item.Item, error)
}

func NewRootGenerator(timeout time.Duration, sources ...Source) GeneratorFunc {
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
						errs[i] = fmt.Errorf("panic: %v", r)
					}
				}()

				ctx, cancel := context.WithTimeout(context.Background(), timeout)
				defer cancel()

				results[i], errs[i] = src.Fetch(ctx)
			})
		}

		wg.Wait()

		var all []item.Item
		for i, src := range sources {
			if errs[i] != nil {
				all = append(all, errorItem(src, errs[i]))
				continue
			}
			all = append(all, results[i]...)
		}
		return all
	}
}

func errorItem(src Source, err error) item.Item {
	errItem := item.NewItem()
	errItem.Type = src.Type
	errItem.Source = src.Name
	errItem.Display = fmt.Sprintf("%s error: %s", src.Name, err)
	return errItem
}
