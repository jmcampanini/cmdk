package generator

import (
	"context"
	"fmt"
	"maps"
	"time"

	"github.com/jmcampanini/cmdk/internal/item"
)

type SessionWindowsFunc func(context.Context, item.Item) ([]item.Item, error)

func NewSessionGenerator(fetchWindows SessionWindowsFunc) GeneratorFunc {
	actions := NewActionsGenerator()

	return func(accumulated []item.Item, ctx Context) []item.Item {
		if len(accumulated) == 0 {
			return nil
		}
		session := accumulated[len(accumulated)-1]
		if session.Type != "session" {
			return nil
		}

		data := maps.Clone(session.Data)
		if data == nil {
			data = make(map[string]string)
		}
		if ctx.PaneID != "" {
			data["pane_id"] = ctx.PaneID
		}

		items := []item.Item{
			{
				Type:    "action",
				Source:  "builtin",
				Display: "Connect",
				Action:  item.ActionExecute,
				Cmd:     sessionConnectCmd(session),
				Data:    data,
			},
		}

		items = append(items, actions(accumulated, ctx)...)
		items = append(items, fetchSessionWindows(session, ctx, fetchWindows)...)
		return items
	}
}

func fetchSessionWindows(session item.Item, ctx Context, fetchWindows SessionWindowsFunc) []item.Item {
	if fetchWindows == nil {
		return []item.Item{ErrorItem(Source{Name: "windows", Type: "window"}, fmt.Errorf("no fetch function"))}
	}

	timeout := ctx.Config.Timeout.Fetch
	if timeout <= 0 {
		timeout = 2 * time.Second
	}
	fetchCtx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	windows, err := fetchWindows(fetchCtx, session)
	if err != nil {
		return []item.Item{ErrorItem(Source{Name: "windows", Type: "window"}, err)}
	}
	return windows
}

func sessionConnectCmd(session item.Item) string {
	if session.Data["session_id"] != "" {
		return `tmux switch-client -t {{sq .session_id}}`
	}
	return `tmux switch-client -t {{sq (printf "=%s" .session_name)}}`
}
