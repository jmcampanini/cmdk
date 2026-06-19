package tmux

import (
	"context"
	"fmt"
	"os/exec"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/jmcampanini/cmdk/internal/item"
)

const (
	tmuxEscapedNewline = "↵"
	tmuxEscapedTab     = "⇥"

	tmuxEscapedSessionNameFormat = "#{s|\t|" + tmuxEscapedTab + "|:#{s|\n|" + tmuxEscapedNewline + "|:#{session_name}}}"
	tmuxEscapedWindowNameFormat  = "#{s|\t|" + tmuxEscapedTab + "|:#{s|\n|" + tmuxEscapedNewline + "|:#{window_name}}}"
	tmuxWindowSwitchCommand      = "tmux switch-client -t {{sq .session_id}}:{{sq .window_id}}"
	tmuxListWindowsFormat        = tmuxEscapedSessionNameFormat + "\t#{session_id}\t#{window_index}\t#{window_id}\t" + tmuxEscapedWindowNameFormat + "\t#{window_bell_flag}"
)

const (
	windowLineSessionField = iota
	windowLineSessionIDField
	windowLineWindowIndexField
	windowLineWindowIDField
	windowLineWindowNameField
	windowLineBellFlagField
	windowLineFieldCount
)

var (
	validTmuxSessionID = regexp.MustCompile(`^\$\d+$`)
	validTmuxWindowID  = regexp.MustCompile(`^@\d+$`)
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
	if len(fields) != windowLineFieldCount {
		return windowLine{}, false
	}

	sessionID := fields[windowLineSessionIDField]
	windowID := fields[windowLineWindowIDField]
	if !validTmuxSessionID.MatchString(sessionID) || !validTmuxWindowID.MatchString(windowID) {
		return windowLine{}, false
	}

	return windowLine{
		session:     fields[windowLineSessionField],
		sessionID:   sessionID,
		windowIndex: fields[windowLineWindowIndexField],
		windowID:    windowID,
		windowName:  fields[windowLineWindowNameField],
		bellFlag:    fields[windowLineBellFlagField],
	}, true
}

func newWindowItem(parsed windowLine, bell bool) item.Item {
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
	return it
}

func newWindowParseErrorItem(malformedRows int) item.Item {
	rowWord := "row"
	if malformedRows != 1 {
		rowWord = "rows"
	}

	it := item.NewItem()
	it.Type = "window"
	it.Source = "tmux"
	it.Display = fmt.Sprintf("tmux parse error: %d unparseable list-windows %s", malformedRows, rowWord)
	return it
}

type windowEntry struct {
	session     string
	windowIndex int
	bell        bool
	item        item.Item
}

func parseWindowEntries(output string) ([]windowEntry, int) {
	lines := strings.Split(output, "\n")
	entries := make([]windowEntry, 0, len(lines))
	malformedRows := 0

	for _, line := range lines {
		if line == "" {
			continue
		}

		parsed, ok := parseWindowLine(line)
		if !ok {
			malformedRows++
			continue
		}

		idx, err := strconv.Atoi(parsed.windowIndex)
		if err != nil {
			malformedRows++
			continue
		}

		bell := parsed.bellFlag == "1"
		entries = append(entries, windowEntry{
			session:     parsed.session,
			windowIndex: idx,
			bell:        bell,
			item:        newWindowItem(parsed, bell),
		})
	}

	return entries, malformedRows
}

func sortWindowEntries(entries []windowEntry) {
	// Bell windows sort first: true sorts before false.
	sort.Slice(entries, func(i, j int) bool {
		left, right := entries[i], entries[j]
		if left.bell != right.bell {
			return left.bell
		}
		if left.session != right.session {
			return left.session < right.session
		}
		return left.windowIndex < right.windowIndex
	})
}

func itemsFromWindowEntries(entries []windowEntry) []item.Item {
	items := make([]item.Item, len(entries))
	for i, e := range entries {
		items[i] = e.item
	}
	return items
}

func ParseWindows(output string) ([]item.Item, error) {
	entries, malformedRows := parseWindowEntries(output)
	if len(entries) == 0 {
		if malformedRows > 0 {
			return nil, fmt.Errorf("could not parse any tmux list-windows rows (%d unparseable)", malformedRows)
		}
		return nil, nil
	}

	sortWindowEntries(entries)
	items := itemsFromWindowEntries(entries)
	if malformedRows > 0 {
		items = append(items, newWindowParseErrorItem(malformedRows))
	}
	return items, nil
}

func ListWindows(ctx context.Context) ([]item.Item, error) {
	out, err := exec.CommandContext(ctx, "tmux", "list-windows", "-a", "-F", tmuxListWindowsFormat).Output()
	if err != nil {
		if ctx.Err() != nil {
			return nil, fmt.Errorf("tmux did not respond within the configured timeout: %w", err)
		}
		return nil, err
	}
	return ParseWindows(string(out))
}
