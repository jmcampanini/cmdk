package tui

import (
	"context"
	"fmt"
	"runtime/debug"
	"time"

	tea "charm.land/bubbletea/v2"
	log "charm.land/log/v2"

	"github.com/jmcampanini/cmdk/internal/item"
)

type AsyncSource struct {
	Name    string
	Type    string
	Limit   int
	Timeout time.Duration
	Fetch   func(context.Context) ([]item.Item, error)
}

type sourceResultMsg struct {
	Name  string
	Items []item.Item
	Err   error
}

func fetchSourceCmd(src AsyncSource) tea.Cmd {
	return func() tea.Msg {
		if src.Fetch == nil {
			return sourceResultMsg{Name: src.Name, Err: fmt.Errorf("no fetch function")}
		}

		timeout := src.Timeout
		if timeout <= 0 {
			timeout = 2 * time.Second
		}
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()

		items, err := func() (result []item.Item, retErr error) {
			defer func() {
				if r := recover(); r != nil {
					log.Error("async source panicked", "source", src.Name, "panic", r, "stack", string(debug.Stack()))
					retErr = fmt.Errorf("panic: %v", r)
				}
			}()
			return src.Fetch(ctx)
		}()

		if err != nil {
			return sourceResultMsg{Name: src.Name, Err: err}
		}
		if src.Limit > 0 && len(items) > src.Limit {
			items = items[:src.Limit]
		}
		return sourceResultMsg{Name: src.Name, Items: items}
	}
}
