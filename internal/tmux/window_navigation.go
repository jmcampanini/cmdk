package tmux

import (
	"context"
	"errors"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strconv"
)

// WindowDirection identifies which adjacent window in the deterministic tmux
// window ring should be selected.
type WindowDirection int

const (
	WindowPrevious WindowDirection = -1
	WindowNext     WindowDirection = 1
)

// WindowSwitchOptions controls relative tmux window navigation.
type WindowSwitchOptions struct {
	// PaneID anchors the current window lookup. When empty, SwitchRelativeWindow
	// falls back to TMUX_PANE and then tmux's default current context.
	PaneID string
}

var (
	validTmuxPaneID             = regexp.MustCompile(`^%\d+$`)
	currentWindowFormat         = tmuxFormatFields("#{session_id}", "#{window_id}")
	windowNavigationListFormat  = tmuxFormatFields("#{session_id}", "#{window_index}", "#{window_id}")
	errInvalidWindowDirection   = errors.New("invalid window direction")
	errNoTmuxWindows            = errors.New("tmux list-windows returned no windows")
	errCurrentWindowNotInWindow = errors.New("current tmux window was not found in window list")
)

// SwitchRelativeWindow switches the current tmux client to the next or previous
// window in a deterministic circular order. Sessions are ordered by numeric
// session_id. Windows within each session are ordered by numeric window_index.
func SwitchRelativeWindow(ctx context.Context, direction WindowDirection, opts WindowSwitchOptions) error {
	return windowNavigator{runner: execTmuxRunner{}, lookupEnv: os.LookupEnv}.switchRelativeWindow(ctx, direction, opts)
}

type windowNavigator struct {
	runner    tmuxRunner
	lookupEnv func(string) (string, bool)
}

type windowIdentity struct {
	sessionID string
	windowID  string
}

type windowRingEntry struct {
	windowIdentity
	sessionNumber uint64
	windowIndex   int64
	windowNumber  uint64
}

func (n windowNavigator) switchRelativeWindow(ctx context.Context, direction WindowDirection, opts WindowSwitchOptions) error {
	ctx = n.ensureDefaults(ctx)
	if direction != WindowNext && direction != WindowPrevious {
		return errInvalidWindowDirection
	}

	paneID, err := n.currentPaneID(opts.PaneID)
	if err != nil {
		return err
	}
	current, err := n.currentWindow(ctx, paneID)
	if err != nil {
		return err
	}
	ring, err := n.windowRing(ctx)
	if err != nil {
		return err
	}
	target, err := adjacentWindow(ring, current, direction)
	if err != nil {
		return err
	}

	_, err = n.output(ctx, "switch-client", "-t", target.sessionID+":"+target.windowID)
	return err
}

func (n *windowNavigator) ensureDefaults(ctx context.Context) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	if n.runner == nil {
		n.runner = execTmuxRunner{}
	}
	if n.lookupEnv == nil {
		n.lookupEnv = os.LookupEnv
	}
	return ctx
}

func (n windowNavigator) output(ctx context.Context, args ...string) ([]byte, error) {
	return n.runner.Output(ctx, args...)
}

func (n windowNavigator) currentPaneID(explicit string) (string, error) {
	if explicit != "" {
		return validatePaneID("--pane-id", explicit)
	}

	if envPaneID, ok := n.lookupEnv("TMUX_PANE"); ok && envPaneID != "" {
		return validatePaneID("TMUX_PANE", envPaneID)
	}
	return "", nil
}

func validatePaneID(source, paneID string) (string, error) {
	if !validTmuxPaneID.MatchString(paneID) {
		return "", fmt.Errorf("%s must be a tmux pane ID like %%1", source)
	}
	return paneID, nil
}

func (n windowNavigator) currentWindow(ctx context.Context, paneID string) (windowIdentity, error) {
	args := []string{"display-message", "-p"}
	if paneID != "" {
		args = append(args, "-t", paneID)
	}
	args = append(args, currentWindowFormat)

	out, err := n.output(ctx, args...)
	if err != nil {
		return windowIdentity{}, err
	}
	return parseCurrentWindow(string(out))
}

