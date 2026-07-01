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

const (
	tmuxWindowSwitchCommand = "tmux switch-client -t {{sq .session_id}}:{{sq .window_id}}"
	defaultWindowActivity   = "0"
)

var tmuxListWindowsFormat = tmuxFormatFields(
	tmuxEscapedSessionNameFormat,
	"#{session_id}",
	"#{window_index}",
	"#{window_id}",
	tmuxEscapedWindowNameFormat,
	"#{window_bell_flag}",
	"#{window_activity}",
)

const (
	windowLineSessionField = iota
	windowLineSessionIDField
	windowLineWindowIndexField
	windowLineWindowIDField
	windowLineWindowNameField
	windowLineBellFlagField
	windowLineActivityField
	windowLineFieldCount
)

var (
	validTmuxSessionID = regexp.MustCompile(`^\$\d+$`)
	validTmuxWindowID  = regexp.MustCompile(`^@\d+$`)
)

// splitWindowFields accepts current window rows and legacy rows that omit the
// trailing activity field.
func splitWindowFields(line string, expectedFields int) ([]string, bool) {
	fields, ok := splitTmuxFields(line, expectedFields)
	if ok {
		if fields[expectedFields-1] == "" {
			fields[expectedFields-1] = defaultWindowActivity
		}
		return fields, true
	}

	legacyFields, ok := splitTmuxFields(line, expectedFields-1)
	if !ok {
		return nil, false
	}
	return append(legacyFields, defaultWindowActivity), true
}

func parseTmuxBoolFlag(s string) (bool, bool) {
	switch s {
	case "0":
		return false, true
	case "1":
		return true, true
	default:
		return false, false
	}
}

type windowSortKey struct {
	bell     bool
	activity int64
	index    int
}

func parseWindowSortKey(windowIndex, bellFlag, activity string) (windowSortKey, bool) {
	index, err := strconv.Atoi(windowIndex)
	if err != nil {
		return windowSortKey{}, false
	}
	bell, ok := parseTmuxBoolFlag(bellFlag)
	if !ok {
		return windowSortKey{}, false
	}
	activityValue, err := strconv.ParseInt(activity, 10, 64)
	if err != nil {
		return windowSortKey{}, false
	}
	return windowSortKey{bell: bell, activity: activityValue, index: index}, true
}

type windowLine struct {
	sortKey        windowSortKey
	rawSessionName string
	sessionID      string
	windowIndex    string
	windowID       string
	rawWindowName  string
}

func parseWindowLine(line string) (windowLine, bool) {
	fields, ok := splitWindowFields(line, windowLineFieldCount)
	if !ok {
		return windowLine{}, false
	}

	sessionID := fields[windowLineSessionIDField]
	windowID := fields[windowLineWindowIDField]
	windowIndex := fields[windowLineWindowIndexField]
	sortKey, ok := parseWindowSortKey(
		windowIndex,
		fields[windowLineBellFlagField],
		fields[windowLineActivityField],
	)
	if !ok || !validTmuxSessionID.MatchString(sessionID) || !validTmuxWindowID.MatchString(windowID) {
		return windowLine{}, false
	}

	return windowLine{
		sortKey:        sortKey,
		rawSessionName: fields[windowLineSessionField],
		sessionID:      sessionID,
		windowIndex:    windowIndex,
		windowID:       windowID,
		rawWindowName:  fields[windowLineWindowNameField],
	}, true
}

func newWindowItem(parsed windowLine) item.Item {
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
	it.Data["window_name"] = windowName
	if parsed.sortKey.bell {
		it.Data["bell"] = "1"
	}
	return it
}

type windowEntry struct {
	sortSessionName string
	sortKey         windowSortKey
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

		entries = append(entries, windowEntry{
			sortSessionName: parsed.rawSessionName,
			sortKey:         parsed.sortKey,
			item:            newWindowItem(parsed),
		})
	}

	return entries, malformedRows
}

// lessWindowPriority reports whether left sorts before right when the bell or
// activity fields determine the order.
func lessWindowPriority(left, right windowSortKey) (less bool, decided bool) {
	if left.bell != right.bell {
		return left.bell, true
	}
	if left.activity != right.activity {
		return left.activity > right.activity, true
	}
	return false, false
}

func sortWindowEntries(entries []windowEntry) {
	// Bell windows sort first, then newest activity first. Session name and
	// window index provide stable, deterministic order when activity ties.
	sort.Slice(entries, func(i, j int) bool {
		left, right := entries[i], entries[j]
		if less, ok := lessWindowPriority(left.sortKey, right.sortKey); ok {
			return less
		}
		if left.sortSessionName != right.sortSessionName {
			return left.sortSessionName < right.sortSessionName
		}
		return left.sortKey.index < right.sortKey.index
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
	sessionWindowLineActivityField
	sessionWindowLineFieldCount
)

// Keep window_bell_flag as a sentinel field so empty window names preserve the
// expected tab count, and window_activity so child windows can use the same
// bell-first recency ordering as root windows.
var windowsForSessionFormat = tmuxFormatFields(
	"#{window_index}",
	"#{window_id}",
	tmuxEscapedWindowNameFormat,
	"#{window_bell_flag}",
	"#{window_activity}",
)

type sessionWindowEntry struct {
	sortKey windowSortKey
	item    item.Item
}

type sessionWindowLine struct {
	sortKey       windowSortKey
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
			sortKey: parsed.sortKey,
			item:    newSessionWindowItem(session, parsed),
		})
	}

	return entries, malformedRows
}

func sortSessionWindowEntries(entries []sessionWindowEntry) {
	sort.Slice(entries, func(i, j int) bool {
		left, right := entries[i], entries[j]
		if less, ok := lessWindowPriority(left.sortKey, right.sortKey); ok {
			return less
		}
		return left.sortKey.index < right.sortKey.index
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
	fields, ok := splitWindowFields(line, sessionWindowLineFieldCount)
	if !ok {
		return sessionWindowLine{}, false
	}

	windowIndex := fields[sessionWindowLineIndexField]
	windowID := fields[sessionWindowLineIDField]
	sortKey, ok := parseWindowSortKey(
		windowIndex,
		fields[sessionWindowLineBellFlagField],
		fields[sessionWindowLineActivityField],
	)
	if !ok || !validTmuxWindowID.MatchString(windowID) {
		return sessionWindowLine{}, false
	}

	return sessionWindowLine{
		sortKey:       sortKey,
		windowIndex:   windowIndex,
		windowID:      windowID,
		rawWindowName: fields[sessionWindowLineNameField],
	}, true
}

func newSessionWindowItem(session item.Item, parsed sessionWindowLine) item.Item {
	windowName := displaySafeTmuxWindowName(parsed.rawWindowName)

	it := item.NewItem()
	it.Type = "window"
	it.Source = "tmux"
	it.Display = tmuxWindowDisplay(parsed.windowIndex, windowName, session.Data["session_name"])
	it.Action = item.ActionExecute
	it.Cmd = tmuxWindowSwitchCommand
	maps.Copy(it.Data, session.Data)
	it.Data["window_index"] = parsed.windowIndex
	it.Data["window_id"] = parsed.windowID
	it.Data["window_name"] = windowName
	if parsed.sortKey.bell {
		it.Data["bell"] = "1"
	}
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
