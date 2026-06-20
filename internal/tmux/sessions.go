package tmux

import (
	"context"
	"fmt"
	"os/exec"
	"sort"
	"strconv"
	"strings"

	"github.com/jmcampanini/cmdk/internal/item"
)

const SessionKindExternal = "external"

func ParseSessions(output string) []item.Item {
	lines := strings.Split(output, "\n")

	type entry struct {
		name string
		item item.Item
	}

	var entries []entry
	for _, line := range lines {
		line = strings.TrimRight(line, "\r")
		if line == "" {
			continue
		}

		parts := strings.Split(line, "\t")
		if len(parts) != 4 {
			continue
		}

		sessionID := parts[0]
		sessionName := parts[1]
		sessionWindows := parts[2]
		sessionAttached := parts[3]
		if sessionID == "" || sessionName == "" {
			continue
		}
		if _, err := strconv.Atoi(sessionWindows); err != nil {
			continue
		}
		if _, err := strconv.Atoi(sessionAttached); err != nil {
			continue
		}

		display := "tmux: " + sessionName
		it := item.NewItem()
		it.Type = "session"
		it.Source = "tmux"
		it.Display = display
		it.Action = item.ActionNextList
		it.Data["session_attached"] = sessionAttached
		it.Data["session_display"] = display
		it.Data["session_id"] = sessionID
		it.Data["session_kind"] = SessionKindExternal
		it.Data["session_name"] = sessionName
		it.Data["session_windows"] = sessionWindows

		entries = append(entries, entry{name: sessionName, item: it})
	}

	sort.SliceStable(entries, func(i, j int) bool {
		return entries[i].name < entries[j].name
	})

	items := make([]item.Item, len(entries))
	for i, e := range entries {
		items[i] = e.item
	}
	return items
}

func ListSessions(ctx context.Context) ([]item.Item, error) {
	out, err := exec.CommandContext(ctx, "tmux", "list-sessions", "-F", "#{session_id}\t#{session_name}\t#{session_windows}\t#{session_attached}").Output()
	if err != nil {
		if ctx.Err() != nil {
			return nil, fmt.Errorf("tmux did not respond within the configured timeout: %w", err)
		}
		return nil, err
	}
	return ParseSessions(string(out)), nil
}
