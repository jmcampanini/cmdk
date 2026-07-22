package tmux

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"
	"unicode"

	"github.com/jmcampanini/cmdk/internal/cmdrun"
	resolver "github.com/jmcampanini/cmdk/internal/session"
)

const (
	cmdkSessionKindOption = "@cmdk_session_kind"
	cmdkSessionKeyOption  = "@cmdk_session_key"

	sessionWindowModeErrorMessage = "session window requires exactly one mode: --new or command args after --"
)

var (
	cmdkSessionKeyListFormat = tmuxFormatFields(
		"#{session_id}",
		tmuxEscapedFormat(cmdkSessionKeyOption),
	)
	newSessionResultFormat = tmuxFormatFields(
		"#{session_id}",
		"#{window_id}",
		"#{pane_id}",
		tmuxEscapedWindowNameFormat,
	)
	newWindowResultFormat = tmuxFormatFields(
		"#{window_id}",
		"#{pane_id}",
		tmuxEscapedWindowNameFormat,
	)
	tmuxSessionNameReplacer = strings.NewReplacer(".", "_", ":", "_")
)

// SessionWindowOptions controls creation of a new tmux window inside the
// cmdk-managed session described by a resolved session plan.
type SessionWindowOptions struct {
	Name          string
	NewShell      bool
	Command       []string
	Switch        bool
	MaxNameLength int
	Timeouts      Timeouts
	TargetClient  ClientTarget
}

// SessionWindowResult identifies a newly created tmux window and its first pane.
type SessionWindowResult struct {
	SessionID  string
	SessionKey string
	WindowID   string
	WindowName string
	PaneID     string
}

// AttachOptions controls attaching the caller's terminal to a cmdk-managed
// tmux session.
type AttachOptions struct {
	WindowName    string
	MaxNameLength int
	Timeouts      Timeouts
	// Terminal carries the streams tmux attach-session takes over; the
	// composition root passes the process's real terminal.
	Terminal TerminalIO
}

type sessionWindowManager struct {
	runner   tmuxRunner
	timeouts Timeouts
}

// CreateResolvedSessionWindow creates a fresh tmux window inside the
// cmdk-managed session described by plan. If the session does not exist yet,
// the first window created by tmux new-session is the requested window. The
// window is targeted by its returned stable window ID, never by display name.
func CreateResolvedSessionWindow(ctx context.Context, plan resolver.Plan, launchPath string, opts SessionWindowOptions) (SessionWindowResult, error) {
	m := sessionWindowManager{runner: execTmuxRunner{}, timeouts: opts.Timeouts}
	return m.createResolvedWindow(ctx, plan, launchPath, opts)
}

// AttachResolvedSession attaches the caller's terminal to the cmdk-managed
// tmux session described by plan, creating that managed session when it is
// missing. The attach itself streams to opts.Terminal and blocks, without a
// deadline, until the user detaches.
func AttachResolvedSession(ctx context.Context, plan resolver.Plan, launchPath string, opts AttachOptions) error {
	m := sessionWindowManager{runner: execTmuxRunner{}, timeouts: opts.Timeouts}
	return m.attachResolvedSession(ctx, plan, launchPath, opts)
}

