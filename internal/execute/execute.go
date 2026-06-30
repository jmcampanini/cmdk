package execute

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"syscall"
	"text/template"
	"time"
	"unicode"

	"github.com/jmcampanini/cmdk/internal/config"
	"github.com/jmcampanini/cmdk/internal/item"
	resolver "github.com/jmcampanini/cmdk/internal/session"
	"github.com/jmcampanini/cmdk/internal/tmux"
)

type ExecFn func(argv0 string, argv []string, envv []string) error

type launchMode string

const (
	defaultWindowNameTemplate = "{{.launch_basename}}"

	launchPathCmdMaxStdoutBytes = 8 * 1024
	launchPathCmdMaxStderrBytes = 32 * 1024
	launchPathCmdPipeDrainDelay = 100 * time.Millisecond

	launchModeSessionWindow launchMode = config.LaunchModeSessionWindow
	launchModeShell         launchMode = config.LaunchModeShell
)

var (
	resolveSessionPlan            = resolver.Resolve
	createResolvedSessionWindow   = tmux.CreateResolvedSessionWindow
	getwd                         = os.Getwd
	chdir                         = os.Chdir
	lookPath                      = exec.LookPath
	launchPathSignalNotifyContext = signal.NotifyContext
)

var tmplFuncs = template.FuncMap{
	"sq": func(s string) string {
		return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
	},
}

func RenderCmd(cmdTemplate string, data map[string]string) (string, error) {
	tmpl, err := template.New("cmd").Funcs(tmplFuncs).Option("missingkey=error").Parse(cmdTemplate)
	if err != nil {
		return "", err
	}
	var buf strings.Builder
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", err
	}
	return buf.String(), nil
}

func FlattenData(accumulated []item.Item) map[string]string {
	merged := make(map[string]string)
	for _, it := range accumulated {
		maps.Copy(merged, it.Data)
	}
	return merged
}

var nonAlphaNum = regexp.MustCompile(`[^a-zA-Z0-9]`)

func NormalizeKey(key string) string {
	return "CMDK_" + strings.ToUpper(nonAlphaNum.ReplaceAllString(key, "_"))
}

func BuildCMDKEnvVars(accumulated []item.Item, paneID string) []string {
	return BuildCMDKEnvVarsFromData(FlattenData(accumulated), paneID)
}

func BuildCMDKEnvVarsFromData(data map[string]string, paneID string) []string {
	normalized := make(map[string]string, len(data)+1)
	for k, v := range data {
		normalized[NormalizeKey(k)] = v
	}
	if paneID != "" {
		normalized["CMDK_PANE_ID"] = paneID
	}
	envs := make([]string, 0, len(normalized))
	for k, v := range normalized {
		envs = append(envs, k+"="+v)
	}
	return envs
}

func Run(accumulated []item.Item, selected item.Item, paneID string, execFn ExecFn) error {
	return RunWithConfig(accumulated, selected, paneID, config.DefaultConfig(), execFn)
}

func RunWithConfig(accumulated []item.Item, selected item.Item, paneID string, cfg config.Config, execFn ExecFn) error {
	if selected.Cmd == "" && !selected.NewShell {
		return fmt.Errorf("selected item has no command to execute (display: %q)", selected.Display)
	}

	data := FlattenData(accumulated)
	maps.Copy(data, selected.Data)
	if paneID != "" {
		data["pane_id"] = paneID
	}

	mode := effectiveLaunchMode(selected)
	if selected.NewShell && mode != launchModeSessionWindow {
		return errors.New("new shell action requires session-window launch_mode")
	}
	launchPath, hasLaunchPath, err := resolveEffectiveLaunchPath(selected, data, mode, cfg.Timeout.Picker, paneID)
	if err != nil {
		return err
	}
	if hasLaunchPath {
		data["launch_path"] = launchPath
		data["launch_basename"] = filepath.Base(filepath.Clean(launchPath))
	}

	switch mode {
	case launchModeSessionWindow:
		return runSessionWindow(selected, data, launchPath, hasLaunchPath, cfg)
	case launchModeShell:
		return runShell(selected, data, launchPath, hasLaunchPath, paneID, execFn)
	default:
		return fmt.Errorf("invalid effective launch_mode %q", mode)
	}
}

func effectiveLaunchMode(selected item.Item) launchMode {
	hasLaunchPath := selected.LaunchPath != "" || selected.LaunchPathCmd != ""
	return launchMode(config.EffectiveLaunchMode(selected.MatchType, selected.LaunchMode, hasLaunchPath))
}

