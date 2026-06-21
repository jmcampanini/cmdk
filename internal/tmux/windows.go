package tmux

import (
	"context"
	"fmt"
	"maps"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/jmcampanini/cmdk/internal/item"
)

const tmuxWindowSwitchCommand = "tmux switch-client -t {{sq .session_id}}:{{sq .window_id}}"

var tmuxListWindowsFormat = tmuxFormatFields(
	tmuxEscapedSessionNameFormat,
	"#{session_id}",
	"#{window_index}",
	"#{window_id}",
	tmuxEscapedWindowNameFormat,
	"#{window_bell_flag}",
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
	fields, ok := splitTmuxFields(line, windowLineFieldCount)
	if !ok {
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
	sessionName := displaySafeTmuxSessionName(parsed.session)
	windowName := displaySafeTmuxWindowName(parsed.windowName)

	it := item.NewItem()
	it.Type = "window"
	it.Source = "tmux"
	it.Display = fmt.Sprintf("tmux: %s:%s %s", sessionName, parsed.windowIndex, windowName)
	it.Action = item.ActionExecute
	it.Cmd = tmuxWindowSwitchCommand
	it.Data["session_name"] = sessionName
	it.Data["session_id"] = parsed.sessionID
	it.Data["window_index"] = parsed.windowIndex
	it.Data["window_id"] = parsed.windowID
	if bell {
		it.Data["bell"] = "1"
	}
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
		line = cleanTmuxLine(line)
		if line == "" {
			continue
		}

		parsed, ok := parseWindowLine(line)
		if !ok {
			malformedRows++
			continue
		}

		windowIndex, err := strconv.Atoi(parsed.windowIndex)
		if err != nil {
			malformedRows++
			continue
		}

		bell := parsed.bellFlag == "1"
		entries = append(entries, windowEntry{
			session:     parsed.session,
			windowIndex: windowIndex,
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
		items = append(items, newTmuxParseErrorItem("list-windows", malformedRows))
	}
	return items, nil
}

func ListWindows(ctx context.Context) ([]item.Item, error) {
	out, err := tmuxOutput(ctx, "list-windows", "-a", "-F", tmuxListWindowsFormat)
	if err != nil {
		return nil, err
	}
	return ParseWindows(string(out))
}

const (
	sessionWindowLineIndexField = iota
	sessionWindowLineIDField
	sessionWindowLineNameField
	sessionWindowLineBellFlagField
	sessionWindowLineFieldCount
)

// Keep window_bell_flag as a sentinel field so empty window names preserve
// the expected tab count. TODO: surface this flag in the TUI for session child
// windows without setting Data["bell"] and reordering them above Connect.
var windowsForSessionFormat = tmuxFormatFields(
	"#{window_index}",
	"#{window_id}",
	tmuxEscapedWindowNameFormat,
	"#{window_bell_flag}",
)

func ParseWindowsForSession(output string, session item.Item) ([]item.Item, error) {
	lines := strings.Split(output, "\n")
	entries := make([]sessionWindowEntry, 0, len(lines))
	malformedRows := 0

	for _, line := range lines {
		line = cleanTmuxLine(line)
		if line == "" {
			continue
		}

		parsed, ok := parseSessionWindowLine(line)
		if !ok {
			malformedRows++
			continue
		}

		entries = append(entries, sessionWindowEntry{
			index: parsed.sortIndex,
			item:  newSessionWindowItem(session, parsed.windowIndex, parsed.windowID, parsed.windowName),
		})
	}

	if len(entries) == 0 {
		if malformedRows > 0 {
			return nil, fmt.Errorf("could not parse any tmux list-windows rows (%d unparseable)", malformedRows)
		}
		return nil, nil
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].index < entries[j].index
	})

	items := make([]item.Item, len(entries))
	for i, e := range entries {
		items[i] = e.item
	}
	if malformedRows > 0 {
		items = append(items, newTmuxParseErrorItem("list-windows", malformedRows))
	}
	return items, nil
}

type sessionWindowEntry struct {
	index int
	item  item.Item
}

type sessionWindowLine struct {
	sortIndex   int
	windowIndex string
	windowID    string
	windowName  string
}

func parseSessionWindowLine(line string) (sessionWindowLine, bool) {
	fields, ok := splitTmuxFields(line, sessionWindowLineFieldCount)
	if !ok {
		return sessionWindowLine{}, false
	}

	windowIndex := fields[sessionWindowLineIndexField]
	windowID := fields[sessionWindowLineIDField]
	idx, err := strconv.Atoi(windowIndex)
	if err != nil || !validTmuxWindowID.MatchString(windowID) {
		return sessionWindowLine{}, false
	}

	return sessionWindowLine{
		sortIndex:   idx,
		windowIndex: windowIndex,
		windowID:    windowID,
		windowName:  fields[sessionWindowLineNameField],
	}, true
}

func newSessionWindowItem(session item.Item, windowIndex, windowID, windowName string) item.Item {
	windowName = displaySafeTmuxWindowName(windowName)

	it := item.NewItem()
	it.Type = "window"
	it.Source = "tmux"
	it.Display = "window " + windowIndex
	if windowName != "" {
		it.Display += " " + windowName
	}
	it.Action = item.ActionExecute
	it.Cmd = tmuxWindowSwitchCommand
	maps.Copy(it.Data, session.Data)
	it.Data["window_index"] = windowIndex
	it.Data["window_id"] = windowID
	return it
}

func ListWindowsForSession(ctx context.Context, session item.Item) ([]item.Item, error) {
	target, err := sessionTarget(session)
	if err != nil {
		return nil, err
	}

	out, err := tmuxOutput(ctx, "list-windows", "-t", target, "-F", windowsForSessionFormat)
	if err != nil {
		return nil, err
	}
	return ParseWindowsForSession(string(out), session)
}

func sessionTarget(session item.Item) (string, error) {
	if sessionID := session.Data["session_id"]; sessionID != "" {
		return sessionID, nil
	}
	return "", fmt.Errorf("session item is missing session_id")
}
