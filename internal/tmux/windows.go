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

func ParseWindows(output string) []item.Item {
	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) == 1 && lines[0] == "" {
		return nil
	}

	type entry struct {
		session string
		index   int
		item    item.Item
	}

	var entries []entry
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		session, rest, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}

		windowIndex, _, ok := strings.Cut(rest, " ")
		if !ok {
			continue
		}

		idx, err := strconv.Atoi(windowIndex)
		if err != nil {
			continue
		}

		it := item.NewItem()
		it.Type = "window"
		it.Source = "tmux"
		it.Display = "tmux: " + line
		it.Action = item.ActionExecute
		it.Cmd = "tmux switch-client -t '{{.session}}:{{.window_index}}'"
		it.Data["session"] = session
		it.Data["window_index"] = windowIndex

		entries = append(entries, entry{session: session, index: idx, item: it})
	}

	sort.Slice(entries, func(i, j int) bool {
		if entries[i].session != entries[j].session {
			return entries[i].session < entries[j].session
		}
		return entries[i].index < entries[j].index
	})

	items := make([]item.Item, len(entries))
	for i, e := range entries {
		items[i] = e.item
	}
	return items
}

func ListWindows(ctx context.Context) ([]item.Item, error) {
	out, err := exec.CommandContext(ctx, "tmux", "list-windows", "-a", "-F", "#{session_name}:#{window_index} #{window_name}").Output()
	if err != nil {
		if ctx.Err() != nil {
			return nil, fmt.Errorf("tmux did not respond within the configured timeout: %w", err)
		}
		return nil, err
	}
	return ParseWindows(string(out)), nil
}
