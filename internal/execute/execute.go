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
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"text/template"
	"time"
	"unicode"

	"github.com/jmcampanini/cmdk/internal/config"
	"github.com/jmcampanini/cmdk/internal/item"
	"github.com/jmcampanini/cmdk/internal/pathfmt"
	resolver "github.com/jmcampanini/cmdk/internal/session"
	"github.com/jmcampanini/cmdk/internal/tmux"
)

type ExecFn func(argv0 string, argv []string, envv []string) error

type launchMode string

const (
	defaultWindowNameTemplate = "{{.launch_basename}}"

	launchModeSessionWindow launchMode = config.LaunchModeSessionWindow
	launchModeShell         launchMode = config.LaunchModeShell
)

var (
	resolveSessionPlan          = resolver.Resolve
	createResolvedSessionWindow = tmux.CreateResolvedSessionWindow
	getwd                       = os.Getwd
	chdir                       = os.Chdir
	lookPath                    = exec.LookPath
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

	all := slices.Concat(accumulated, []item.Item{selected})
	data := FlattenData(all)
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
		return runSessionWindow(selected, data, launchPath, hasLaunchPath, cfg, paneID)
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
	if s == "~" || strings.HasPrefix(s, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("launch_path expands ~: %w", err)
		}
		if s == "~" {
			s = home
		} else {
			s = filepath.Join(home, s[2:])
		}
	}
	return expandEnvVarsSafe(s), nil
}

func expandEnvVarsSafe(s string) string {
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
				b.WriteString(os.Getenv(name))
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
		b.WriteString(os.Getenv(s[i+1 : j]))
		i = j
	}
	return b.String()
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

	ctx := context.Background()
	if timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}

	var stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, "sh", "-c", rendered)
	cmd.Stderr = &stderr
	cmd.Env = envWithCMDK(data, paneID)
	out, err := cmd.Output()
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return "", fmt.Errorf("launch_path_cmd timed out after %s", timeout)
		}
		if stderr.Len() > 0 {
			return "", fmt.Errorf("launch_path_cmd failed: %w\nstderr: %s", err, stderr.String())
		}
		return "", fmt.Errorf("launch_path_cmd failed: %w", err)
	}

	path, err := parseLaunchPathCmdOutput(out)
	if err != nil {
		return "", err
	}
	if !filepath.IsAbs(path) {
		return "", errors.New("launch_path_cmd output must be an absolute path")
	}
	return validateExistingDirectory("launch_path_cmd output", path)
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

func runSessionWindow(selected item.Item, data map[string]string, launchPath string, hasLaunchPath bool, cfg config.Config, paneID string) error {
	if !hasLaunchPath {
		return errors.New("session-window action requires a launch_path")
	}

	display, err := sessionDisplayOptions(cfg)
	if err != nil {
		return err
	}

	resolveCtx, cancel := sessionResolveContext(cfg)
	defer cancel()
	plan, err := resolveSessionPlan(resolveCtx, launchPath, display)
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

	return createResolvedSessionWindow(context.Background(), plan, tmux.SessionWindowOptions{
		Name:     windowName,
		NewShell: selected.NewShell,
		Command:  command,
		Env:      BuildCMDKEnvVarsFromData(data, paneID),
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

func sessionDisplayOptions(cfg config.Config) (resolver.DisplayOptions, error) {
	home, err := os.UserHomeDir()
	if err != nil && cfg.Display.ShortenHome != "" {
		return resolver.DisplayOptions{}, fmt.Errorf("cannot shorten home prefix: %w", err)
	}
	if cfg.Display.ShortenHome != "" && home != "" {
		resolvedHome, err := filepath.EvalSymlinks(home)
		if err != nil {
			return resolver.DisplayOptions{}, fmt.Errorf("cannot resolve home prefix: %w", err)
		}
		home = filepath.Clean(resolvedHome)
	}

	return resolver.DisplayOptions{
		Home:        home,
		ShortenHome: cfg.Display.ShortenHome,
		Rules:       pathfmt.CompileRules(cfg.Display.Rules),
		Truncation: pathfmt.Truncation{
			Length: cfg.Display.TruncationLength,
			Symbol: cfg.Display.TruncationSymbol,
		},
	}, nil
}
