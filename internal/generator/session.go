package generator

import (
	"context"
	"fmt"
	"maps"
	"time"

	"github.com/jmcampanini/cmdk/internal/item"
)

// SessionWindowsFunc lists windows belonging to a selected session. It is injected
// so NewSessionGenerator stays unit-testable without shelling out to tmux.
type SessionWindowsFunc func(context.Context, item.Item) ([]item.Item, error)

const sessionConnectCommand = `tmux switch-client -t {{sq .session_id}}`

// NewSessionGenerator builds the child list shown after selecting a tmux
// session: built-in Connect first, then user-defined session actions, then
// windows in that session. Connect and configured actions are type "action"
// while windows are type "window", so GroupAndOrder preserves the required
// Connect/actions-before-windows display order.
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
		if session.Data["session_id"] == "" {
			return []item.Item{ErrorItem(Source{Name: "session", Type: "action"}, fmt.Errorf("missing session_id"))}
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
				Cmd:     sessionConnectCommand,
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
