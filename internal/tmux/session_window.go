package tmux

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"unicode"

	resolver "github.com/jmcampanini/cmdk/internal/session"
)

const (
	cmdkSessionKindOption    = "@cmdk_session_kind"
	cmdkSessionKeyOption     = "@cmdk_session_key"
	cmdkSessionDisplayOption = "@cmdk_session_display"

	sessionWindowModeErrorMessage = "session window requires exactly one mode: --new or command args after --"
)

var (
	cmdkSessionKeyListFormat = tmuxFormatFields(
		"#{session_id}",
		tmuxEscapedFormat(cmdkSessionKeyOption),
	)
	newSessionIDsFormat = tmuxFormatFields("#{session_id}", "#{window_id}")
)

// SessionWindowOptions controls creation of a new tmux window inside the
// cmdk-managed session described by a resolved session plan. Env entries are
// KEY=value strings passed to tmux via -e for the created window.
type SessionWindowOptions struct {
	Name     string
	NewShell bool
	Command  []string
	Env      []string
	Switch   bool
}

type sessionWindowManager struct {
	runner tmuxRunner
}

// CreateResolvedSessionWindow creates a fresh tmux window inside the
// cmdk-managed session described by plan. If the session does not exist yet,
// the first window created by tmux new-session is the requested window. The
// window is targeted by its returned stable window ID, never by display name.
func CreateResolvedSessionWindow(ctx context.Context, plan resolver.Plan, opts SessionWindowOptions) error {
	return sessionWindowManager{runner: execTmuxRunner{}}.createResolvedWindow(ctx, plan, opts)
}

// AttachResolvedSession attaches the current terminal to the cmdk-managed tmux
// session described by plan, creating that managed session when it is missing.
func AttachResolvedSession(ctx context.Context, plan resolver.Plan) error {
	return sessionWindowManager{runner: execTmuxRunner{}}.attachResolvedSession(ctx, plan)
}

func (m sessionWindowManager) createResolvedWindow(ctx context.Context, plan resolver.Plan, opts SessionWindowOptions) error {
	ctx = m.ensureDefaults(ctx)

	if err := validateSessionPlan(plan); err != nil {
		return err
	}
	windowName := sessionWindowName(plan, opts)
	if err := validateSessionWindowOptions(windowName, opts); err != nil {
		return err
	}

	sessionID, err := m.findManagedSession(ctx, plan.SessionKey)
	if err != nil {
		return err
	}

	shellCommand := shellCommandFromArgv(opts.Command)
	var windowID string
	if sessionID == "" {
		sessionID, windowID, err = m.createManagedSession(ctx, plan, windowName, shellCommand, opts.Env)
	} else {
		windowID, err = m.createWindow(ctx, sessionID, plan.LaunchPath, windowName, shellCommand, opts.Env)
	}
	if err != nil {
		return err
	}

	if opts.Switch {
		return m.switchClient(ctx, sessionID, windowID)
	}
	return nil
}

