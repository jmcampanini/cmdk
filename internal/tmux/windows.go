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

const (
	tmuxWindowSwitchCommand = "tmux switch-client -t '{{.session_id}}:{{.window_id}}'"
	tmuxListWindowsFormat   = "#{session_name}\t#{session_id}\t#{window_index}\t#{window_id}\t#{window_name}\t#{window_bell_flag}"
)

type windowLine struct {
	session     string
	sessionID   string
	windowIndex string
	windowID    string
	windowName  string
	bellFlag    string
}

func parseWindowLine(line string) (windowLine, bool) {
	fields := strings.Split(line, "\t")
	if len(fields) < 6 {
		return windowLine{}, false
	}

	return windowLine{
		session:     fields[0],
		sessionID:   fields[1],
		windowIndex: fields[2],
		windowID:    fields[3],
		windowName:  strings.Join(fields[4:len(fields)-1], "\t"),
		bellFlag:    fields[len(fields)-1],
	}, true
}

func ParseWindows(output string) []item.Item {
	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) == 1 && lines[0] == "" {
		return nil
	}

	type entry struct {
		session     string
		windowIndex int
		bell        bool
		item        item.Item
	}

	entries := make([]entry, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		parsed, ok := parseWindowLine(line)
		if !ok {
			continue
		}

		idx, err := strconv.Atoi(parsed.windowIndex)
		if err != nil {
			continue
		}

		bell := parsed.bellFlag == "1"

		it := item.NewItem()
		it.Type = "window"
		it.Source = "tmux"
		it.Display = fmt.Sprintf("tmux: %s:%s %s", parsed.session, parsed.windowIndex, parsed.windowName)
		it.Action = item.ActionExecute
		it.Cmd = tmuxWindowSwitchCommand
		it.Data["session"] = parsed.session
		it.Data["session_id"] = parsed.sessionID
		it.Data["window_index"] = parsed.windowIndex
		it.Data["window_id"] = parsed.windowID
		if bell {
			it.Data["bell"] = "1"
		}

		entries = append(entries, entry{session: parsed.session, windowIndex: idx, bell: bell, item: it})
	}

	// Bell windows sort first: true sorts before false.
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].bell != entries[j].bell {
			return entries[i].bell
		}
		if entries[i].session != entries[j].session {
			return entries[i].session < entries[j].session
		}
		return entries[i].windowIndex < entries[j].windowIndex
	})

	items := make([]item.Item, len(entries))
	for i, e := range entries {
		items[i] = e.item
	}
	return items
}

func ListWindows(ctx context.Context) ([]item.Item, error) {
	out, err := exec.CommandContext(ctx, "tmux", "list-windows", "-a", "-F", tmuxListWindowsFormat).Output()
	if err != nil {
		if ctx.Err() != nil {
			return nil, fmt.Errorf("tmux did not respond within the configured timeout: %w", err)
		}
		return nil, err
	}
	return ParseWindows(string(out)), nil
}