func resolveEffectiveLaunchPath(selected item.Item, data map[string]string, mode launchMode, timeout time.Duration, paneID string) (string, bool, error) {
	switch {
	case selected.LaunchPathCmd != "":
		path, err := resolveLaunchPathCmd(selected.LaunchPathCmd, data, timeout, paneID)
		return path, true, err
	case selected.LaunchPath != "":
		path, err := resolveLaunchPath(selected.LaunchPath, data)
		return path, true, err
	case selected.MatchType == "dir":
		path, err := validateExistingDirectory("launch_path", data["path"])
		return path, true, err
	case mode == launchModeSessionWindow:
		wd, err := getwd()
		if err != nil {
			return "", false, fmt.Errorf("launch_path cwd fallback: %w", err)
		}
		path, err := validateExistingDirectory("launch_path", wd)
		return path, true, err
	default:
		return "", false, nil
	}
}

func resolveLaunchPath(templateText string, data map[string]string) (string, error) {
	expanded, err := safeExpandLaunchPath(templateText)
	if err != nil {
		return "", err
	}
	rendered, err := RenderCmd(expanded, data)
	if err != nil {
		return "", fmt.Errorf("launch_path template: %w", err)
	}
	return validateExistingDirectory("launch_path", rendered)
}

func safeExpandLaunchPath(s string) (string, error) {
	if s != "~" && !strings.HasPrefix(s, "~/") {
		return expandEnvVarsSafe(s)
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("launch_path expands ~: %w", err)
	}
	if s == "~" {
		return expandEnvVarsSafe(home)
	}
	return expandEnvVarsSafe(filepath.Join(home, s[2:]))
}

func expandEnvVarsSafe(s string) (string, error) {
	var b strings.Builder
	for i := 0; i < len(s); {
		if s[i] != '$' {
			b.WriteByte(s[i])
			i++
			continue
		}

		if i+1 >= len(s) {
			b.WriteByte(s[i])
			i++
			continue
		}

		if s[i+1] == '{' {
			end := strings.IndexByte(s[i+2:], '}')
			if end < 0 {
				b.WriteByte(s[i])
				i++
				continue
			}
			name := s[i+2 : i+2+end]
			if !validEnvName(name) {
				b.WriteString(s[i : i+3+end])
			} else {
				value, ok := os.LookupEnv(name)
				if !ok {
					return "", fmt.Errorf("launch_path expands ${%s}: environment variable is not set", name)
				}
				b.WriteString(value)
			}
			i += 3 + end
			continue
		}

		if !isEnvNameStart(rune(s[i+1])) {
			b.WriteByte(s[i])
			i++
			continue
		}
		j := i + 2
		for j < len(s) && isEnvNamePart(rune(s[j])) {
			j++
		}
		name := s[i+1 : j]
		value, ok := os.LookupEnv(name)
		if !ok {
			return "", fmt.Errorf("launch_path expands $%s: environment variable is not set", name)
		}
		b.WriteString(value)
		i = j
	}
	return b.String(), nil
}

func validEnvName(name string) bool {
	if name == "" {
		return false
	}
	for i, r := range name {
		if i == 0 {
			if !isEnvNameStart(r) {
				return false
			}
			continue
		}
		if !isEnvNamePart(r) {
			return false
		}
	}
	return true
}

func isEnvNameStart(r rune) bool {
	return r == '_' || ('A' <= r && r <= 'Z') || ('a' <= r && r <= 'z')
}

func isEnvNamePart(r rune) bool {
	return isEnvNameStart(r) || ('0' <= r && r <= '9')
}

