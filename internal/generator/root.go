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
	return func(accumulated []item.Item, ctx Context) []item.Item {
		results := make([][]item.Item, len(sources))
		errs := make([]error, len(sources))
		var wg sync.WaitGroup

		for i, src := range sources {
			if src.Fetch == nil {
				errs[i] = fmt.Errorf("no fetch function")
				continue
			}

			wg.Add(1)
			go func(i int, src Source) {
				defer wg.Done()
				defer func() {
					if r := recover(); r != nil {
						errs[i] = fmt.Errorf("panic: %v", r)
					}
				}()

				fetchCtx, cancel := newFetchContext(timeout)
				defer cancel()

				results[i], errs[i] = src.Fetch(fetchCtx)
			}(i, src)
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

func newFetchContext(timeout time.Duration) (context.Context, context.CancelFunc) {
	if timeout <= 0 {
		timeout = 2 * time.Second
	}
	return context.WithTimeout(context.Background(), timeout)
}

func errorItem(src Source, err error) item.Item {
	errItem := item.NewItem()
	errItem.Type = src.Type
	errItem.Source = src.Name
	errItem.Display = fmt.Sprintf("%s error: %s", src.Name, err)
	return errItem
}
