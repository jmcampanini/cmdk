package tmux

import (
	"context"
	"errors"
	"maps"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/jmcampanini/cmdk/internal/item"
)

const (
	tmuxWindowSwitchCommand = "tmux switch-client -t {{sq .session_id}}:{{sq .window_id}}"
	defaultWindowActivity   = "0"
)

var tmuxListWindowsFormat = tmuxFormatFields(
	tmuxEscapedSessionNameFormat,
	tmuxEscapedFormat(cmdkSessionKeyOption),
	"#{session_id}",
	"#{window_index}",
	"#{window_id}",
	tmuxEscapedWindowNameFormat,
	"#{window_bell_flag}",
	"#{window_activity}",
)

const (
	windowLineSessionField = iota
	windowLineSessionKeyField
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
	rawSessionKey  string
	sessionID      string
	windowIndex    string
	windowID       string
	rawWindowName  string
}

func parseWindowLine(line string) (windowLine, bool) {
	fields, ok := splitRootWindowFields(line)
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
		rawSessionKey:  fields[windowLineSessionKeyField],
		sessionID:      sessionID,
		windowIndex:    windowIndex,
		windowID:       windowID,
		rawWindowName:  fields[windowLineWindowNameField],
	}, true
}

func splitRootWindowFields(line string) ([]string, bool) {
	fields := strings.Split(cleanTmuxLine(line), tmuxFieldSeparator)
	switch len(fields) {
	case windowLineFieldCount:
		return rootWindowFieldsWithDefaultActivity(fields), true
	case windowLineFieldCount - 1:
		if rootWindowFieldsHaveSessionKey(fields) {
			fields = append(fields, defaultWindowActivity)
			return fields, true
		}
		return rootWindowFieldsWithoutSessionKey(fields)
	case windowLineFieldCount - 2:
		return rootWindowFieldsWithoutSessionKey(append(fields, defaultWindowActivity))
	default:
		return nil, false
	}
}

func rootWindowFieldsHaveSessionKey(fields []string) bool {
	return len(fields) == windowLineFieldCount-1 &&
		validTmuxSessionID.MatchString(fields[windowLineSessionIDField]) &&
		validTmuxWindowID.MatchString(fields[windowLineWindowIDField])
}

func rootWindowFieldsWithDefaultActivity(fields []string) []string {
	if fields[windowLineActivityField] == "" {
		fields[windowLineActivityField] = defaultWindowActivity
	}
	return fields
}

func rootWindowFieldsWithoutSessionKey(fields []string) ([]string, bool) {
	if len(fields) != windowLineFieldCount-1 {
		return nil, false
	}
	sessionID := fields[1]
	windowID := fields[3]
	if !validTmuxSessionID.MatchString(sessionID) || !validTmuxWindowID.MatchString(windowID) {
		return nil, false
	}
	withSessionKey := make([]string, 0, windowLineFieldCount)
	withSessionKey = append(withSessionKey, fields[0], "")
	withSessionKey = append(withSessionKey, fields[1:]...)
	return rootWindowFieldsWithDefaultActivity(withSessionKey), true
}

func newWindowItem(parsed windowLine, display DisplayOptions) item.Item {
	sessionName := displaySafeTmuxSessionName(parsed.rawSessionName)
	sessionKey := displaySafeTmuxControls(parsed.rawSessionKey)
	windowName := displaySafeTmuxWindowName(parsed.rawWindowName)
	sessionDisplay := display.formatSessionDisplay(sessionName, sessionKey)

	it := item.NewItem()
	it.Type = "window"
	it.Source = "tmux"
	it.Display = tmuxWindowDisplay(windowName, sessionDisplay)
	it.Action = item.ActionExecute
	it.Cmd = tmuxWindowSwitchCommand
	it.Data["session_name"] = sessionName
	if sessionKey != "" {
		it.Data["session_key"] = sessionKey
	}
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
	sortSessionValue string
	sortKey          windowSortKey
	item             item.Item
}

func parseWindowEntries(output string, display DisplayOptions) ([]windowEntry, int) {
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
			sortSessionValue: sessionDisplayValue(parsed.rawSessionName, parsed.rawSessionKey),
			sortKey:          parsed.sortKey,
			item:             newWindowItem(parsed, display),
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
		if left.sortSessionValue != right.sortSessionValue {
			return left.sortSessionValue < right.sortSessionValue
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
	return ParseWindowsWithDisplay(output, DisplayOptions{})
}

func ParseWindowsWithDisplay(output string, display DisplayOptions) ([]item.Item, error) {
	entries, malformedRows := parseWindowEntries(output, display)
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
	return ListWindowsWithDisplay(ctx, DisplayOptions{})
}

func ListWindowsWithDisplay(ctx context.Context, display DisplayOptions) ([]item.Item, error) {
	out, err := tmuxOutput(ctx, "list-windows", "-a", "-F", tmuxListWindowsFormat)
	if err != nil {
		return nil, err
	}
	return ParseWindowsWithDisplay(string(out), display)
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
	return ParseWindowsForSessionWithDisplay(output, session, DisplayOptions{})
}

func ParseWindowsForSessionWithDisplay(output string, session item.Item, display DisplayOptions) ([]item.Item, error) {
	entries, malformedRows := parseSessionWindowEntries(output, session, display)
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

func parseSessionWindowEntries(output string, session item.Item, display DisplayOptions) ([]sessionWindowEntry, int) {
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
			item:    newSessionWindowItem(session, parsed, display),
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

func newSessionWindowItem(session item.Item, parsed sessionWindowLine, display DisplayOptions) item.Item {
	windowName := displaySafeTmuxWindowName(parsed.rawWindowName)
	sessionName := session.Data["session_name"]
	sessionKey := session.Data["session_key"]
	sessionDisplay := display.formatSessionDisplay(sessionName, sessionKey)

	it := item.NewItem()
	it.Type = "window"
	it.Source = "tmux"
	it.Display = tmuxWindowDisplay(windowName, sessionDisplay)
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

func tmuxWindowDisplay(windowName, sessionDisplay string) string {
	display := "tmux win"
	if windowName != "" {
		display += " " + windowName
	}
	if sessionDisplay != "" {
		display += " ‹ " + sessionDisplay
	}
	return display
}

func ListWindowsForSession(ctx context.Context, session item.Item) ([]item.Item, error) {
	return ListWindowsForSessionWithDisplay(ctx, session, DisplayOptions{})
}

func ListWindowsForSessionWithDisplay(ctx context.Context, session item.Item, display DisplayOptions) ([]item.Item, error) {
	target, err := sessionTarget(session)
	if err != nil {
		return nil, err
	}

	out, err := tmuxOutput(ctx, "list-windows", "-t", target, "-F", windowsForSessionFormat)
	if err != nil {
		return nil, err
	}
	return ParseWindowsForSessionWithDisplay(string(out), session, display)
}

func sessionTarget(session item.Item) (string, error) {
	if sessionID := session.Data["session_id"]; sessionID != "" {
		return sessionID, nil
	}
	return "", errors.New("session item is missing session_id")
}
