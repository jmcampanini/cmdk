package generator

import (
	"context"
	"errors"
	"maps"
	"time"

	"github.com/jmcampanini/cmdk/internal/item"
)

// SessionWindowsFunc lists windows belonging to a selected session. It is injected
// so NewSessionGenerator stays unit-testable without shelling out to tmux.
type SessionWindowsFunc func(context.Context, item.Item) ([]item.Item, error)

const (
	sessionConnectCommand      = `tmux switch-client -t {{sq .session_id}}`
	defaultSessionFetchTimeout = 2 * time.Second
)

// NewSessionGenerator builds the child list shown after selecting a tmux
// session: built-in Connect first, then user-defined session actions, then
// windows in that session. Connect and configured actions are type "action"
// while windows are type "window", so GroupAndOrder preserves the required
// Connect/actions-before-windows display order.
func NewSessionGenerator(fetchWindows SessionWindowsFunc) GeneratorFunc {
	actions := NewActionsGenerator()

	return func(accumulated []item.Item, ctx Context) []item.Item {
		session, ok := selectedSession(accumulated)
		if !ok {
			return nil
		}
		if session.Data["session_id"] == "" {
			return []item.Item{ErrorItem(Source{Name: "session"}, errors.New("missing session_id"))}
		}

		items := []item.Item{sessionConnectItem(session, ctx.PaneID)}
		items = append(items, actions(accumulated, ctx)...)
		items = append(items, fetchSessionWindows(session, ctx, fetchWindows)...)
		return items
	}
}

func selectedSession(accumulated []item.Item) (item.Item, bool) {
	if len(accumulated) == 0 {
		return item.Item{}, false
	}
	session := accumulated[len(accumulated)-1]
	return session, session.Type == "session"
}

func sessionConnectItem(session item.Item, paneID string) item.Item {
	return item.Item{
		Type:    "action",
		Source:  "builtin",
		Display: "Connect",
		Action:  item.ActionExecute,
		Cmd:     sessionConnectCommand,
		Data:    sessionData(session, paneID),
	}
}

func sessionData(session item.Item, paneID string) map[string]string {
	data := make(map[string]string, len(session.Data)+1)
	maps.Copy(data, session.Data)
	if paneID != "" {
		data["pane_id"] = paneID
	}
	return data
}

func fetchSessionWindows(session item.Item, ctx Context, fetchWindows SessionWindowsFunc) []item.Item {
	if fetchWindows == nil {
		return []item.Item{ErrorItem(Source{Name: "windows"}, errors.New("no fetch function"))}
	}

	timeout := ctx.Config.Timeout.Fetch
	if timeout <= 0 {
		timeout = defaultSessionFetchTimeout
	}
	fetchCtx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	windows, err := fetchWindows(fetchCtx, session)
	if err != nil {
		return []item.Item{ErrorItem(Source{Name: "windows"}, err)}
	}
	return windows
}