func (m sessionWindowManager) createResolvedWindow(ctx context.Context, plan resolver.Plan, launchPath string, opts SessionWindowOptions) (SessionWindowResult, error) {
	ctx = m.ensureDefaults(ctx)

	if err := validateSessionPlan(plan); err != nil {
		return SessionWindowResult{}, err
	}
	if err := validateLaunchPath(launchPath); err != nil {
		return SessionWindowResult{}, err
	}
	windowName := opts.Name
	if err := validateSessionWindowOptions(windowName, opts); err != nil {
		return SessionWindowResult{}, err
	}
	windowName = truncateWindowName(windowName, opts.MaxNameLength)

	sessionID, err := m.findManagedSession(ctx, plan.SessionKey)
	if err != nil {
		return SessionWindowResult{}, err
	}
	if !opts.TargetClient.isZero() {
		if err := validateAttachedClient(ctx, m.timeouts.Query, m.runner, opts.TargetClient); err != nil {
			return SessionWindowResult{}, fmt.Errorf("target tmux client: %w", err)
		}
	}

	shellCommand := shellCommandFromArgv(opts.Command)
	var result SessionWindowResult
	if sessionID == "" {
		result, err = m.createManagedSession(ctx, plan, launchPath, windowName, shellCommand)
	} else {
		result, err = m.createWindow(ctx, sessionID, launchPath, windowName, shellCommand, !opts.Switch)
		if err == nil {
			result.SessionID = sessionID
			result.SessionKey = plan.SessionKey
		}
	}
	if err != nil {
		return result, err
	}

	if opts.Switch {
		return result, m.switchClient(ctx, result.SessionID, result.WindowID, opts.TargetClient.Name)
	}
	return result, nil
}

func (m sessionWindowManager) attachResolvedSession(ctx context.Context, plan resolver.Plan, launchPath string, opts AttachOptions) error {
	ctx = m.ensureDefaults(ctx)

	if err := validateSessionPlan(plan); err != nil {
		return err
	}
	if err := validateLaunchPath(launchPath); err != nil {
		return err
	}
	windowName := opts.WindowName
	if err := validateWindowName(windowName); err != nil {
		return err
	}
	windowName = truncateWindowName(windowName, opts.MaxNameLength)

	sessionID, err := m.findAttachTargetSession(ctx, plan.SessionKey)
	if err != nil {
		return err
	}
	if sessionID == "" {
		result, createErr := m.createManagedSession(ctx, plan, launchPath, windowName, "")
		if createErr != nil {
			return createErr
		}
		sessionID = result.SessionID
	}

	return m.attachSession(ctx, sessionID, opts.Terminal)
}

func (m *sessionWindowManager) ensureDefaults(ctx context.Context) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	if m.runner == nil {
		m.runner = execTmuxRunner{}
	}
	return ctx
}

func (m sessionWindowManager) query(ctx context.Context, shape cmdrun.Shape, timeout time.Duration, args ...string) (cmdrun.Result, error) {
	return m.runner.query(ctx, tmuxQuerySpec(shape, timeout, args...))
}

func validateSessionPlan(plan resolver.Plan) error {
	fields := []struct {
		name  string
		value string
	}{
		{name: "session_kind", value: plan.SessionKind},
		{name: "session_key", value: plan.SessionKey},
	}
	for _, field := range fields {
		if containsControl(field.value) {
			return fmt.Errorf("session plan field %s contains control characters", field.name)
		}
	}
	return nil
}

func validateSessionWindowOptions(windowName string, opts SessionWindowOptions) error {
	if windowName == "" {
		return errors.New("window name cannot be empty")
	}
	if !opts.TargetClient.isZero() {
		if !opts.Switch {
			return errors.New("target client requires switching")
		}
		if err := validateClientTarget(opts.TargetClient); err != nil {
			return err
		}
	}
	haveCommand := len(opts.Command) > 0
	if opts.NewShell && haveCommand {
		return errors.New(sessionWindowModeErrorMessage)
	}
	if !opts.NewShell && !haveCommand {
		return errors.New(sessionWindowModeErrorMessage)
	}
	return validateWindowName(windowName)
}

func validateWindowName(windowName string) error {
	if windowName == "" {
		return errors.New("window name cannot be empty")
	}
	if containsControl(windowName) {
		return errors.New("window name contains control characters")
	}
	return nil
}

const windowNameEllipsis = "…"

// truncateWindowName end-truncates windowName to at most max runes, with the
// appended ellipsis counting toward the limit. max <= 0 disables truncation.
func truncateWindowName(windowName string, max int) string {
	if max <= 0 {
		return windowName
	}
	runes := []rune(windowName)
	if len(runes) <= max {
		return windowName
	}
	return string(runes[:max-1]) + windowNameEllipsis
}

