package tmux

import (
	"context"
	"errors"
	"maps"
	"regexp"
	"sort"
	"strconv"

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
	sortIndex      int
	rawSessionName string
	sessionID      string
	windowIndex    string
	windowID       string
	rawWindowName  string
	bellFlag       string
}

func parseWindowLine(line string) (windowLine, bool) {
	fields, ok := splitTmuxFields(line, windowLineFieldCount)
	if !ok {
		return windowLine{}, false
	}

	sessionID := fields[windowLineSessionIDField]
	windowID := fields[windowLineWindowIDField]
	windowIndex := fields[windowLineWindowIndexField]
	sortIndex, err := strconv.Atoi(windowIndex)
	if err != nil {
		return windowLine{}, false
	}
	if !validTmuxSessionID.MatchString(sessionID) || !validTmuxWindowID.MatchString(windowID) {
		return windowLine{}, false
	}

	return windowLine{
		sortIndex:      sortIndex,
		rawSessionName: fields[windowLineSessionField],
		sessionID:      sessionID,
		windowIndex:    windowIndex,
		windowID:       windowID,
		rawWindowName:  fields[windowLineWindowNameField],
		bellFlag:       fields[windowLineBellFlagField],
	}, true
}

func newWindowItem(parsed windowLine, bell bool) item.Item {
	sessionName := displaySafeTmuxSessionName(parsed.rawSessionName)
	windowName := displaySafeTmuxWindowName(parsed.rawWindowName)

	it := item.NewItem()
	it.Type = "window"
	it.Source = "tmux"
	it.Display = tmuxWindowDisplay(parsed.windowIndex, windowName, sessionName)
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
	sortSessionName string
	sortIndex       int
	bell            bool
	item            item.Item
}

func parseWindowEntries(output string) ([]windowEntry, int) {
	lines := tmuxLines(output)
	entries := make([]windowEntry, 0, len(lines))
	malformedRows := 0

	for _, line := range lines {
		parsed, ok := parseWindowLine(line)
		if !ok {
			malformedRows++
			continue
		}

		bell := parsed.bellFlag == "1"
		entries = append(entries, windowEntry{
			sortSessionName: parsed.rawSessionName,
			sortIndex:       parsed.sortIndex,
			bell:            bell,
			item:            newWindowItem(parsed, bell),
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
		if left.sortSessionName != right.sortSessionName {
			return left.sortSessionName < right.sortSessionName
		}
		return left.sortIndex < right.sortIndex
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
			return nil, newTmuxRowsParseError("list-windows", malformedRows)
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

type sessionWindowEntry struct {
	sortIndex int
	item      item.Item
}

type sessionWindowLine struct {
	sortIndex     int
	windowIndex   string
	windowID      string
	rawWindowName string
}

func ParseWindowsForSession(output string, session item.Item) ([]item.Item, error) {
	entries, malformedRows := parseSessionWindowEntries(output, session)
	if len(entries) == 0 {
		if malformedRows > 0 {
			return nil, newTmuxRowsParseError("list-windows", malformedRows)
		}
		return nil, nil
	}

	sortSessionWindowEntries(entries)
	items := itemsFromSessionWindowEntries(entries)
	if malformedRows > 0 {
		items = append(items, newTmuxParseErrorItem("list-windows", malformedRows))
	}
	return items, nil
}

func parseSessionWindowEntries(output string, session item.Item) ([]sessionWindowEntry, int) {
	lines := tmuxLines(output)
	entries := make([]sessionWindowEntry, 0, len(lines))
	malformedRows := 0

	for _, line := range lines {
		parsed, ok := parseSessionWindowLine(line)
		if !ok {
			malformedRows++
			continue
		}

		entries = append(entries, sessionWindowEntry{
			sortIndex: parsed.sortIndex,
			item:      newSessionWindowItem(session, parsed.windowIndex, parsed.windowID, parsed.rawWindowName),
		})
	}

	return entries, malformedRows
}

func sortSessionWindowEntries(entries []sessionWindowEntry) {
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].sortIndex < entries[j].sortIndex
	})
}

func itemsFromSessionWindowEntries(entries []sessionWindowEntry) []item.Item {
	items := make([]item.Item, len(entries))
	for i, e := range entries {
		items[i] = e.item
	}
	return items
}

func parseSessionWindowLine(line string) (sessionWindowLine, bool) {
	fields, ok := splitTmuxFields(line, sessionWindowLineFieldCount)
	if !ok {
		return sessionWindowLine{}, false
	}

	windowIndex := fields[sessionWindowLineIndexField]
	windowID := fields[sessionWindowLineIDField]
	sortIndex, err := strconv.Atoi(windowIndex)
	if err != nil {
		return sessionWindowLine{}, false
	}
	if !validTmuxWindowID.MatchString(windowID) {
		return sessionWindowLine{}, false
	}

	return sessionWindowLine{
		sortIndex:     sortIndex,
		windowIndex:   windowIndex,
		windowID:      windowID,
		rawWindowName: fields[sessionWindowLineNameField],
	}, true
}

func newSessionWindowItem(session item.Item, windowIndex, windowID, rawWindowName string) item.Item {
	windowName := displaySafeTmuxWindowName(rawWindowName)

	it := item.NewItem()
	it.Type = "window"
	it.Source = "tmux"
	it.Display = tmuxWindowDisplay(windowIndex, windowName, session.Data["session_name"])
	it.Action = item.ActionExecute
	it.Cmd = tmuxWindowSwitchCommand
	maps.Copy(it.Data, session.Data)
	it.Data["window_index"] = windowIndex
	it.Data["window_id"] = windowID
	return it
}

func tmuxWindowDisplay(windowIndex, windowName, sessionName string) string {
	display := "tmux:win: " + windowIndex
	if windowName != "" {
		display += " " + windowName
	}
	if sessionName != "" {
		display += " ‹ " + sessionName
	}
	return display
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
	return "", errors.New("session item is missing session_id")
}
