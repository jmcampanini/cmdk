package tmux

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"unicode"

	resolver "github.com/jmcampanini/cmdk/internal/session"
)

const (
	cmdkSessionKindOption    = "@cmdk_session_kind"
	cmdkSessionKeyOption     = "@cmdk_session_key"
	cmdkSessionDisplayOption = "@cmdk_session_display"
)

var (
	cmdkSessionKeyListFormat = tmuxFormatFields(
		"#{session_id}",
		tmuxEscapedFormat(cmdkSessionKeyOption),
	)
	connectWindowListFormat = tmuxFormatFields(
		"#{window_index}",
		"#{window_id}",
		tmuxEscapedWindowNameFormat,
	)
	newSessionIDsFormat = tmuxFormatFields("#{session_id}", "#{window_id}")
)

type connector struct {
	runner tmuxRunner
}

// ConnectResolvedSession creates or reuses the tmux session/window described by
// plan and switches the current tmux client to the chosen window.
func ConnectResolvedSession(ctx context.Context, plan resolver.Plan) error {
	return connector{runner: execTmuxRunner{}}.connect(ctx, plan)
}

func (c connector) connect(ctx context.Context, plan resolver.Plan) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if c.runner == nil {
		c.runner = execTmuxRunner{}
	}

	if err := validateConnectionPlan(plan); err != nil {
		return err
	}

	sessionID, err := c.findManagedSession(ctx, plan.SessionKey)
	if err != nil {
		return err
	}

	var windowID string
	if sessionID == "" {
		sessionID, windowID, err = c.createSession(ctx, plan)
		if err != nil {
			return err
		}
		if err := c.setSessionMetadata(ctx, sessionID, plan); err != nil {
			return err
		}
	} else {
		windowID, err = c.ensureWindow(ctx, sessionID, plan)
		if err != nil {
			return err
		}
	}

	return c.switchClient(ctx, sessionID, windowID)
}

func (c connector) output(ctx context.Context, args ...string) ([]byte, error) {
	return c.runner.Output(ctx, args...)
}

func validateConnectionPlan(plan resolver.Plan) error {
	fields := []struct {
		name  string
		value string
	}{
		{name: "session_kind", value: plan.SessionKind},
		{name: "session_key", value: plan.SessionKey},
		{name: "session_display", value: plan.SessionDisplay},
		{name: "launch_path", value: plan.LaunchPath},
		{name: "planned_tmux_session_name", value: plan.PlannedTmuxSessionName},
		{name: "planned_tmux_window_name", value: plan.PlannedTmuxWindowName},
	}
	for _, field := range fields {
		if containsControl(field.value) {
			return fmt.Errorf("session plan field %s contains control characters", field.name)
		}
	}
	return nil
}

func containsControl(s string) bool {
	return strings.ContainsFunc(s, unicode.IsControl)
}

func (c connector) findManagedSession(ctx context.Context, sessionKey string) (string, error) {
	out, err := c.output(ctx, "list-sessions", "-F", cmdkSessionKeyListFormat)
	if err != nil {
		return "", err
	}

	rows, err := parseManagedSessionRows(string(out))
	if err != nil {
		return "", err
	}

	var matches []string
	for _, row := range rows {
		if row.sessionKey == sessionKey {
			matches = append(matches, row.sessionID)
		}
	}

	switch len(matches) {
	case 0:
		return "", nil
	case 1:
		return matches[0], nil
	default:
		return "", fmt.Errorf("multiple tmux sessions have %s=%q", cmdkSessionKeyOption, sessionKey)
	}
}

type managedSessionRow struct {
	sessionID  string
	sessionKey string
}

func parseManagedSessionRows(output string) ([]managedSessionRow, error) {
	lines := tmuxLines(output)
	rows := make([]managedSessionRow, 0, len(lines))
	for i, line := range lines {
		fields, ok := splitTmuxFields(line, 2)
		if !ok {
			return nil, fmt.Errorf("could not parse tmux list-sessions row %d: expected 2 fields", i+1)
		}
		if !validTmuxSessionID.MatchString(fields[0]) {
			return nil, fmt.Errorf("could not parse tmux list-sessions row %d: invalid session_id %q", i+1, fields[0])
		}
		rows = append(rows, managedSessionRow{sessionID: fields[0], sessionKey: fields[1]})
	}
	return rows, nil
}

func (c connector) createSession(ctx context.Context, plan resolver.Plan) (string, string, error) {
	out, err := c.output(ctx,
		"new-session", "-d", "-P", "-F", newSessionIDsFormat,
		"-s", plan.PlannedTmuxSessionName,
		"-n", plan.PlannedTmuxWindowName,
		"-c", plan.LaunchPath,
	)
	if err != nil {
		return "", "", err
	}

	sessionID, windowID, err := parseCreatedSessionIDs(string(out))
	if err != nil {
		return "", "", err
	}
	return sessionID, windowID, nil
}