func validateLaunchPath(launchPath string) error {
	if launchPath == "" {
		return errors.New("launch path cannot be empty")
	}
	if containsControl(launchPath) {
		return errors.New("launch path contains control characters")
	}
	return nil
}

func containsControl(s string) bool {
	return strings.ContainsFunc(s, unicode.IsControl)
}

func isTmuxListSessionsUnavailable(err error) bool {
	// tmux list-sessions exits 1 both when no server is running and when the
	// socket path is absent; attach recovers by trying to create the session.
	var cmdErr *cmdrun.CommandError
	return errors.As(err, &cmdErr) && cmdErr.Kind == cmdrun.KindExit && cmdErr.ExitCode == 1
}

func (m sessionWindowManager) findAttachTargetSession(ctx context.Context, sessionKey string) (string, error) {
	sessionID, err := m.findManagedSession(ctx, sessionKey)
	if err == nil {
		return sessionID, nil
	}
	if isTmuxListSessionsUnavailable(err) {
		return "", nil
	}
	return "", err
}

func (m sessionWindowManager) findManagedSession(ctx context.Context, sessionKey string) (string, error) {
	res, err := m.query(ctx, cmdrun.ShapeLines, m.timeouts.Query, "list-sessions", "-F", cmdkSessionKeyListFormat)
	if err != nil {
		return "", err
	}

	rows, err := parseManagedSessionRows(res.Stdout)
	if err != nil {
		return "", err
	}

	var match string
	for _, row := range rows {
		if row.sessionKey != sessionKey {
			continue
		}
		if match != "" {
			return "", fmt.Errorf("multiple tmux sessions have %s=%q", cmdkSessionKeyOption, sessionKey)
		}
		match = row.sessionID
	}
	return match, nil
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

func (m sessionWindowManager) createManagedSession(ctx context.Context, plan resolver.Plan, launchPath, windowName, shellCommand string) (SessionWindowResult, error) {
	result, err := m.createSession(ctx, plan, launchPath, windowName, shellCommand)
	if err != nil {
		return result, err
	}
	if err := m.setSessionMetadata(ctx, result.SessionID, plan); err != nil {
		return result, err
	}
	return result, nil
}

func (m sessionWindowManager) createSession(ctx context.Context, plan resolver.Plan, launchPath, windowName, shellCommand string) (SessionWindowResult, error) {
	args := []string{
		"new-session", "-d", "-P", "-F", newSessionResultFormat,
		"-s", tmuxSafeSessionName(plan.SessionKey),
		"-n", windowName,
		"-c", launchPath,
	}
	if shellCommand != "" {
		args = append(args, shellCommand)
	}

	res, err := m.query(ctx, cmdrun.ShapeSingleLine, m.timeouts.Mutation, args...)
	if err != nil {
		return SessionWindowResult{}, err
	}

	result, err := parseCreatedSessionResult(res.Stdout)
	if err != nil {
		return result, err
	}
	result.SessionKey = plan.SessionKey
	return result, nil
}

func parseCreatedSessionResult(output string) (SessionWindowResult, error) {
	lines := tmuxLines(output)
	if len(lines) != 1 {
		return SessionWindowResult{}, fmt.Errorf("could not parse tmux new-session output: expected 1 row, got %d", len(lines))
	}
	fields, ok := splitTmuxFields(lines[0], 4)
	if !ok {
		return SessionWindowResult{}, errors.New("could not parse tmux new-session output: expected session_id, window_id, pane_id, and window_name")
	}
	if !validTmuxSessionID.MatchString(fields[0]) {
		return SessionWindowResult{}, fmt.Errorf("could not parse tmux new-session output: invalid session_id %q", fields[0])
	}
	if !validTmuxWindowID.MatchString(fields[1]) {
		return SessionWindowResult{}, fmt.Errorf("could not parse tmux new-session output: invalid window_id %q", fields[1])
	}
	if !validTmuxPaneID.MatchString(fields[2]) {
		return SessionWindowResult{}, fmt.Errorf("could not parse tmux new-session output: invalid pane_id %q", fields[2])
	}
	return SessionWindowResult{
		SessionID:  fields[0],
		WindowID:   fields[1],
		WindowName: displaySafeTmuxWindowName(fields[3]),
		PaneID:     fields[2],
	}, nil
}

func (m sessionWindowManager) setSessionMetadata(ctx context.Context, sessionID string, plan resolver.Plan) error {
	metadata := []struct {
		option string
		value  string
	}{
		{option: cmdkSessionKindOption, value: plan.SessionKind},
		{option: cmdkSessionKeyOption, value: plan.SessionKey},
	}
	for _, entry := range metadata {
		if _, err := m.query(ctx, cmdrun.ShapeEmpty, m.timeouts.Mutation, "set-option", "-t", sessionID, entry.option, entry.value); err != nil {
			return err
		}
	}
	return nil
}

func (m sessionWindowManager) createWindow(ctx context.Context, sessionID, launchPath, windowName, shellCommand string, detached bool) (SessionWindowResult, error) {
	args := []string{"new-window"}
	if detached {
		args = append(args, "-d")
	}
	args = append(args,
		"-P", "-F", newWindowResultFormat,
		"-t", sessionID+":",
		"-n", windowName,
		"-c", launchPath,
	)
	if shellCommand != "" {
		args = append(args, shellCommand)
	}

	res, err := m.query(ctx, cmdrun.ShapeSingleLine, m.timeouts.Mutation, args...)
	if err != nil {
		return SessionWindowResult{}, err
	}
	return parseCreatedWindowResult(res.Stdout)
}

func parseCreatedWindowResult(output string) (SessionWindowResult, error) {
	lines := tmuxLines(output)
	if len(lines) != 1 {
		return SessionWindowResult{}, fmt.Errorf("could not parse tmux new-window output: expected 1 row, got %d", len(lines))
	}
	fields, ok := splitTmuxFields(lines[0], 3)
	if !ok {
		return SessionWindowResult{}, errors.New("could not parse tmux new-window output: expected window_id, pane_id, and window_name")
	}
	if !validTmuxWindowID.MatchString(fields[0]) {
		return SessionWindowResult{}, fmt.Errorf("could not parse tmux new-window output: invalid window_id %q", fields[0])
	}
	if !validTmuxPaneID.MatchString(fields[1]) {
		return SessionWindowResult{}, fmt.Errorf("could not parse tmux new-window output: invalid pane_id %q", fields[1])
	}
	return SessionWindowResult{
		WindowID:   fields[0],
		WindowName: displaySafeTmuxWindowName(fields[2]),
		PaneID:     fields[1],
	}, nil
}

func shellCommandFromArgv(argv []string) string {
	if len(argv) == 0 {
		return ""
	}
	parts := make([]string, len(argv))
	for i, arg := range argv {
		parts[i] = shellQuoteArg(arg)
	}
	return strings.Join(parts, " ")
}

func shellQuoteArg(arg string) string {
	return "'" + strings.ReplaceAll(arg, "'", "'\\''") + "'"
}

func tmuxSafeSessionName(sessionKey string) string {
	name := filepath.ToSlash(filepath.Clean(sessionKey))
	name = strings.TrimLeft(name, "/")
	name = tmuxSessionNameReplacer.Replace(name)
	if name == "" || name == "." {
		return "_"
	}
	return name
}

func (m sessionWindowManager) switchClient(ctx context.Context, sessionID, windowID, clientName string) error {
	args := []string{"switch-client"}
	if clientName != "" {
		args = append(args, "-c", clientName)
	}
	args = append(args, "-t", sessionID+":"+windowID)
	_, err := m.query(ctx, cmdrun.ShapeEmpty, m.timeouts.Mutation, args...)
	return err
}

func (m sessionWindowManager) attachSession(ctx context.Context, sessionID string, terminal TerminalIO) error {
	return m.runner.stream(ctx, tmuxStreamSpec(terminal, "attach-session", "-t", sessionID))
}
