package execute

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/jmcampanini/cmdk/internal/cmdrun"
	"github.com/jmcampanini/cmdk/internal/config"
	"github.com/jmcampanini/cmdk/internal/item"
	resolver "github.com/jmcampanini/cmdk/internal/session"
	"github.com/jmcampanini/cmdk/internal/tmux"
)

func resolveAndExecute(t *testing.T, accumulated []item.Item, selected item.Item, paneID string, execFn ExecFn) error {
	t.Helper()
	launch, _, err := ResolveLaunch(accumulated, selected, paneID, config.DefaultConfig())
	if err != nil {
		return err
	}
	return launch.Execute(execFn)
}

func TestRenderCmd(t *testing.T) {
	tmpl := "tmux switch-client -t {{sq .session_id}}:{{sq .window_id}}"
	data := map[string]string{"session_id": "$1", "window_id": "@2"}

	got, err := RenderCmd(tmpl, data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "tmux switch-client -t '$1':'@2'" {
		t.Errorf("got %q", got)
	}
}

func TestRenderCmd_MissingKey(t *testing.T) {
	tmpl := "tmux switch-client -t {{sq .session_id}}:{{sq .window_id}}"
	data := map[string]string{"session_id": "$1"}

	_, err := RenderCmd(tmpl, data)
	if err == nil {
		t.Error("expected error for missing key")
	}
}

func TestFlattenData_Single(t *testing.T) {
	items := []item.Item{
		{Data: map[string]string{"a": "1", "b": "2"}},
	}
	got := FlattenData(items)
	if got["a"] != "1" || got["b"] != "2" {
		t.Errorf("unexpected: %v", got)
	}
}

func TestFlattenData_LastWriteWins(t *testing.T) {
	items := []item.Item{
		{Data: map[string]string{"key": "first"}},
		{Data: map[string]string{"key": "second"}},
	}
	got := FlattenData(items)
	if got["key"] != "second" {
		t.Errorf("got %q, want %q", got["key"], "second")
	}
}

func TestFlattenData_Empty(t *testing.T) {
	got := FlattenData(nil)
	if len(got) != 0 {
		t.Errorf("expected empty map, got %v", got)
	}
}

func TestLaunchExecute_CallsExecFn(t *testing.T) {
	selected := item.Item{
		Cmd:  "tmux switch-client -t {{sq .session_id}}:{{sq .window_id}}",
		Data: map[string]string{"session_id": "$1", "window_id": "@2"},
	}

	var capturedArgv0 string
	var capturedArgv []string
	var capturedEnvv []string

	mockExec := func(argv0 string, argv []string, envv []string) error {
		capturedArgv0 = argv0
		capturedArgv = argv
		capturedEnvv = envv
		return nil
	}

	err := resolveAndExecute(t, nil, selected, "%1", mockExec)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.HasSuffix(capturedArgv0, "/sh") && capturedArgv0 != "sh" {
		t.Errorf("argv0 = %q, want sh path", capturedArgv0)
	}
	if len(capturedArgv) != 3 || capturedArgv[0] != "sh" || capturedArgv[1] != "-c" {
		t.Errorf("argv = %v, want [sh -c <cmd>]", capturedArgv)
	}
	if capturedArgv[2] != "tmux switch-client -t '$1':'@2'" {
		t.Errorf("rendered cmd = %q", capturedArgv[2])
	}
	if len(capturedEnvv) == 0 {
		t.Error("expected envv to be populated")
	}
}

func TestResolveLaunch_SelectedDataAvailableInTemplate(t *testing.T) {
	accumulated := []item.Item{
		{Data: map[string]string{"dir": "/tmp"}},
	}
	selected := item.Item{
		Cmd:  "echo {{.dir}} {{.name}}",
		Data: map[string]string{"name": "test"},
	}

	var capturedArgv []string
	mockExec := func(argv0 string, argv []string, envv []string) error {
		capturedArgv = argv
		return nil
	}

	err := resolveAndExecute(t, accumulated, selected, "%1", mockExec)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if capturedArgv[2] != "echo /tmp test" {
		t.Errorf("rendered cmd = %q, want %q", capturedArgv[2], "echo /tmp test")
	}
}

func TestRenderCmd_SqEscapesSingleQuotes(t *testing.T) {
	tmpl := "tmux new-window -c {{sq .path}}"
	data := map[string]string{"path": "/home/user/jane's-project"}

	got, err := RenderCmd(tmpl, data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := "tmux new-window -c '/home/user/jane'\\''s-project'"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestRenderCmd_SqNormalPath(t *testing.T) {
	tmpl := "tmux new-window -c {{sq .path}}"
	data := map[string]string{"path": "/home/user/projects"}

	got, err := RenderCmd(tmpl, data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "tmux new-window -c '/home/user/projects'" {
		t.Errorf("got %q", got)
	}
}

func TestRenderCmd_TemplateSyntaxInDataIsSafe(t *testing.T) {
	tmpl := "echo {{.name}}"
	data := map[string]string{"name": "{{.evil}}"}

	got, err := RenderCmd(tmpl, data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "echo {{.evil}}" {
		t.Errorf("got %q, want template syntax passed through literally", got)
	}
}

func TestLaunchExecute_ExecFnError(t *testing.T) {
	selected := item.Item{
		Cmd:  "echo hello",
		Data: map[string]string{},
	}

	mockExec := func(argv0 string, argv []string, envv []string) error {
		return errors.New("exec failed")
	}

	err := resolveAndExecute(t, nil, selected, "%1", mockExec)
	if err == nil || err.Error() != "exec failed" {
		t.Errorf("expected exec failed error, got: %v", err)
	}
}

func TestResolveLaunch_MissingKeyDoesNotCallExecFn(t *testing.T) {
	selected := item.Item{
		Cmd:  "echo {{.missing}}",
		Data: map[string]string{},
	}

	called := false
	mockExec := func(argv0 string, argv []string, envv []string) error {
		called = true
		return nil
	}

	err := resolveAndExecute(t, nil, selected, "%1", mockExec)
	if err == nil {
		t.Error("expected error for missing template key")
	}
	if !strings.Contains(err.Error(), "cmd template") {
		t.Errorf("error should name the failing field, got: %v", err)
	}
	if called {
		t.Error("execFn should not be called when template rendering fails")
	}
}

func TestResolveLaunch_EmptyCmd(t *testing.T) {
	selected := item.Item{
		Display: "test item",
		Cmd:     "",
		Data:    map[string]string{},
	}

	called := false
	mockExec := func(argv0 string, argv []string, envv []string) error {
		called = true
		return nil
	}

	err := resolveAndExecute(t, nil, selected, "%1", mockExec)
	if err == nil {
		t.Error("expected error for empty Cmd")
	}
	if !strings.Contains(err.Error(), "no command") {
		t.Errorf("error should mention no command, got: %v", err)
	}
	if called {
		t.Error("execFn should not be called with empty Cmd")
	}
}

func TestNormalizeKey(t *testing.T) {
	tests := []struct {
		input, want string
	}{
		{"path", "CMDK_PATH"},
		{"window-index", "CMDK_WINDOW_INDEX"},
		{"my.key", "CMDK_MY_KEY"},
		{"already_UPPER", "CMDK_ALREADY_UPPER"},
		{"", "CMDK_"},
		{"---", "CMDK____"},
	}
	for _, tt := range tests {
		got := NormalizeKey(tt.input)
		if got != tt.want {
			t.Errorf("NormalizeKey(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestBuildCMDKEnvVars_Basic(t *testing.T) {
	items := []item.Item{
		{Data: map[string]string{
			"path":         "/home/user",
			"session_name": "main",
			"session_id":   "$1",
			"window_id":    "@2",
		}},
	}
	envs := BuildCMDKEnvVars(items, "%1")
	envMap := envSliceToMap(envs)

	if envMap["CMDK_PATH"] != "/home/user" {
		t.Errorf("CMDK_PATH = %q, want /home/user", envMap["CMDK_PATH"])
	}
	if envMap["CMDK_SESSION_NAME"] != "main" {
		t.Errorf("CMDK_SESSION_NAME = %q, want main", envMap["CMDK_SESSION_NAME"])
	}
	if _, ok := envMap["CMDK_SESSION"]; ok {
		t.Error("CMDK_SESSION should not be set; use CMDK_SESSION_NAME")
	}
	if envMap["CMDK_SESSION_ID"] != "$1" {
		t.Errorf("CMDK_SESSION_ID = %q, want $1", envMap["CMDK_SESSION_ID"])
	}
	if envMap["CMDK_WINDOW_ID"] != "@2" {
		t.Errorf("CMDK_WINDOW_ID = %q, want @2", envMap["CMDK_WINDOW_ID"])
	}
	if envMap["CMDK_PANE_ID"] != "%1" {
		t.Errorf("CMDK_PANE_ID = %q, want %%1", envMap["CMDK_PANE_ID"])
	}
}

func TestBuildCMDKEnvVars_UserSessionKeyIsPreserved(t *testing.T) {
	items := []item.Item{{Data: map[string]string{"session": "scratch"}}}
	envs := BuildCMDKEnvVars(items, "")
	envMap := envSliceToMap(envs)

	if envMap["CMDK_SESSION"] != "scratch" {
		t.Errorf("CMDK_SESSION = %q, want scratch", envMap["CMDK_SESSION"])
	}
}

func TestBuildCMDKEnvVars_CollisionLastWriteWins(t *testing.T) {
	items := []item.Item{
		{Data: map[string]string{"path": "/first"}},
		{Data: map[string]string{"path": "/second"}},
	}
	envs := BuildCMDKEnvVars(items, "")
	envMap := envSliceToMap(envs)

	if envMap["CMDK_PATH"] != "/second" {
		t.Errorf("CMDK_PATH = %q, want /second (last-write-wins)", envMap["CMDK_PATH"])
	}
}

func TestBuildCMDKEnvVars_EmptyPaneID(t *testing.T) {
	items := []item.Item{
		{Data: map[string]string{"key": "val"}},
	}
	envs := BuildCMDKEnvVars(items, "")
	for _, e := range envs {
		if strings.HasPrefix(e, "CMDK_PANE_ID=") {
			t.Error("CMDK_PANE_ID should not be set when paneID is empty")
		}
	}
}

func TestBuildCMDKEnvVars_MultipleItems(t *testing.T) {
	items := []item.Item{
		{Data: map[string]string{"session_name": "dev"}},
		{Data: map[string]string{"path": "/projects"}},
	}
	envs := BuildCMDKEnvVars(items, "%5")
	envMap := envSliceToMap(envs)

	if envMap["CMDK_SESSION_NAME"] != "dev" {
		t.Errorf("CMDK_SESSION_NAME = %q, want dev", envMap["CMDK_SESSION_NAME"])
	}
	if _, ok := envMap["CMDK_SESSION"]; ok {
		t.Error("CMDK_SESSION should not be set; use CMDK_SESSION_NAME")
	}
	if envMap["CMDK_PATH"] != "/projects" {
		t.Errorf("CMDK_PATH = %q, want /projects", envMap["CMDK_PATH"])
	}
	if envMap["CMDK_PANE_ID"] != "%5" {
		t.Errorf("CMDK_PANE_ID = %q, want %%5", envMap["CMDK_PANE_ID"])
	}
}

func TestResolveLaunch_EnvVarsContainCMDK(t *testing.T) {
	selected := item.Item{
		Cmd:  "echo hi",
		Data: map[string]string{"session_name": "main"},
	}

	var capturedEnvv []string
	mockExec := func(argv0 string, argv []string, envv []string) error {
		capturedEnvv = envv
		return nil
	}

	err := resolveAndExecute(t, nil, selected, "%3", mockExec)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	envMap := envSliceToMap(capturedEnvv)
	if envMap["CMDK_SESSION_NAME"] != "main" {
		t.Errorf("CMDK_SESSION_NAME = %q, want main", envMap["CMDK_SESSION_NAME"])
	}
	if _, ok := envMap["CMDK_SESSION"]; ok {
		t.Error("CMDK_SESSION should not be set; use CMDK_SESSION_NAME")
	}
	if envMap["CMDK_PANE_ID"] != "%3" {
		t.Errorf("CMDK_PANE_ID = %q, want %%3", envMap["CMDK_PANE_ID"])
	}
}

func TestResolveLaunch_StripsExistingCMDKVars(t *testing.T) {
	t.Setenv("CMDK_STALE", "leftover")

	selected := item.Item{
		Cmd:  "echo hi",
		Data: map[string]string{"session_name": "main"},
	}

	var capturedEnvv []string
	mockExec := func(argv0 string, argv []string, envv []string) error {
		capturedEnvv = envv
		return nil
	}

	err := resolveAndExecute(t, nil, selected, "%1", mockExec)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	envMap := envSliceToMap(capturedEnvv)
	if _, ok := envMap["CMDK_STALE"]; ok {
		t.Error("CMDK_STALE should be stripped from env")
	}
	if envMap["CMDK_SESSION_NAME"] != "main" {
		t.Errorf("CMDK_SESSION_NAME = %q, want main", envMap["CMDK_SESSION_NAME"])
	}
	if _, ok := envMap["CMDK_SESSION"]; ok {
		t.Error("CMDK_SESSION should not be set; use CMDK_SESSION_NAME")
	}
	if envMap["CMDK_PANE_ID"] != "%1" {
		t.Errorf("CMDK_PANE_ID = %q, want %%1", envMap["CMDK_PANE_ID"])
	}
}

func TestResolveLaunch_PaneIDAvailableInTemplate(t *testing.T) {
	selected := item.Item{
		Cmd:  "tmux split-window -t {{.pane_id}}",
		Data: map[string]string{},
	}

	var capturedArgv []string
	mockExec := func(argv0 string, argv []string, envv []string) error {
		capturedArgv = argv
		return nil
	}

	err := resolveAndExecute(t, nil, selected, "%5", mockExec)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if capturedArgv[2] != "tmux split-window -t %5" {
		t.Errorf("rendered cmd = %q, want %q", capturedArgv[2], "tmux split-window -t %5")
	}
}

func TestResolveLaunch_PaneIDWithSq(t *testing.T) {
	selected := item.Item{
		Cmd:  "tmux split-window -t {{sq .pane_id}} -c {{sq .path}}",
		Data: map[string]string{"path": "/home/user"},
	}

	var capturedArgv []string
	mockExec := func(argv0 string, argv []string, envv []string) error {
		capturedArgv = argv
		return nil
	}

	err := resolveAndExecute(t, nil, selected, "%5", mockExec)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := "tmux split-window -t '%5' -c '/home/user'"
	if capturedArgv[2] != want {
		t.Errorf("rendered cmd = %q, want %q", capturedArgv[2], want)
	}
}

func TestResolveLaunch_EmptyPaneIDNotInTemplateData(t *testing.T) {
	selected := item.Item{
		Cmd:  "echo {{.pane_id}}",
		Data: map[string]string{},
	}

	mockExec := func(argv0 string, argv []string, envv []string) error {
		return nil
	}

	err := resolveAndExecute(t, nil, selected, "", mockExec)
	if err == nil {
		t.Error("expected error when pane_id is empty and template references it")
	}
}

func TestEffectiveLaunchMode(t *testing.T) {
	tests := []struct {
		name string
		it   item.Item
		want launchMode
	}{
		{"dir minimal", item.Item{MatchType: "dir"}, launchModeSessionWindow},
		{"root minimal", item.Item{MatchType: "root"}, launchModeShell},
		{"session minimal", item.Item{MatchType: "session"}, launchModeShell},
		{"root launch path", item.Item{MatchType: "root", LaunchPath: "/tmp"}, launchModeSessionWindow},
		{"root launch path cmd", item.Item{MatchType: "root", LaunchPathCmd: "pwd"}, launchModeSessionWindow},
		{"explicit shell wins", item.Item{MatchType: "dir", LaunchMode: "shell"}, launchModeShell},
		{"explicit session wins", item.Item{MatchType: "root", LaunchMode: "session-window"}, launchModeSessionWindow},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := effectiveLaunchMode(tt.it); got != tt.want {
				t.Errorf("effectiveLaunchMode() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestResolveEffectiveLaunchPath(t *testing.T) {
	dir := t.TempDir()
	got, ok, err := resolveEffectiveLaunchPath(item.Item{MatchType: "dir"}, map[string]string{"path": dir}, launchModeSessionWindow, 0, "")
	if err != nil {
		t.Fatalf("dir path: %v", err)
	}
	if !ok || got != filepath.Clean(dir) {
		t.Fatalf("dir path = %q, %v; want %q, true", got, ok, filepath.Clean(dir))
	}

	oldGetwd := getwd
	getwd = func() (string, error) { return dir, nil }
	t.Cleanup(func() { getwd = oldGetwd })
	got, ok, err = resolveEffectiveLaunchPath(item.Item{MatchType: "root"}, map[string]string{}, launchModeSessionWindow, 0, "")
	if err != nil {
		t.Fatalf("cwd fallback: %v", err)
	}
	if !ok || got != filepath.Clean(dir) {
		t.Fatalf("cwd fallback = %q, %v; want %q, true", got, ok, filepath.Clean(dir))
	}

	got, ok, err = resolveEffectiveLaunchPath(item.Item{MatchType: "root"}, map[string]string{}, launchModeShell, 0, "")
	if err != nil {
		t.Fatalf("no path shell: %v", err)
	}
	if ok || got != "" {
		t.Fatalf("no path shell = %q, %v; want empty, false", got, ok)
	}
}

func TestResolveLaunchPath_ExpandsSafely(t *testing.T) {
	home := t.TempDir()
	project := filepath.Join(home, "Code", "proj")
	if err := os.MkdirAll(project, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", home)
	t.Setenv("PROJECT_NAME", "proj")

	got, err := resolveLaunchPath("~/Code/$PROJECT_NAME", nil)
	if err != nil {
		t.Fatalf("~/$VAR: %v", err)
	}
	if got != project {
		t.Errorf("got %q, want %q", got, project)
	}

	got, err = resolveLaunchPath("${HOME}/Code/{{.name}}", map[string]string{"name": "proj"})
	if err != nil {
		t.Fatalf("${VAR}+template: %v", err)
	}
	if got != project {
		t.Errorf("got %q, want %q", got, project)
	}
}

func TestResolveLaunchPath_MissingEnvVarErrors(t *testing.T) {
	name := "CMDK_TEST_MISSING_LAUNCH_PATH_VAR"
	old, hadOld := os.LookupEnv(name)
	if err := os.Unsetenv(name); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if hadOld {
			_ = os.Setenv(name, old)
			return
		}
		_ = os.Unsetenv(name)
	})

	for _, templateText := range []string{"$" + name + "/project", "${" + name + "}/project"} {
		_, err := resolveLaunchPath(templateText, nil)
		if err == nil {
			t.Fatalf("resolveLaunchPath(%q) expected error", templateText)
		}
		if !strings.Contains(err.Error(), "launch_path expands") || !strings.Contains(err.Error(), name) || !strings.Contains(err.Error(), "not set") {
			t.Fatalf("resolveLaunchPath(%q) error = %v, want missing env var", templateText, err)
		}
	}
}

func TestResolveLaunchPath_DoesNotExpandTemplateData(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "$LITERAL")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("LITERAL", "wrong")

	got, err := resolveLaunchPath("{{.path}}", map[string]string{"path": dir})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != dir {
		t.Errorf("got %q, want literal template-derived path %q", got, dir)
	}
}

func TestResolveLaunchPath_Errors(t *testing.T) {
	if _, err := resolveLaunchPath("{{.empty}}", map[string]string{"empty": ""}); err == nil || !strings.Contains(err.Error(), "launch_path rendered empty") {
		t.Fatalf("empty error = %v", err)
	}
	missing := filepath.Join(t.TempDir(), "missing")
	if _, err := resolveLaunchPath(missing, nil); err == nil || !strings.Contains(err.Error(), "does not exist") {
		t.Fatalf("missing error = %v", err)
	}
	file := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(file, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := resolveLaunchPath(file, nil); err == nil || !strings.Contains(err.Error(), "not a directory") {
		t.Fatalf("file error = %v", err)
	}
}

func TestResolveLaunchPathCmd(t *testing.T) {
	dir := t.TempDir()
	got, err := resolveLaunchPathCmd("printf '%s\\n' {{sq .target}}", map[string]string{"target": dir}, time.Second, "")
	if err != nil {
		t.Fatalf("success: %v", err)
	}
	if got != filepath.Clean(dir) {
		t.Errorf("got %q, want %q", got, filepath.Clean(dir))
	}

	got, err = resolveLaunchPathCmd("[ -z \"$CMDK_LAUNCH_PATH\" ] || exit 9; printf '%s\\n' \"$CMDK_TARGET\"", map[string]string{"target": dir}, time.Second, "")
	if err != nil {
		t.Fatalf("env success: %v", err)
	}
	if got != filepath.Clean(dir) {
		t.Errorf("env got %q, want %q", got, filepath.Clean(dir))
	}

	tests := []struct {
		name string
		cmd  string
		want string
		kind cmdrun.Kind
		exit int
	}{
		{"empty", "printf ''", "cannot be empty", cmdrun.KindOutput, 0},
		{"multiple", "printf '/tmp\\n/tmp\\n'", "exactly one line", cmdrun.KindOutput, -1},
		{"relative", "printf 'relative\\n'", "absolute path", cmdrun.KindOutput, 0},
		{"nonzero", "printf 'bad news' >&2; exit 7", "bad news", cmdrun.KindExit, 7},
		{"nonzero-silent", "exit 5", "exit status 5", cmdrun.KindExit, 5},
		{"control-chars", "printf '/tmp/bad\\aname\\n'", "control characters", cmdrun.KindOutput, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := resolveLaunchPathCmd(tt.cmd, nil, time.Second, "")
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error = %v, want substring %q", err, tt.want)
			}
			var cmdErr *cmdrun.CommandError
			if !errors.As(err, &cmdErr) {
				t.Fatalf("error = %v, want *cmdrun.CommandError", err)
			}
			if cmdErr.Kind != tt.kind {
				t.Errorf("Kind = %q, want %q", cmdErr.Kind, tt.kind)
			}
			if cmdErr.ExitCode != tt.exit {
				t.Errorf("ExitCode = %d, want %d", cmdErr.ExitCode, tt.exit)
			}
		})
	}
}

func TestResolveLaunchPathCmd_SilentFailureHasEmptyRawStreams(t *testing.T) {
	_, err := resolveLaunchPathCmd("exit 5", nil, time.Second, "")
	var cmdErr *cmdrun.CommandError
	if !errors.As(err, &cmdErr) {
		t.Fatalf("error = %v, want *cmdrun.CommandError", err)
	}
	if cmdErr.Stdout != "" || cmdErr.Stderr != "" {
		t.Errorf("Stdout/Stderr = %q/%q, want empty raw streams", cmdErr.Stdout, cmdErr.Stderr)
	}
	if strings.Contains(err.Error(), "stderr:") {
		t.Errorf("Error() = %q, want no dangling stream labels", err.Error())
	}
}

func TestRenderWindowName(t *testing.T) {
	data := map[string]string{"launch_path": "/tmp/project", "launch_basename": "project"}
	got, err := renderWindowName(item.Item{}, data)
	if err != nil {
		t.Fatalf("default: %v", err)
	}
	if got != "project" {
		t.Errorf("default = %q, want project", got)
	}
	got, err = renderWindowName(item.Item{WindowName: "pi:{{.launch_basename}}"}, data)
	if err != nil {
		t.Fatalf("template: %v", err)
	}
	if got != "pi:project" {
		t.Errorf("template = %q, want pi:project", got)
	}
	if _, err := renderWindowName(item.Item{WindowName: "{{.missing}}"}, data); err == nil {
		t.Fatal("expected missing key error")
	}
	if _, err := renderWindowName(item.Item{WindowName: ""}, map[string]string{"launch_basename": ""}); err == nil || !strings.Contains(err.Error(), "empty") {
		t.Fatalf("empty error = %v", err)
	}
	if _, err := renderWindowName(item.Item{WindowName: "bad\nname"}, data); err == nil || !strings.Contains(err.Error(), "control") {
		t.Fatalf("control error = %v", err)
	}
}

func TestLaunchExecute_ShellLaunchPathChdirsAndSetsEnv(t *testing.T) {
	dir := t.TempDir()
	oldwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(oldwd) })

	selected := item.Item{Cmd: "echo {{.launch_basename}}", MatchType: "root", LaunchMode: "shell", LaunchPath: dir}
	var cwd string
	var argv []string
	var envv []string
	mockExec := func(argv0 string, gotArgv []string, gotEnvv []string) error {
		var err error
		cwd, err = os.Getwd()
		if err != nil {
			return err
		}
		argv = gotArgv
		envv = gotEnvv
		return nil
	}

	if err := resolveAndExecute(t, nil, selected, "%9", mockExec); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Getwd returns the symlink-resolved directory (on Darwin, t.TempDir()
	// lives under the /var -> /private/var symlink), so canonicalize the
	// expectation the same way.
	wantCwd, err := filepath.EvalSymlinks(filepath.Clean(dir))
	if err != nil {
		t.Fatal(err)
	}
	if cwd != wantCwd {
		t.Errorf("cwd = %q, want %q", cwd, wantCwd)
	}
	if argv[2] != "echo "+filepath.Base(dir) {
		t.Errorf("rendered = %q", argv[2])
	}
	envMap := envSliceToMap(envv)
	if envMap["CMDK_LAUNCH_PATH"] != filepath.Clean(dir) {
		t.Errorf("CMDK_LAUNCH_PATH = %q", envMap["CMDK_LAUNCH_PATH"])
	}
	if envMap["CMDK_LAUNCH_BASENAME"] != filepath.Base(dir) {
		t.Errorf("CMDK_LAUNCH_BASENAME = %q", envMap["CMDK_LAUNCH_BASENAME"])
	}
	if envMap["CMDK_PANE_ID"] != "%9" {
		t.Errorf("CMDK_PANE_ID = %q", envMap["CMDK_PANE_ID"])
	}
}

func TestLaunchExecute_ShellWithoutLaunchPathKeepsCwdAndOmitsLaunchEnv(t *testing.T) {
	oldwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	selected := item.Item{Cmd: "echo ok", MatchType: "root"}
	var cwd string
	var envv []string
	mockExec := func(argv0 string, argv []string, gotEnvv []string) error {
		var err error
		cwd, err = os.Getwd()
		envv = gotEnvv
		return err
	}
	if err := resolveAndExecute(t, nil, selected, "", mockExec); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cwd != oldwd {
		t.Errorf("cwd = %q, want %q", cwd, oldwd)
	}
	envMap := envSliceToMap(envv)
	if _, ok := envMap["CMDK_LAUNCH_PATH"]; ok {
		t.Error("CMDK_LAUNCH_PATH should be omitted")
	}
}

func TestLaunchExecute_SessionWindowNewShellCreatesInteractiveWindow(t *testing.T) {
	dir := t.TempDir()
	oldResolve := resolveSessionPlan
	oldCreate := createResolvedSessionWindow
	t.Cleanup(func() {
		resolveSessionPlan = oldResolve
		createResolvedSessionWindow = oldCreate
	})

	resolveSessionPlan = func(_ context.Context, path string, _ time.Duration) (resolver.Plan, error) {
		return resolver.Plan{SessionKind: resolver.KindDirectory, SessionKey: path}, nil
	}

	var gotLaunchPath string
	var gotOpts tmux.SessionWindowOptions
	createResolvedSessionWindow = func(_ context.Context, _ resolver.Plan, launchPath string, opts tmux.SessionWindowOptions) (tmux.SessionWindowResult, error) {
		gotLaunchPath = launchPath
		gotOpts = opts
		return tmux.SessionWindowResult{}, nil
	}

	selected := item.Item{MatchType: "dir", LaunchMode: "session-window", NewShell: true}
	accumulated := []item.Item{{Type: "dir", Data: map[string]string{"path": dir}}}
	if err := resolveAndExecute(t, accumulated, selected, "", func(string, []string, []string) error {
		t.Fatal("execFn should not be called for session-window mode")
		return nil
	}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotLaunchPath != filepath.Clean(dir) {
		t.Errorf("launchPath = %q, want %q", gotLaunchPath, filepath.Clean(dir))
	}
	if !gotOpts.NewShell {
		t.Error("NewShell = false, want true")
	}
	if len(gotOpts.Command) != 0 {
		t.Errorf("Command = %#v, want empty", gotOpts.Command)
	}
	if gotOpts.Name != filepath.Base(dir) {
		t.Errorf("Name = %q, want basename", gotOpts.Name)
	}
}

func TestLaunchExecute_SessionWindowCreatesManagedWindow(t *testing.T) {
	dir := t.TempDir()
	oldResolve := resolveSessionPlan
	oldCreate := createResolvedSessionWindow
	t.Cleanup(func() {
		resolveSessionPlan = oldResolve
		createResolvedSessionWindow = oldCreate
	})

	var resolvedPath string
	resolveSessionPlan = func(_ context.Context, path string, _ time.Duration) (resolver.Plan, error) {
		resolvedPath = path
		return resolver.Plan{SessionKind: resolver.KindDirectory, SessionKey: path}, nil
	}

	var gotPlan resolver.Plan
	var gotLaunchPath string
	var gotOpts tmux.SessionWindowOptions
	createResolvedSessionWindow = func(_ context.Context, plan resolver.Plan, launchPath string, opts tmux.SessionWindowOptions) (tmux.SessionWindowResult, error) {
		gotPlan = plan
		gotLaunchPath = launchPath
		gotOpts = opts
		return tmux.SessionWindowResult{
			SessionID:  "$5",
			SessionKey: plan.SessionKey,
			WindowID:   "@18",
			WindowName: "x-result",
			PaneID:     "%51",
		}, nil
	}

	selected := item.Item{Cmd: "echo {{.launch_path}}", MatchType: "dir", WindowName: "x-{{.launch_basename}}"}
	accumulated := []item.Item{{Type: "dir", Data: map[string]string{"path": dir}}}
	launch, _, err := ResolveLaunch(accumulated, selected, "", config.DefaultConfig())
	if err != nil {
		t.Fatalf("ResolveLaunch: %v", err)
	}
	target := tmux.ClientTarget{Name: "/dev/pts/4", PaneID: "%17"}
	result, err := launch.ForClient(target).ExecuteWithResult(func(string, []string, []string) error {
		t.Fatal("execFn should not be called for session-window mode")
		return nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resolvedPath != filepath.Clean(dir) || gotLaunchPath != filepath.Clean(dir) || gotPlan.SessionKey != filepath.Clean(dir) {
		t.Fatalf("resolvedPath/launchPath/sessionKey = %q/%q/%q, want %q", resolvedPath, gotLaunchPath, gotPlan.SessionKey, filepath.Clean(dir))
	}
	if gotOpts.Name != "x-"+filepath.Base(dir) {
		t.Errorf("Name = %q", gotOpts.Name)
	}
	wantCmd := []string{"sh", "-lc", "echo " + filepath.Clean(dir)}
	if !slices.Equal(gotOpts.Command, wantCmd) {
		t.Errorf("Command = %#v, want %#v", gotOpts.Command, wantCmd)
	}
	if !gotOpts.Switch {
		t.Error("Switch = false, want true")
	}
	if gotOpts.TargetClient != target {
		t.Errorf("TargetClient = %#v, want %#v", gotOpts.TargetClient, target)
	}
	if gotOpts.MaxNameLength != 20 {
		t.Errorf("MaxNameLength = %d, want default 20", gotOpts.MaxNameLength)
	}
	wantResult := LaunchResult{
		LaunchPath: filepath.Clean(dir),
		SessionID:  "$5",
		SessionKey: filepath.Clean(dir),
		WindowID:   "@18",
		WindowName: "x-result",
		PaneID:     "%51",
	}
	if result != wantResult {
		t.Errorf("result = %#v, want %#v", result, wantResult)
	}
}

func TestExecuteWithResultRejectsInvalidUTF8BeforeSessionResolution(t *testing.T) {
	oldResolve := resolveSessionPlan
	t.Cleanup(func() { resolveSessionPlan = oldResolve })
	resolveSessionPlan = func(context.Context, string, time.Duration) (resolver.Plan, error) {
		t.Fatal("session resolution ran for a non-UTF-8 JSON field")
		return resolver.Plan{}, nil
	}

	tests := []struct {
		name       string
		launchPath string
		windowName string
		want       string
	}{
		{name: "launch path", launchPath: "/tmp/" + string([]byte{0xff}), windowName: "main", want: "launch_path"},
		{name: "window name", launchPath: "/tmp", windowName: string([]byte{0xff}), want: "window_name"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			launch := Launch{mode: launchModeSessionWindow, path: test.launchPath, windowName: test.windowName}
			_, err := launch.ExecuteWithResult(func(string, []string, []string) error { return nil })
			if err == nil || !strings.Contains(err.Error(), test.want) || !strings.Contains(err.Error(), "UTF-8") {
				t.Fatalf("error = %v, want %s UTF-8 validation", err, test.want)
			}
		})
	}
}

func TestExecuteWithResultRejectsInvalidUTF8SessionKeyBeforeTmux(t *testing.T) {
	oldResolve := resolveSessionPlan
	oldCreate := createResolvedSessionWindow
	t.Cleanup(func() {
		resolveSessionPlan = oldResolve
		createResolvedSessionWindow = oldCreate
	})
	resolveSessionPlan = func(context.Context, string, time.Duration) (resolver.Plan, error) {
		return resolver.Plan{SessionKey: "/tmp/" + string([]byte{0xff})}, nil
	}
	createResolvedSessionWindow = func(context.Context, resolver.Plan, string, tmux.SessionWindowOptions) (tmux.SessionWindowResult, error) {
		t.Fatal("tmux mutation ran for a non-UTF-8 session key")
		return tmux.SessionWindowResult{}, nil
	}

	launch := Launch{mode: launchModeSessionWindow, path: t.TempDir(), windowName: "main"}
	_, err := launch.ExecuteWithResult(func(string, []string, []string) error { return nil })
	if err == nil || !strings.Contains(err.Error(), "session_key") || !strings.Contains(err.Error(), "UTF-8") {
		t.Fatalf("error = %v, want session_key UTF-8 validation", err)
	}
}

func TestResolveLaunch_LaunchPathCmdFailureIncludesStdoutAndStderr(t *testing.T) {
	selected := item.Item{
		Cmd:           "true",
		MatchType:     "root",
		LaunchPathCmd: "sh -c 'echo out; echo err >&2; exit 23'",
	}

	_, _, err := ResolveLaunch(nil, selected, "", config.DefaultConfig())
	var cmdErr *cmdrun.CommandError
	if !errors.As(err, &cmdErr) {
		t.Fatalf("error = %v, want *cmdrun.CommandError", err)
	}
	if cmdErr.Kind != cmdrun.KindExit {
		t.Errorf("Kind = %q, want %q", cmdErr.Kind, cmdrun.KindExit)
	}
	if cmdErr.ExitCode != 23 {
		t.Errorf("ExitCode = %d, want 23", cmdErr.ExitCode)
	}
	if cmdErr.Command != selected.LaunchPathCmd {
		t.Errorf("Command = %q, want %q", cmdErr.Command, selected.LaunchPathCmd)
	}
	if !strings.Contains(cmdErr.Stdout, "out") {
		t.Errorf("Stdout = %q, want to contain %q", cmdErr.Stdout, "out")
	}
	if !strings.Contains(cmdErr.Stderr, "err") {
		t.Errorf("Stderr = %q, want to contain %q", cmdErr.Stderr, "err")
	}
	if !strings.Contains(err.Error(), "out") || !strings.Contains(err.Error(), "err") {
		t.Errorf("Error() = %q, want to contain both streams", err.Error())
	}
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) || exitErr.ExitCode() != 23 {
		t.Errorf("errors.As(*exec.ExitError) = %v (%v), want exit code 23", exitErr, err)
	}
}

func TestResolveLaunch_RelativeOutputWrapsAsCommandError(t *testing.T) {
	selected := item.Item{
		Cmd:           "true",
		MatchType:     "root",
		LaunchPathCmd: "printf 'noise' >&2; printf 'relative\\n'",
	}

	_, _, err := ResolveLaunch(nil, selected, "", config.DefaultConfig())
	var cmdErr *cmdrun.CommandError
	if !errors.As(err, &cmdErr) {
		t.Fatalf("error = %v, want *cmdrun.CommandError", err)
	}
	if cmdErr.Kind != cmdrun.KindOutput {
		t.Errorf("Kind = %q, want %q", cmdErr.Kind, cmdrun.KindOutput)
	}
	if cmdErr.ExitCode != 0 {
		t.Errorf("ExitCode = %d, want 0 for a succeeded command with invalid output", cmdErr.ExitCode)
	}
	if cmdErr.Command == "" {
		t.Error("Command is empty, want the rendered command")
	}
	if !strings.Contains(cmdErr.Stdout, "relative") {
		t.Errorf("Stdout = %q, want captured output", cmdErr.Stdout)
	}
	if !strings.Contains(cmdErr.Stderr, "noise") {
		t.Errorf("Stderr = %q, want captured stderr", cmdErr.Stderr)
	}
	if !strings.Contains(err.Error(), "absolute path") {
		t.Errorf("Error() = %q, want absolute path violation", err.Error())
	}
}

func TestResolveLaunch_CmdTemplateFailsAtResolveTime(t *testing.T) {
	dir := t.TempDir()

	shell := item.Item{Cmd: "echo {{.missing}}", MatchType: "root"}
	if _, _, err := ResolveLaunch(nil, shell, "", config.DefaultConfig()); err == nil || !strings.Contains(err.Error(), "cmd template") {
		t.Fatalf("shell error = %v, want cmd template", err)
	}

	sessionWindow := item.Item{Cmd: "echo {{.missing}}", MatchType: "dir"}
	accumulated := []item.Item{{Type: "dir", Data: map[string]string{"path": dir}}}
	if _, _, err := ResolveLaunch(accumulated, sessionWindow, "", config.DefaultConfig()); err == nil || !strings.Contains(err.Error(), "cmd template") {
		t.Fatalf("session-window error = %v, want cmd template", err)
	}
}

func TestResolveLaunch_WindowNameTemplateFailsAtResolveTime(t *testing.T) {
	dir := t.TempDir()
	selected := item.Item{Cmd: "true", MatchType: "dir", WindowName: "{{.missing}}"}
	accumulated := []item.Item{{Type: "dir", Data: map[string]string{"path": dir}}}
	if _, _, err := ResolveLaunch(accumulated, selected, "", config.DefaultConfig()); err == nil || !strings.Contains(err.Error(), "window_name template") {
		t.Fatalf("error = %v, want window_name template", err)
	}
}

func TestResolveLaunch_FailureDataIncludesResolvedLaunchPath(t *testing.T) {
	dir := t.TempDir()
	selected := item.Item{Cmd: "true", MatchType: "dir", WindowName: "{{.missing}}"}
	accumulated := []item.Item{{Type: "dir", Data: map[string]string{"path": dir}}}

	_, data, err := ResolveLaunch(accumulated, selected, "", config.DefaultConfig())
	if err == nil {
		t.Fatal("expected window_name template error")
	}
	if data["launch_path"] != filepath.Clean(dir) {
		t.Errorf("data[launch_path] = %q, want %q (failing template saw it)", data["launch_path"], filepath.Clean(dir))
	}
	if data["launch_basename"] != filepath.Base(dir) {
		t.Errorf("data[launch_basename] = %q, want %q", data["launch_basename"], filepath.Base(dir))
	}
}

func TestValidateExistingDirectory_RejectsControlCharacters(t *testing.T) {
	if _, err := validateExistingDirectory("launch_path", "/tmp/bad\aname"); err == nil || !strings.Contains(err.Error(), "control characters") {
		t.Fatalf("error = %v, want control characters", err)
	}
}

func envSliceToMap(envs []string) map[string]string {
	m := make(map[string]string)
	for _, e := range envs {
		k, v, ok := strings.Cut(e, "=")
		if ok {
			m[k] = v
		}
	}
	return m
}

func TestLaunchExecute_SessionWindowThreadsConfiguredWindowNameMaxLength(t *testing.T) {
	dir := t.TempDir()
	oldResolve := resolveSessionPlan
	oldCreate := createResolvedSessionWindow
	t.Cleanup(func() {
		resolveSessionPlan = oldResolve
		createResolvedSessionWindow = oldCreate
	})

	resolveSessionPlan = func(_ context.Context, path string, _ time.Duration) (resolver.Plan, error) {
		return resolver.Plan{SessionKind: resolver.KindDirectory, SessionKey: path}, nil
	}
	var gotOpts tmux.SessionWindowOptions
	createResolvedSessionWindow = func(_ context.Context, _ resolver.Plan, _ string, opts tmux.SessionWindowOptions) (tmux.SessionWindowResult, error) {
		gotOpts = opts
		return tmux.SessionWindowResult{}, nil
	}

	selected := item.Item{MatchType: "dir", LaunchMode: "session-window", NewShell: true}
	accumulated := []item.Item{{Type: "dir", Data: map[string]string{"path": dir}}}
	for _, max := range []int{7, 0} {
		cfg := config.DefaultConfig()
		cfg.Behavior.WindowNameMaxLength = max
		launch, _, err := ResolveLaunch(accumulated, selected, "", cfg)
		if err != nil {
			t.Fatalf("ResolveLaunch(max=%d): %v", max, err)
		}
		if err := launch.Execute(func(string, []string, []string) error {
			t.Fatal("execFn should not be called for session-window mode")
			return nil
		}); err != nil {
			t.Fatalf("Execute(max=%d): %v", max, err)
		}
		if gotOpts.MaxNameLength != max {
			t.Errorf("MaxNameLength = %d, want configured %d", gotOpts.MaxNameLength, max)
		}
	}
}