func resolveLaunchPathCmd(cmdTemplate string, data map[string]string, timeout time.Duration, paneID string) (string, error) {
	rendered, err := RenderCmd(cmdTemplate, data)
	if err != nil {
		return "", fmt.Errorf("launch_path_cmd template: %w", err)
	}

	ctx, cancel := launchPathCommandContext(timeout)
	defer cancel()

	stdoutCapture := &launchPathStdoutCapture{limit: launchPathCmdMaxStdoutBytes}
	stderrCapture := &boundedDiagnosticCapture{limit: launchPathCmdMaxStderrBytes}

	// TODO(#87): Replace this local bounded capture with the shared external-command
	// output helper once cmdk standardizes stdout/stderr limits repository-wide.
	cmd := exec.CommandContext(ctx, "sh", "-c", rendered)
	cmd.Env = envWithCMDK(data, paneID)
	cmd.Stdout = stdoutCapture
	cmd.Stderr = stderrCapture
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error { return killCommandGroup(cmd) }
	cmd.WaitDelay = launchPathCmdPipeDrainDelay
	stdoutCapture.onError = func() { _ = killCommandGroup(cmd) }

	// Do not use StdoutPipe/StderrPipe here: os/exec closes those readers from
	// Wait, so racing Wait against reader goroutines can produce partial output.
	// Supplying Writers lets os/exec own that synchronization. WaitDelay bounds
	// the only tolerated local recovery case: the shell exited successfully after
	// printing a valid path, but an orphaned descendant kept stdout/stderr open.
	waitErr := cmd.Run()
	stderrText := formatCommandDiagnostic(stderrCapture.result(), launchPathCmdMaxStderrBytes)

	if ctxErr := ctx.Err(); ctxErr != nil {
		if errors.Is(ctxErr, context.DeadlineExceeded) {
			return "", fmt.Errorf("launch_path_cmd timed out after %s", timeout)
		}
		return "", fmt.Errorf("launch_path_cmd canceled: %w", ctxErr)
	}
	if stdoutErr := stdoutCapture.Err(); stdoutErr != nil {
		return "", stdoutErr
	}
	if waitErr != nil && !errors.Is(waitErr, exec.ErrWaitDelay) {
		if stderrText != "" {
			return "", fmt.Errorf("launch_path_cmd failed: %w\nstderr: %s", waitErr, stderrText)
		}
		return "", fmt.Errorf("launch_path_cmd failed: %w", waitErr)
	}

	path, err := parseLaunchPathCmdOutput(stdoutCapture.Bytes())
	if err != nil {
		return "", err
	}
	if !filepath.IsAbs(path) {
		return "", errors.New("launch_path_cmd output must be an absolute path")
	}
	return validateExistingDirectory("launch_path_cmd output", path)
}

func launchPathCommandContext(timeout time.Duration) (context.Context, context.CancelFunc) {
	ctx, stopSignals := launchPathSignalNotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	if timeout <= 0 {
		return ctx, stopSignals
	}

	timeoutCtx, timeoutCancel := context.WithTimeout(ctx, timeout)
	return timeoutCtx, func() {
		timeoutCancel()
		stopSignals()
	}
}

func killCommandGroup(cmd *exec.Cmd) error {
	if cmd.Process == nil {
		return nil
	}
	err := syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	if err != nil && !errors.Is(err, syscall.ESRCH) {
		return err
	}
	return nil
}

type launchPathStdoutCapture struct {
	buf         bytes.Buffer
	limit       int
	seenNewline bool
	err         error
	onError     func()
}

func (c *launchPathStdoutCapture) Write(p []byte) (int, error) {
	for i, b := range p {
		if c.err != nil {
			return i, c.err
		}
		if c.seenNewline {
			return i, c.fail(errors.New("launch_path_cmd output must contain exactly one line"))
		}
		if c.buf.Len() >= c.limit {
			return i, c.fail(fmt.Errorf("launch_path_cmd output exceeds %d bytes", c.limit))
		}
		c.buf.WriteByte(b)
		if b == '\n' {
			c.seenNewline = true
		}
	}
	return len(p), nil
}

func (c *launchPathStdoutCapture) fail(err error) error {
	if c.err == nil {
		c.err = err
		if c.onError != nil {
			c.onError()
		}
	}
	return c.err
}

func (c *launchPathStdoutCapture) Bytes() []byte {
	return c.buf.Bytes()
}

func (c *launchPathStdoutCapture) Err() error {
	return c.err
}

type boundedDiagnosticCapture struct {
	buf       bytes.Buffer
	limit     int
	truncated bool
}

func (c *boundedDiagnosticCapture) Write(p []byte) (int, error) {
	written := len(p)
	if written == 0 {
		return 0, nil
	}

	remaining := c.limit - c.buf.Len()
	if remaining <= 0 {
		c.truncated = true
		return written, nil
	}
	if written > remaining {
		p = p[:remaining]
		c.truncated = true
	}
	c.buf.Write(p)
	return written, nil
}

func (c *boundedDiagnosticCapture) result() commandOutputResult {
	return commandOutputResult{data: c.buf.Bytes(), truncated: c.truncated}
}

type commandOutputResult struct {
	data      []byte
	truncated bool
	err       error
}

func formatCommandDiagnostic(result commandOutputResult, limit int) string {
	text := string(result.data)
	var notes []string
	if result.truncated {
		notes = append(notes, fmt.Sprintf("truncated after %d bytes", limit))
	}
	if result.err != nil {
		notes = append(notes, fmt.Sprintf("read error: %v", result.err))
	}
	if len(notes) == 0 {
		return text
	}
	if text != "" && !strings.HasSuffix(text, "\n") {
		text += "\n"
	}
	return text + "[stderr " + strings.Join(notes, "; ") + "]"
}

