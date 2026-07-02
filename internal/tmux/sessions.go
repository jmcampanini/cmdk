package tmux

import (
	"context"
	"sort"
	"strconv"

	"github.com/jmcampanini/cmdk/internal/item"
)

const sessionKindExternal = "external"

const (
	sessionLineIDField = iota
	sessionLineNameField
	sessionLineWindowsField
	sessionLineAttachedField
	sessionLineKeyField
	sessionLineFieldCount
)

var sessionListFormat = tmuxFormatFields(
	"#{session_id}",
	tmuxEscapedSessionNameFormat,
	"#{session_windows}",
	"#{session_attached}",
	tmuxEscapedFormat(cmdkSessionKeyOption),
)

func ParseSessions(output string) ([]item.Item, error) {
	return ParseSessionsWithDisplay(output, DisplayOptions{})
}

func ParseSessionsWithDisplay(output string, display DisplayOptions) ([]item.Item, error) {
	lines := tmuxLines(output)
	entries := make([]sessionEntry, 0, len(lines))
	malformedRows := 0

	for _, line := range lines {
		entry, ok := parseSessionLine(line, display)
		if !ok {
			malformedRows++
			continue
		}
		entries = append(entries, entry)
	}

	if len(entries) == 0 {
		if malformedRows > 0 {
			return nil, newTmuxRowsParseError("list-sessions", malformedRows)
		}
		return nil, nil
	}

	sort.SliceStable(entries, func(i, j int) bool {
		return entries[i].sortValue < entries[j].sortValue
	})

	items := make([]item.Item, len(entries))
	for i, e := range entries {
		items[i] = e.item
	}
	if malformedRows > 0 {
		items = append(items, newTmuxParseErrorItem("list-sessions", malformedRows))
	}
	return items, nil
}

type sessionEntry struct {
	sortValue string
	item      item.Item
}

func parseSessionLine(line string, display DisplayOptions) (sessionEntry, bool) {
	fields, ok := splitSessionFields(line)
	if !ok {
		return sessionEntry{}, false
	}

	sessionID := fields[sessionLineIDField]
	sessionName := displaySafeTmuxSessionName(fields[sessionLineNameField])
	sessionKey := displaySafeTmuxControls(fields[sessionLineKeyField])
	sessionWindows := fields[sessionLineWindowsField]
	sessionAttached := fields[sessionLineAttachedField]
	if !validTmuxSessionID.MatchString(sessionID) || sessionName == "" {
		return sessionEntry{}, false
	}
	if _, err := strconv.Atoi(sessionWindows); err != nil {
		return sessionEntry{}, false
	}
	if _, err := strconv.Atoi(sessionAttached); err != nil {
		return sessionEntry{}, false
	}

	return sessionEntry{
		sortValue: sessionDisplayValue(sessionName, sessionKey),
		item:      newSessionItem(sessionID, sessionName, sessionKey, sessionWindows, sessionAttached, display),
	}, true
}

func splitSessionFields(line string) ([]string, bool) {
	fields, ok := splitTmuxFields(line, sessionLineFieldCount)
	if ok {
		return fields, true
	}
	legacyFields, ok := splitTmuxFields(line, sessionLineFieldCount-1)
	if !ok {
		return nil, false
	}
	return append(legacyFields, ""), true
}

func newSessionItem(sessionID, sessionName, sessionKey, sessionWindows, sessionAttached string, displayOptions DisplayOptions) item.Item {
	displayText := "tmux ses " + displayOptions.formatSessionDisplay(sessionName, sessionKey)
	it := item.NewItem()
	it.Type = "session"
	it.Source = "tmux"
	it.Display = displayText
	it.Action = item.ActionNextList
	it.Data["session_attached"] = sessionAttached
	it.Data["session_display"] = displayText
	it.Data["session_id"] = sessionID
	if sessionKey != "" {
		it.Data["session_key"] = sessionKey
	}
	it.Data["session_kind"] = sessionKindExternal
	it.Data["session_name"] = sessionName
	it.Data["session_windows"] = sessionWindows
	return it
}

func ListSessions(ctx context.Context) ([]item.Item, error) {
	return ListSessionsWithDisplay(ctx, DisplayOptions{})
}

func ListSessionsWithDisplay(ctx context.Context, display DisplayOptions) ([]item.Item, error) {
	out, err := tmuxOutput(ctx, "list-sessions", "-F", sessionListFormat)
	if err != nil {
		return nil, err
	}
	return ParseSessionsWithDisplay(string(out), display)
}
