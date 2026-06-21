package tmux

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/jmcampanini/cmdk/internal/item"
)

const SessionKindExternal = "external"

const (
	sessionLineIDField = iota
	sessionLineNameField
	sessionLineWindowsField
	sessionLineAttachedField
	sessionLineFieldCount
)

var sessionListFormat = tmuxFormatFields(
	"#{session_id}",
	tmuxEscapedSessionNameFormat,
	"#{session_windows}",
	"#{session_attached}",
)

func ParseSessions(output string) ([]item.Item, error) {
	lines := strings.Split(output, "\n")
	entries := make([]sessionEntry, 0, len(lines))
	malformedRows := 0

	for _, line := range lines {
		line = cleanTmuxLine(line)
		if line == "" {
			continue
		}

		entry, ok := parseSessionLine(line)
		if !ok {
			malformedRows++
			continue
		}
		entries = append(entries, entry)
	}

	if len(entries) == 0 {
		if malformedRows > 0 {
			return nil, fmt.Errorf("could not parse any tmux list-sessions rows (%d unparseable)", malformedRows)
		}
		return nil, nil
	}

	sort.SliceStable(entries, func(i, j int) bool {
		return entries[i].sortName < entries[j].sortName
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
	sortName string
	item     item.Item
}

func parseSessionLine(line string) (sessionEntry, bool) {
	fields, ok := splitTmuxFields(line, sessionLineFieldCount)
	if !ok {
		return sessionEntry{}, false
	}

	sessionID := fields[sessionLineIDField]
	sessionName := displaySafeTmuxSessionName(fields[sessionLineNameField])
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
		sortName: sessionName,
		item:     newSessionItem(sessionID, sessionName, sessionWindows, sessionAttached),
	}, true
}

func newSessionItem(sessionID, sessionName, sessionWindows, sessionAttached string) item.Item {
	display := "tmux: " + sessionName
	it := item.NewItem()
	it.Type = "session"
	it.Source = "tmux"
	it.Display = display
	it.Action = item.ActionNextList
	it.Data["session_attached"] = sessionAttached
	it.Data["session_display"] = display
	it.Data["session_id"] = sessionID
	it.Data["session_kind"] = SessionKindExternal
	it.Data["session_name"] = sessionName
	it.Data["session_windows"] = sessionWindows
	return it
}

func ListSessions(ctx context.Context) ([]item.Item, error) {
	out, err := tmuxOutput(ctx, "list-sessions", "-F", sessionListFormat)
	if err != nil {
		return nil, err
	}
	return ParseSessions(string(out))
}