func parseCreatedSessionIDs(output string) (string, string, error) {
	lines := tmuxLines(output)
	if len(lines) != 1 {
		return "", "", fmt.Errorf("could not parse tmux new-session output: expected 1 row, got %d", len(lines))
	}
	fields, ok := splitTmuxFields(lines[0], 2)
	if !ok {
		return "", "", errors.New("could not parse tmux new-session output: expected session_id and window_id")
	}
	if !validTmuxSessionID.MatchString(fields[0]) {
		return "", "", fmt.Errorf("could not parse tmux new-session output: invalid session_id %q", fields[0])
	}
	if !validTmuxWindowID.MatchString(fields[1]) {
		return "", "", fmt.Errorf("could not parse tmux new-session output: invalid window_id %q", fields[1])
	}
	return fields[0], fields[1], nil
}

func (c connector) setSessionMetadata(ctx context.Context, sessionID string, plan resolver.Plan) error {
	metadata := []struct {
		option string
		value  string
	}{
		{option: cmdkSessionKindOption, value: plan.SessionKind},
		{option: cmdkSessionKeyOption, value: plan.SessionKey},
		{option: cmdkSessionDisplayOption, value: plan.SessionDisplay},
	}
	for _, entry := range metadata {
		if _, err := c.output(ctx, "set-option", "-t", sessionID, entry.option, entry.value); err != nil {
			return err
		}
	}
	return nil
}

func (c connector) ensureWindow(ctx context.Context, sessionID string, plan resolver.Plan) (string, error) {
	windowID, err := c.findWindowByName(ctx, sessionID, plan.PlannedTmuxWindowName)
	if err != nil {
		return "", err
	}
	if windowID != "" {
		return windowID, nil
	}
	return c.createWindow(ctx, sessionID, plan)
}

func (c connector) findWindowByName(ctx context.Context, sessionID, windowName string) (string, error) {
	out, err := c.output(ctx, "list-windows", "-t", sessionID, "-F", connectWindowListFormat)
	if err != nil {
		return "", err
	}

	rows, err := parseConnectWindowRows(string(out))
	if err != nil {
		return "", err
	}

	matches := make([]connectWindowRow, 0, 1)
	for _, row := range rows {
		if row.windowName == windowName {
			matches = append(matches, row)
		}
	}
	if len(matches) == 0 {
		return "", nil
	}
	slices.SortFunc(matches, func(a, b connectWindowRow) int {
		return a.index - b.index
	})
	return matches[0].windowID, nil
}

type connectWindowRow struct {
	index      int
	windowID   string
	windowName string
}

func parseConnectWindowRows(output string) ([]connectWindowRow, error) {
	lines := tmuxLines(output)
	rows := make([]connectWindowRow, 0, len(lines))
	for i, line := range lines {
		fields, ok := splitTmuxFields(line, 3)
		if !ok {
			return nil, fmt.Errorf("could not parse tmux list-windows row %d: expected 3 fields", i+1)
		}
		index, err := strconv.Atoi(fields[0])
		if err != nil {
			return nil, fmt.Errorf("could not parse tmux list-windows row %d: invalid window_index %q", i+1, fields[0])
		}
		if !validTmuxWindowID.MatchString(fields[1]) {
			return nil, fmt.Errorf("could not parse tmux list-windows row %d: invalid window_id %q", i+1, fields[1])
		}
		rows = append(rows, connectWindowRow{index: index, windowID: fields[1], windowName: decodeTmuxLiteralBackslashes(fields[2])})
	}
	return rows, nil
}

func (c connector) createWindow(ctx context.Context, sessionID string, plan resolver.Plan) (string, error) {
	out, err := c.output(ctx,
		"new-window", "-P", "-F", "#{window_id}",
		"-t", sessionID+":",
		"-n", plan.PlannedTmuxWindowName,
		"-c", plan.LaunchPath,
	)
	if err != nil {
		return "", err
	}
	return parseCreatedWindowID(string(out))
}

func parseCreatedWindowID(output string) (string, error) {
	lines := tmuxLines(output)
	if len(lines) != 1 {
		return "", fmt.Errorf("could not parse tmux new-window output: expected 1 row, got %d", len(lines))
	}
	windowID := lines[0]
	if !validTmuxWindowID.MatchString(windowID) {
		return "", fmt.Errorf("could not parse tmux new-window output: invalid window_id %q", windowID)
	}
	return windowID, nil
}

func (c connector) switchClient(ctx context.Context, sessionID, windowID string) error {
	_, err := c.output(ctx, "switch-client", "-t", sessionID+":"+windowID)
	return err
}