func (m sessionWindowManager) attachResolvedSession(ctx context.Context, plan resolver.Plan) error {
	ctx = m.ensureDefaults(ctx)

	if err := validateSessionPlan(plan); err != nil {
		return err
	}

	sessionID, err := m.findAttachTargetSession(ctx, plan.SessionKey)
	if err != nil {
		return err
	}
	if sessionID == "" {
		sessionID, _, err = m.createManagedSession(ctx, plan, plan.PlannedTmuxWindowName, "", nil)
		if err != nil {
			return err
		}
	}

	return m.attachSession(ctx, sessionID)
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

func (m sessionWindowManager) output(ctx context.Context, args ...string) ([]byte, error) {
	return m.runner.Output(ctx, args...)
}

func (m sessionWindowManager) run(ctx context.Context, args ...string) error {
	return m.runner.Run(ctx, args...)
}

func sessionWindowName(plan resolver.Plan, opts SessionWindowOptions) string {
	if opts.Name != "" {
		return opts.Name
	}
	return plan.PlannedTmuxWindowName
}

func validateSessionPlan(plan resolver.Plan) error {
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

func validateSessionWindowOptions(windowName string, opts SessionWindowOptions) error {
	haveCommand := len(opts.Command) > 0
	if opts.NewShell && haveCommand {
		return errors.New(sessionWindowModeErrorMessage)
	}
	if !opts.NewShell && !haveCommand {
		return errors.New(sessionWindowModeErrorMessage)
	}
	if containsControl(windowName) {
		return errors.New("window name contains control characters")
	}
	return nil
}

func containsControl(s string) bool {
	return strings.ContainsFunc(s, unicode.IsControl)
}

func isTmuxListSessionsUnavailable(err error) bool {
	// tmux list-sessions exits 1 both when no server is running and when the
	// socket path is absent; attach recovers by trying to create the session.
	var exitErr *exec.ExitError
	return errors.As(err, &exitErr) && exitErr.ExitCode() == 1
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
	out, err := m.output(ctx, "list-sessions", "-F", cmdkSessionKeyListFormat)
	if err != nil {
		return "", err
	}

	rows, err := parseManagedSessionRows(string(out))
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

func (m sessionWindowManager) createManagedSession(ctx context.Context, plan resolver.Plan, windowName, shellCommand string, env []string) (string, string, error) {
	sessionID, windowID, err := m.createSession(ctx, plan, windowName, shellCommand, env)
	if err != nil {
		return "", "", err
	}
	if err := m.setSessionMetadata(ctx, sessionID, plan); err != nil {
		return "", "", err
	}
	return sessionID, windowID, nil
}

func (m sessionWindowManager) createSession(ctx context.Context, plan resolver.Plan, windowName, shellCommand string, env []string) (string, string, error) {
	args := []string{
		"new-session", "-d", "-P", "-F", newSessionIDsFormat,
		"-s", plan.PlannedTmuxSessionName,
		"-n", windowName,
		"-c", plan.LaunchPath,
	}
	args = appendTmuxEnv(args, env)
	if shellCommand != "" {
		args = append(args, shellCommand)
	}

	out, err := m.output(ctx, args...)
	if err != nil {
		return "", "", err
	}

	return parseCreatedSessionIDs(string(out))
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

func (m sessionWindowManager) setSessionMetadata(ctx context.Context, sessionID string, plan resolver.Plan) error {
	metadata := []struct {
		option string
		value  string
	}{
		{option: cmdkSessionKindOption, value: plan.SessionKind},
		{option: cmdkSessionKeyOption, value: plan.SessionKey},
		{option: cmdkSessionDisplayOption, value: plan.SessionDisplay},
	}
	for _, entry := range metadata {
		if _, err := m.output(ctx, "set-option", "-t", sessionID, entry.option, entry.value); err != nil {
			return err
		}
	}
	return nil
}

func (m sessionWindowManager) createWindow(ctx context.Context, sessionID, launchPath, windowName, shellCommand string, env []string) (string, error) {
	args := []string{
		"new-window", "-P", "-F", "#{window_id}",
		"-t", sessionID + ":",
		"-n", windowName,
		"-c", launchPath,
	}
	args = appendTmuxEnv(args, env)
	if shellCommand != "" {
		args = append(args, shellCommand)
	}

	out, err := m.output(ctx, args...)
	if err != nil {
		return "", err
	}
	return parseCreatedWindowID(string(out))
}

func appendTmuxEnv(args []string, env []string) []string {
	for _, entry := range env {
		args = append(args, "-e", entry)
	}
	return args
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

func (m sessionWindowManager) switchClient(ctx context.Context, sessionID, windowID string) error {
	_, err := m.output(ctx, "switch-client", "-t", sessionID+":"+windowID)
	return err
}

func (m sessionWindowManager) attachSession(ctx context.Context, sessionID string) error {
	return m.run(ctx, "attach-session", "-t", sessionID)
}
