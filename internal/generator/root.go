package generator

import (
	"context"
	"errors"
	"fmt"
	"runtime/debug"
	"sync"
	"time"

	log "charm.land/log/v2"
	"github.com/jmcampanini/cmdk/internal/cmdrun"
	"github.com/jmcampanini/cmdk/internal/item"
)

type Source struct {
	Name  string
	Limit int
	Async bool
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
						log.Error("source panicked", "source", src.Name, "panic", r, "stack", string(debug.Stack()))
						errs[i] = fmt.Errorf("panic: %v", r)
					}
				}()

				// timeout <= 0 skips the outer fetch deadline; every
				// external command a source runs still carries its own
				// per-command bound.
				ctx := context.Background()
				if timeout > 0 {
					var cancel context.CancelFunc
					ctx, cancel = context.WithTimeout(ctx, timeout)
					defer cancel()
				}

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
				logSourceError(src.Name, errs[i])
				all = append(all, ErrorItem(src, errs[i]))
				continue
			}
			all = append(all, results[i]...)
		}
		return all
	}
}

func ErrorItem(src Source, err error) item.Item {
	// Command failures render their one-line headline in the list; the full
	// bounded streams go to the log via logSourceError at the fetch sites.
	message := err.Error()
	var cmdErr *cmdrun.CommandError
	if errors.As(err, &cmdErr) {
		message = cmdErr.Headline()
	}

	errItem := item.NewItem()
	errItem.Type = "error"
	errItem.Source = src.Name
	errItem.Display = fmt.Sprintf("%s error: %s", src.Name, message)
	return errItem
}

// logSourceError records a source failure with its full bounded detail —
// CommandError.Error() carries the annotated captured streams — so the
// headline-only list row never becomes the only trace of the cause.
func logSourceError(source string, err error) {
	log.Error("source failed", "source", source, "error", err)
}

func LoadingItem(src Source) item.Item {
	it := item.NewItem()
	it.Type = "loading"
	it.Source = src.Name
	it.Display = fmt.Sprintf("Loading %s\u2026", src.Name)
	return it
}