func parseLaunchPathCmdOutput(out []byte) (string, error) {
	s := string(out)
	if strings.HasSuffix(s, "\r\n") {
		s = strings.TrimSuffix(s, "\r\n")
	} else if strings.HasSuffix(s, "\n") {
		s = strings.TrimSuffix(s, "\n")
	}
	if s == "" {
		return "", errors.New("launch_path_cmd output cannot be empty")
	}
	if strings.Contains(s, "\n") || strings.Contains(s, "\r") {
		return "", errors.New("launch_path_cmd output must contain exactly one line")
	}
	return s, nil
}

func validateExistingDirectory(field, path string) (string, error) {
	if path == "" {
		if field == "launch_path" {
			return "", errors.New("launch_path rendered empty")
		}
		return "", fmt.Errorf("%s cannot be empty", field)
	}
	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("%s absolute path: %w", field, err)
	}
	absPath = filepath.Clean(absPath)
	info, err := os.Stat(absPath)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return "", fmt.Errorf("%s does not exist: %s", field, absPath)
		}
		return "", fmt.Errorf("%s is not accessible: %w", field, err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("%s is not a directory: %s", field, absPath)
	}
	return absPath, nil
}

func runSessionWindow(selected item.Item, data map[string]string, launchPath string, hasLaunchPath bool, cfg config.Config) error {
	if !hasLaunchPath {
		return errors.New("session-window action requires a launch_path")
	}

	resolveCtx, cancel := sessionResolveContext(cfg)
	defer cancel()
	plan, err := resolveSessionPlan(resolveCtx, launchPath)
	if err != nil {
		return err
	}

	command, err := sessionWindowCommand(selected, data)
	if err != nil {
		return err
	}

	windowName, err := renderWindowName(selected, data)
	if err != nil {
		return err
	}

	return createResolvedSessionWindow(context.Background(), plan, launchPath, tmux.SessionWindowOptions{
		Name:     windowName,
		NewShell: selected.NewShell,
		Command:  command,
		Switch:   true,
	})
}

func sessionWindowCommand(selected item.Item, data map[string]string) ([]string, error) {
	if selected.NewShell {
		if selected.Cmd != "" {
			return nil, errors.New("new shell session-window action cannot also set cmd")
		}
		return nil, nil
	}

	renderedCmd, err := RenderCmd(selected.Cmd, data)
	if err != nil {
		return nil, err
	}
	return []string{"sh", "-lc", renderedCmd}, nil
}

func renderWindowName(selected item.Item, data map[string]string) (string, error) {
	templateText := selected.WindowName
	if templateText == "" {
		templateText = defaultWindowNameTemplate
	}
	name, err := RenderCmd(templateText, data)
	if err != nil {
		return "", fmt.Errorf("window_name template: %w", err)
	}
	if name == "" {
		return "", errors.New("window_name rendered empty")
	}
	if strings.ContainsFunc(name, unicode.IsControl) {
		return "", errors.New("window_name contains control characters")
	}
	return name, nil
}

func runShell(selected item.Item, data map[string]string, launchPath string, hasLaunchPath bool, paneID string, execFn ExecFn) error {
	if selected.WindowName != "" {
		return errors.New("window_name is only valid when effective launch_mode is session-window")
	}

	rendered, err := RenderCmd(selected.Cmd, data)
	if err != nil {
		return err
	}

	shPath, err := lookPath("sh")
	if err != nil {
		return err
	}

	envv := envWithCMDK(data, paneID)
	if hasLaunchPath {
		if err := chdir(launchPath); err != nil {
			return fmt.Errorf("chdir to launch_path %s: %w", launchPath, err)
		}
	}
	return execFn(shPath, []string{"sh", "-c", rendered}, envv)
}

func envWithCMDK(data map[string]string, paneID string) []string {
	base := slices.DeleteFunc(os.Environ(), func(e string) bool {
		return strings.HasPrefix(e, "CMDK_")
	})
	return slices.Concat(base, BuildCMDKEnvVarsFromData(data, paneID))
}

func sessionResolveContext(cfg config.Config) (context.Context, context.CancelFunc) {
	timeout := cfg.Timeout.Fetch
	if timeout <= 0 {
		timeout = config.DefaultConfig().Timeout.Fetch
	}
	return context.WithTimeout(context.Background(), timeout)
}
