package execute

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/jmcampanini/cmdk/internal/config"
	"github.com/jmcampanini/cmdk/internal/item"
	resolver "github.com/jmcampanini/cmdk/internal/session"
	"github.com/jmcampanini/cmdk/internal/tmux"
)

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

func TestRun_PushesSelectedAndCallsExecFn(t *testing.T) {
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

	err := Run(nil, selected, "%1", mockExec)
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

func TestRun_SelectedDataAvailableInTemplate(t *testing.T) {
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

	err := Run(accumulated, selected, "%1", mockExec)
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

func TestRun_ExecFnError(t *testing.T) {
	selected := item.Item{
		Cmd:  "echo hello",
		Data: map[string]string{},
	}

	mockExec := func(argv0 string, argv []string, envv []string) error {
		return fmt.Errorf("exec failed")
	}

	err := Run(nil, selected, "%1", mockExec)
	if err == nil || err.Error() != "exec failed" {
		t.Errorf("expected exec failed error, got: %v", err)
	}
}

func TestRun_MissingKeyDoesNotCallExecFn(t *testing.T) {
	selected := item.Item{
		Cmd:  "echo {{.missing}}",
		Data: map[string]string{},
	}

	called := false
	mockExec := func(argv0 string, argv []string, envv []string) error {
		called = true
		return nil
	}

	err := Run(nil, selected, "%1", mockExec)
	if err == nil {
		t.Error("expected error for missing template key")
	}
	if called {
		t.Error("execFn should not be called when template rendering fails")
	}
}

func TestRun_EmptyCmd(t *testing.T) {
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

	err := Run(nil, selected, "%1", mockExec)
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

func TestRun_EnvVarsContainCMDK(t *testing.T) {
	selected := item.Item{
		Cmd:  "echo hi",
		Data: map[string]string{"session_name": "main"},
	}

	var capturedEnvv []string
	mockExec := func(argv0 string, argv []string, envv []string) error {
		capturedEnvv = envv
		return nil
	}

	err := Run(nil, selected, "%3", mockExec)
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

func TestRun_StripsExistingCMDKVars(t *testing.T) {
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

	err := Run(nil, selected, "%1", mockExec)
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

func TestRun_PaneIDAvailableInTemplate(t *testing.T) {
	selected := item.Item{
		Cmd:  "tmux split-window -t {{.pane_id}}",
		Data: map[string]string{},
	}

	var capturedArgv []string
	mockExec := func(argv0 string, argv []string, envv []string) error {
		capturedArgv = argv
		return nil
	}

	err := Run(nil, selected, "%5", mockExec)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if capturedArgv[2] != "tmux split-window -t %5" {
		t.Errorf("rendered cmd = %q, want %q", capturedArgv[2], "tmux split-window -t %5")
	}
}

func TestRun_PaneIDWithSq(t *testing.T) {
	selected := item.Item{
		Cmd:  "tmux split-window -t {{sq .pane_id}} -c {{sq .path}}",
		Data: map[string]string{"path": "/home/user"},
	}

	var capturedArgv []string
	mockExec := func(argv0 string, argv []string, envv []string) error {
		capturedArgv = argv
		return nil
	}

	err := Run(nil, selected, "%5", mockExec)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := "tmux split-window -t '%5' -c '/home/user'"
	if capturedArgv[2] != want {
		t.Errorf("rendered cmd = %q, want %q", capturedArgv[2], want)
	}
}

func TestRun_EmptyPaneIDNotInTemplateData(t *testing.T) {
	selected := item.Item{
		Cmd:  "echo {{.pane_id}}",
		Data: map[string]string{},
	}

	mockExec := func(argv0 string, argv []string, envv []string) error {
		return nil
	}

	err := Run(nil, selected, "", mockExec)
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
	}{
		{"empty", "printf ''", "cannot be empty"},
		{"multiple", "printf '/tmp\\n/tmp\\n'", "exactly one line"},
		{"relative", "printf 'relative\\n'", "absolute path"},
		{"nonzero", "printf 'bad news' >&2; exit 7", "bad news"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := resolveLaunchPathCmd(tt.cmd, nil, time.Second, "")
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error = %v, want substring %q", err, tt.want)
			}
		})
	}
}

func TestResolveLaunchPathCmd_OversizedStdout(t *testing.T) {
	cmd := fmt.Sprintf("i=0; while [ $i -le %d ]; do printf x; i=$((i+1)); done", launchPathCmdMaxStdoutBytes)
	_, err := resolveLaunchPathCmd(cmd, nil, time.Second, "")
	if err == nil || !strings.Contains(err.Error(), "output exceeds") {
		t.Fatalf("error = %v, want output limit", err)
	}
}

func TestResolveLaunchPathCmd_Timeout(t *testing.T) {
	_, err := resolveLaunchPathCmd("sleep 1; printf /tmp", nil, 10*time.Millisecond, "")
	if err == nil || !strings.Contains(err.Error(), "timed out") {
		t.Fatalf("error = %v, want timeout", err)
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

func TestRun_ShellLaunchPathChdirsAndSetsEnv(t *testing.T) {
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

	if err := RunWithConfig(nil, selected, "%9", config.DefaultConfig(), mockExec); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cwd != filepath.Clean(dir) {
		t.Errorf("cwd = %q, want %q", cwd, filepath.Clean(dir))
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

func TestRun_ShellWithoutLaunchPathKeepsCwdAndOmitsLaunchEnv(t *testing.T) {
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
	if err := Run(nil, selected, "", mockExec); err != nil {
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

func TestRun_SessionWindowNewShellCreatesInteractiveWindow(t *testing.T) {
	dir := t.TempDir()
	oldResolve := resolveSessionPlan
	oldCreate := createResolvedSessionWindow
	t.Cleanup(func() {
		resolveSessionPlan = oldResolve
		createResolvedSessionWindow = oldCreate
	})

	resolveSessionPlan = func(_ context.Context, path string, _ resolver.DisplayOptions) (resolver.Plan, error) {
		return resolver.Plan{
			SessionKind:            resolver.KindDirectory,
			SessionKey:             path,
			SessionDisplay:         path,
			LaunchPath:             path,
			PlannedTmuxSessionName: "planned",
			PlannedTmuxWindowName:  filepath.Base(path),
		}, nil
	}

	var gotOpts tmux.SessionWindowOptions
	createResolvedSessionWindow = func(_ context.Context, _ resolver.Plan, opts tmux.SessionWindowOptions) error {
		gotOpts = opts
		return nil
	}

	selected := item.Item{MatchType: "dir", LaunchMode: "session-window", NewShell: true}
	accumulated := []item.Item{{Type: "dir", Data: map[string]string{"path": dir}}}
	if err := RunWithConfig(accumulated, selected, "", config.DefaultConfig(), func(string, []string, []string) error {
		t.Fatal("execFn should not be called for session-window mode")
		return nil
	}); err != nil {
		t.Fatalf("unexpected error: %v", err)
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

func TestRun_SessionWindowCreatesManagedWindow(t *testing.T) {
	dir := t.TempDir()
	oldResolve := resolveSessionPlan
	oldCreate := createResolvedSessionWindow
	t.Cleanup(func() {
		resolveSessionPlan = oldResolve
		createResolvedSessionWindow = oldCreate
	})

	var resolvedPath string
	resolveSessionPlan = func(_ context.Context, path string, _ resolver.DisplayOptions) (resolver.Plan, error) {
		resolvedPath = path
		return resolver.Plan{
			SessionKind:            resolver.KindDirectory,
			SessionKey:             path,
			SessionDisplay:         path,
			LaunchPath:             path,
			PlannedTmuxSessionName: "planned",
			PlannedTmuxWindowName:  filepath.Base(path),
		}, nil
	}

	var gotPlan resolver.Plan
	var gotOpts tmux.SessionWindowOptions
	createResolvedSessionWindow = func(_ context.Context, plan resolver.Plan, opts tmux.SessionWindowOptions) error {
		gotPlan = plan
		gotOpts = opts
		return nil
	}

	selected := item.Item{Cmd: "echo {{.launch_path}}", MatchType: "dir", WindowName: "x-{{.launch_basename}}"}
	accumulated := []item.Item{{Type: "dir", Data: map[string]string{"path": dir}}}
	if err := RunWithConfig(accumulated, selected, "", config.DefaultConfig(), func(string, []string, []string) error {
		t.Fatal("execFn should not be called for session-window mode")
		return nil
	}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resolvedPath != filepath.Clean(dir) || gotPlan.LaunchPath != filepath.Clean(dir) {
		t.Fatalf("resolvedPath/gotPlan = %q/%q, want %q", resolvedPath, gotPlan.LaunchPath, filepath.Clean(dir))
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
