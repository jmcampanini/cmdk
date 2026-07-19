package execute

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"text/template"
	"time"
	"unicode"

	"github.com/jmcampanini/cmdk/internal/cmdrun"
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

	launchModeSessionWindow launchMode = config.LaunchModeSessionWindow
	launchModeShell         launchMode = config.LaunchModeShell
)

var (
	resolveSessionPlan          = resolver.Resolve
	createResolvedSessionWindow = tmux.CreateResolvedSessionWindow
	getwd                       = os.Getwd
	chdir                       = os.Chdir
	lookPath                    = cmdrun.LookPath
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

// templateData merges FlattenData(accumulated), selected.Data, and pane_id:
// the exact variable set launch templates see.
func templateData(accumulated []item.Item, selected item.Item, paneID string) map[string]string {
	data := FlattenData(accumulated)
	maps.Copy(data, selected.Data)
	if paneID != "" {
		data["pane_id"] = paneID
	}
	return data
}

// Launch is a fully resolved, validated launch plan. ResolveLaunch performs
// every user-config interpretation that can fail (templates, launch-path
// resolution, validation); Execute performs only launch mechanics.
type Launch struct {
	mode             launchMode
	path             string
	windowName       string
	windowNameMaxLen int
	command          []string
	newShell         bool
	argv0            string
	argv             []string
	env              []string
	resolveTimeout   time.Duration
	tmuxTimeouts     tmux.Timeouts
}

// ResolveLaunch also returns the template-data map as of the point resolution
// stopped, so failure diagnostics show the exact variable set the failing
// template saw (including launch_path/launch_basename once resolved).
func ResolveLaunch(accumulated []item.Item, selected item.Item, paneID string, cfg config.Config) (Launch, map[string]string, error) {
	data := templateData(accumulated, selected, paneID)

	if selected.Cmd == "" && !selected.NewShell {
		return Launch{}, data, fmt.Errorf("selected item has no command to execute (display: %q)", selected.Display)
	}

	mode := effectiveLaunchMode(selected)
	if selected.NewShell && mode != launchModeSessionWindow {
		return Launch{}, data, errors.New("new shell action requires session-window launch_mode")
	}
	launchPath, hasLaunchPath, err := resolveEffectiveLaunchPath(selected, data, mode, cfg.Timeout.Picker, paneID)
	if err != nil {
		return Launch{}, data, err
	}
	if hasLaunchPath {
		data["launch_path"] = launchPath
		data["launch_basename"] = filepath.Base(filepath.Clean(launchPath))
	}

	switch mode {
	case launchModeSessionWindow:
		if !hasLaunchPath {
			return Launch{}, data, errors.New("session-window action requires a launch_path")
		}
		command, err := sessionWindowCommand(selected, data)
		if err != nil {
			return Launch{}, data, err
		}
		windowName, err := renderWindowName(selected, data)
		if err != nil {
			return Launch{}, data, err
		}
		return Launch{
			mode:             mode,
			path:             launchPath,
			windowName:       windowName,
			windowNameMaxLen: cfg.Behavior.WindowNameMaxLength,
			command:          command,
			newShell:         selected.NewShell,
			resolveTimeout:   cfg.Timeout.EffectiveFetch(),
			tmuxTimeouts: tmux.Timeouts{
				Query:    cfg.Timeout.EffectiveFetch(),
				Mutation: cfg.Timeout.EffectiveMutation(),
			},
		}, data, nil
	case launchModeShell:
		if selected.WindowName != "" {
			return Launch{}, data, errors.New("window_name is only valid when effective launch_mode is session-window")
		}
		rendered, err := RenderCmd(selected.Cmd, data)
		if err != nil {
			return Launch{}, data, fmt.Errorf("cmd template: %w", err)
		}
		argv0, err := lookPath("sh")
		if err != nil {
			return Launch{}, data, err
		}
		return Launch{
			mode:  mode,
			path:  launchPath,
			argv0: argv0,
			argv:  []string{"sh", "-c", rendered},
			env:   envWithCMDK(data, paneID),
		}, data, nil
	default:
		return Launch{}, data, fmt.Errorf("invalid effective launch_mode %q", mode)
	}
}

func (l Launch) Execute(execFn ExecFn) error {
	switch l.mode {
	case launchModeSessionWindow:
		resolveCtx, cancel := context.WithTimeout(context.Background(), l.resolveTimeout)
		defer cancel()
		plan, err := resolveSessionPlan(resolveCtx, l.path, l.resolveTimeout)
		if err != nil {
			return err
		}
		return createResolvedSessionWindow(context.Background(), plan, l.path, tmux.SessionWindowOptions{
			Name:          l.windowName,
			NewShell:      l.newShell,
			Command:       l.command,
			Switch:        true,
			MaxNameLength: l.windowNameMaxLen,
			Timeouts:      l.tmuxTimeouts,
		})
	case launchModeShell:
		if l.path != "" {
			if err := chdir(l.path); err != nil {
				return fmt.Errorf("chdir to launch_path %s: %w", l.path, err)
			}
		}
		return execFn(l.argv0, l.argv, l.env)
	default:
		return fmt.Errorf("invalid effective launch_mode %q", l.mode)
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

	res, err := cmdrun.Run(cmdrun.Spec{
		Op:         "launch_path_cmd",
		Rendered:   rendered,
		Timeout:    timeout,
		Env:        envWithCMDK(data, paneID),
		SingleLine: true,
		MaxStdout:  launchPathCmdMaxStdoutBytes,
		MaxStderr:  launchPathCmdMaxStderrBytes,
	})
	if err != nil {
		return "", err
	}

	// The command exited zero, so output-contract violations carry ExitCode 0
	// alongside the captured streams.
	outputErr := func(cause error) error {
		return &cmdrun.CommandError{
			Op:       "launch_path_cmd",
			Kind:     cmdrun.KindOutput,
			Command:  rendered,
			Timeout:  timeout,
			ExitCode: 0,
			Stdout:   res.Stdout,
			Stderr:   res.AnnotatedStderr(),
			Err:      cause,
		}
	}

	path, err := parseLaunchPathCmdOutput(res.Stdout)
	if err != nil {
		return "", outputErr(err)
	}
	if !filepath.IsAbs(path) {
		return "", outputErr(errors.New("output must be an absolute path"))
	}
	abs, err := validateExistingDirectory("output", path)
	if err != nil {
		return "", outputErr(err)
	}
	return abs, nil
}

func parseLaunchPathCmdOutput(out string) (string, error) {
	s := out
	if strings.HasSuffix(s, "\r\n") {
		s = strings.TrimSuffix(s, "\r\n")
	} else if strings.HasSuffix(s, "\n") {
		s = strings.TrimSuffix(s, "\n")
	}
	if s == "" {
		return "", errors.New("output cannot be empty")
	}
	if strings.Contains(s, "\n") || strings.Contains(s, "\r") {
		return "", errors.New("output must contain exactly one line")
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
	if strings.ContainsFunc(absPath, unicode.IsControl) {
		return "", fmt.Errorf("%s contains control characters", field)
	}
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

func sessionWindowCommand(selected item.Item, data map[string]string) ([]string, error) {
	if selected.NewShell {
		if selected.Cmd != "" {
			return nil, errors.New("new shell session-window action cannot also set cmd")
		}
		return nil, nil
	}

	renderedCmd, err := RenderCmd(selected.Cmd, data)
	if err != nil {
		return nil, fmt.Errorf("cmd template: %w", err)
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

func envWithCMDK(data map[string]string, paneID string) []string {
	base := slices.DeleteFunc(os.Environ(), func(e string) bool {
		return strings.HasPrefix(e, "CMDK_")
	})
	return slices.Concat(base, BuildCMDKEnvVarsFromData(data, paneID))
}
