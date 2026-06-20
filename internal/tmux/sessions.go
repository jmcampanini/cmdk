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

	type entry struct {
		name string
		item item.Item
	}

	var entries []entry
	malformedRows := 0
	for _, line := range lines {
		line = cleanTmuxLine(line)
		if line == "" {
			continue
		}

		fields, ok := splitTmuxFields(line, sessionLineFieldCount)
		if !ok {
			malformedRows++
			continue
		}

		sessionID := fields[sessionLineIDField]
		sessionName := displaySafeTmuxText(fields[sessionLineNameField])
		sessionWindows := fields[sessionLineWindowsField]
		sessionAttached := fields[sessionLineAttachedField]
		if sessionID == "" || sessionName == "" {
			malformedRows++
			continue
		}
		if _, err := strconv.Atoi(sessionWindows); err != nil {
			malformedRows++
			continue
		}
		if _, err := strconv.Atoi(sessionAttached); err != nil {
			malformedRows++
			continue
		}

		entries = append(entries, entry{
			name: sessionName,
			item: newSessionItem(sessionID, sessionName, sessionWindows, sessionAttached),
		})
	}

	if len(entries) == 0 {
		if malformedRows > 0 {
			return nil, fmt.Errorf("could not parse any tmux list-sessions rows (%d unparseable)", malformedRows)
		}
		return nil, nil
	}

	sort.SliceStable(entries, func(i, j int) bool {
		return entries[i].name < entries[j].name
	})

	items := make([]item.Item, len(entries))
	for i, e := range entries {
		items[i] = e.item
	}
	if malformedRows > 0 {
		items = append(items, newSessionParseErrorItem(malformedRows))
	}
	return items, nil
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

func newSessionParseErrorItem(malformedRows int) item.Item {
	rowWord := "row"
	if malformedRows != 1 {
		rowWord = "rows"
	}

	it := item.NewItem()
	it.Type = "error"
	it.Source = "tmux"
	it.Display = fmt.Sprintf("tmux parse error: %d unparseable list-sessions %s", malformedRows, rowWord)
	return it
}

func ListSessions(ctx context.Context) ([]item.Item, error) {
	out, err := exec.CommandContext(ctx, "tmux", "list-sessions", "-F", sessionListFormat).Output()
	if err != nil {
		if ctx.Err() != nil {
			return nil, fmt.Errorf("tmux did not respond within the configured timeout: %w", err)
		}
		return nil, err
	}
	return ParseSessions(string(out))
}