func parseCurrentWindow(output string) (windowIdentity, error) {
	lines := tmuxLines(output)
	if len(lines) != 1 {
		return windowIdentity{}, fmt.Errorf("could not parse tmux current window: expected 1 row, got %d", len(lines))
	}
	fields, ok := splitTmuxFields(lines[0], 2)
	if !ok {
		return windowIdentity{}, errors.New("could not parse tmux current window: expected session_id and window_id")
	}
	return parseWindowIdentity(fields[0], fields[1])
}

func parseWindowIdentity(sessionID, windowID string) (windowIdentity, error) {
	if !validTmuxSessionID.MatchString(sessionID) {
		return windowIdentity{}, fmt.Errorf("invalid session_id %q", sessionID)
	}
	if !validTmuxWindowID.MatchString(windowID) {
		return windowIdentity{}, fmt.Errorf("invalid window_id %q", windowID)
	}
	return windowIdentity{sessionID: sessionID, windowID: windowID}, nil
}

func (n windowNavigator) windowRing(ctx context.Context) ([]windowRingEntry, error) {
	out, err := n.output(ctx, "list-windows", "-a", "-F", windowNavigationListFormat)
	if err != nil {
		return nil, err
	}
	return parseWindowRing(string(out))
}

func parseWindowRing(output string) ([]windowRingEntry, error) {
	lines := tmuxLines(output)
	entries := make([]windowRingEntry, 0, len(lines))
	for i, line := range lines {
		fields, ok := splitTmuxFields(line, 3)
		if !ok {
			return nil, fmt.Errorf("could not parse tmux list-windows row %d: expected session_id, window_index, and window_id", i+1)
		}
		entry, err := parseWindowRingEntry(fields[0], fields[1], fields[2])
		if err != nil {
			return nil, fmt.Errorf("could not parse tmux list-windows row %d: %w", i+1, err)
		}
		entries = append(entries, entry)
	}
	if len(entries) == 0 {
		return nil, errNoTmuxWindows
	}

	sort.SliceStable(entries, func(i, j int) bool {
		left, right := entries[i], entries[j]
		if left.sessionNumber != right.sessionNumber {
			return left.sessionNumber < right.sessionNumber
		}
		if left.windowIndex != right.windowIndex {
			return left.windowIndex < right.windowIndex
		}
		return left.windowNumber < right.windowNumber
	})
	return entries, nil
}

func parseWindowRingEntry(sessionID, windowIndex, windowID string) (windowRingEntry, error) {
	identity, err := parseWindowIdentity(sessionID, windowID)
	if err != nil {
		return windowRingEntry{}, err
	}
	sessionNumber, err := parseTmuxIDNumber(sessionID, '$')
	if err != nil {
		return windowRingEntry{}, err
	}
	windowNumber, err := parseTmuxIDNumber(windowID, '@')
	if err != nil {
		return windowRingEntry{}, err
	}
	index, err := strconv.ParseInt(windowIndex, 10, 64)
	if err != nil || index < 0 {
		return windowRingEntry{}, fmt.Errorf("invalid window_index %q", windowIndex)
	}
	return windowRingEntry{
		windowIdentity: identity,
		sessionNumber:  sessionNumber,
		windowIndex:    index,
		windowNumber:   windowNumber,
	}, nil
}

func parseTmuxIDNumber(value string, prefix byte) (uint64, error) {
	if len(value) < 2 || value[0] != prefix {
		return 0, fmt.Errorf("invalid tmux ID %q", value)
	}
	number, err := strconv.ParseUint(value[1:], 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid tmux ID %q", value)
	}
	return number, nil
}

func adjacentWindow(ring []windowRingEntry, current windowIdentity, direction WindowDirection) (windowRingEntry, error) {
	if len(ring) == 0 {
		return windowRingEntry{}, errNoTmuxWindows
	}

	currentIndex := -1
	for i, entry := range ring {
		if entry.sessionID == current.sessionID && entry.windowID == current.windowID {
			currentIndex = i
			break
		}
	}
	if currentIndex < 0 {
		return windowRingEntry{}, errCurrentWindowNotInWindow
	}

	switch direction {
	case WindowNext:
		return ring[(currentIndex+1)%len(ring)], nil
	case WindowPrevious:
		return ring[(currentIndex-1+len(ring))%len(ring)], nil
	default:
		return windowRingEntry{}, errInvalidWindowDirection
	}
}
