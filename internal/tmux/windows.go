package tmux

import (
	"context"
	"fmt"
	"maps"
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

		displayPart, bellFlag, _ := strings.Cut(line, "\t")

		session, rest, ok := strings.Cut(displayPart, ":")
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

		bell := bellFlag == "1"

		it := item.NewItem()
		it.Type = "window"
		it.Source = "tmux"
		it.Display = "tmux: " + displayPart
		it.Action = item.ActionExecute
		it.Cmd = "tmux switch-client -t '{{.session}}:{{.window_index}}'"
		it.Data["session"] = session
		it.Data["window_index"] = windowIndex
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
	out, err := exec.CommandContext(ctx, "tmux", "list-windows", "-a", "-F", "#{session_name}:#{window_index} #{window_name}\t#{window_bell_flag}").Output()
	if err != nil {
		if ctx.Err() != nil {
			return nil, fmt.Errorf("tmux did not respond within the configured timeout: %w", err)
		}
		return nil, err
	}
	return ParseWindows(string(out)), nil
}

func ParseWindowsForSession(output string, session item.Item) []item.Item {
	lines := strings.Split(output, "\n")

	type entry struct {
		index int
		item  item.Item
	}

	var entries []entry
	for _, line := range lines {
		line = strings.TrimRight(line, "\r")
		if line == "" {
			continue
		}

		parts := strings.SplitN(line, "\t", 3)
		if len(parts) != 3 {
			continue
		}

		windowIndex := parts[0]
		windowName := parts[1]
		idx, err := strconv.Atoi(windowIndex)
		if err != nil {
			continue
		}

		it := item.NewItem()
		it.Type = "window"
		it.Source = "tmux"
		it.Display = "window " + windowIndex
		if windowName != "" {
			it.Display += " " + windowName
		}
		it.Action = item.ActionExecute
		it.Cmd = sessionWindowCmd(session)
		maps.Copy(it.Data, session.Data)
		if sessionName := session.Data["session_name"]; sessionName != "" {
			it.Data["session"] = sessionName
		}
		it.Data["window_index"] = windowIndex

		entries = append(entries, entry{index: idx, item: it})
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].index < entries[j].index
	})

	items := make([]item.Item, len(entries))
	for i, e := range entries {
		items[i] = e.item
	}
	return items
}

func ListWindowsForSession(ctx context.Context, session item.Item) ([]item.Item, error) {
	target, err := sessionTarget(session)
	if err != nil {
		return nil, err
	}

	out, err := exec.CommandContext(ctx, "tmux", "list-windows", "-t", target, "-F", "#{window_index}\t#{window_name}\t#{window_bell_flag}").Output()
	if err != nil {
		if ctx.Err() != nil {
			return nil, fmt.Errorf("tmux did not respond within the configured timeout: %w", err)
		}
		return nil, err
	}
	return ParseWindowsForSession(string(out), session), nil
}

func sessionTarget(session item.Item) (string, error) {
	if sessionID := session.Data["session_id"]; sessionID != "" {
		return sessionID, nil
	}
	if sessionName := session.Data["session_name"]; sessionName != "" {
		return "=" + sessionName, nil
	}
	return "", fmt.Errorf("session item is missing session_id and session_name")
}

func sessionWindowCmd(session item.Item) string {
	if session.Data["session_id"] != "" {
		return `tmux switch-client -t {{sq (printf "%s:%s" .session_id .window_index)}}`
	}
	return `tmux switch-client -t {{sq (printf "=%s:%s" .session_name .window_index)}}`
}
