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
		bell    bool
		item    item.Item
	}

	var entries []entry
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		session, rest, ok := strings.Cut(line, "\t")
		if !ok {
			continue
		}
		sessionID, rest, ok := strings.Cut(rest, "\t")
		if !ok {
			continue
		}
		windowIndex, rest, ok := strings.Cut(rest, "\t")
		if !ok {
			continue
		}
		windowID, rest, ok := strings.Cut(rest, "\t")
		if !ok {
			continue
		}
		bellSep := strings.LastIndex(rest, "\t")
		if bellSep < 0 {
			continue
		}
		windowName := rest[:bellSep]
		bellFlag := rest[bellSep+1:]

		idx, err := strconv.Atoi(windowIndex)
		if err != nil {
			continue
		}

		bell := bellFlag == "1"

		it := item.NewItem()
		it.Type = "window"
		it.Source = "tmux"
		it.Display = fmt.Sprintf("tmux: %s:%s %s", session, windowIndex, windowName)
		it.Action = item.ActionExecute
		it.Cmd = "tmux switch-client -t '{{.session_id}}:{{.window_id}}'"
		it.Data["session"] = session
		it.Data["session_id"] = sessionID
		it.Data["window_index"] = windowIndex
		it.Data["window_id"] = windowID
		if bell {
			it.Data["bell"] = "1"
		}

		entries = append(entries, entry{session: session, index: idx, bell: bell, item: it})
	}

	// Bell windows sort first: true sorts before false.
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].bell != entries[j].bell {
			return entries[i].bell
		}
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
	out, err := exec.CommandContext(ctx, "tmux", "list-windows", "-a", "-F", "#{session_name}\t#{session_id}\t#{window_index}\t#{window_id}\t#{window_name}\t#{window_bell_flag}").Output()
	if err != nil {
		if ctx.Err() != nil {
			return nil, fmt.Errorf("tmux did not respond within the configured timeout: %w", err)
		}
		return nil, err
	}
	return ParseWindows(string(out)), nil
}
