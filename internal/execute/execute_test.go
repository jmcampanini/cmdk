package execute

import (
	"fmt"
	"strings"
	"testing"

	"github.com/jmcampanini/cmdk/internal/item"
)

func TestRenderCmd(t *testing.T) {
	tmpl := "tmux switch-client -t '{{.session}}:{{.window_index}}'"
	data := map[string]string{"session": "main", "window_index": "2"}

	got, err := RenderCmd(tmpl, data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "tmux switch-client -t 'main:2'" {
		t.Errorf("got %q", got)
	}
}

func TestRenderCmd_MissingKey(t *testing.T) {
	tmpl := "tmux switch-client -t '{{.session}}:{{.window_index}}'"
	data := map[string]string{"session": "main"}

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
		Cmd:  "tmux switch-client -t '{{.session}}:{{.window_index}}'",
		Data: map[string]string{"session": "main", "window_index": "2"},
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
	if capturedArgv[2] != "tmux switch-client -t 'main:2'" {
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
		{Data: map[string]string{"path": "/home/user", "session": "main"}},
	}
	envs := BuildCMDKEnvVars(items, "%1")
	envMap := envSliceToMap(envs)

	if envMap["CMDK_PATH"] != "/home/user" {
		t.Errorf("CMDK_PATH = %q, want /home/user", envMap["CMDK_PATH"])
	}
	if envMap["CMDK_SESSION"] != "main" {
		t.Errorf("CMDK_SESSION = %q, want main", envMap["CMDK_SESSION"])
	}
	if envMap["CMDK_PANE_ID"] != "%1" {
		t.Errorf("CMDK_PANE_ID = %q, want %%1", envMap["CMDK_PANE_ID"])
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
		{Data: map[string]string{"session": "dev"}},
		{Data: map[string]string{"path": "/projects"}},
	}
	envs := BuildCMDKEnvVars(items, "%5")
	envMap := envSliceToMap(envs)

	if envMap["CMDK_SESSION"] != "dev" {
		t.Errorf("CMDK_SESSION = %q, want dev", envMap["CMDK_SESSION"])
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
		Data: map[string]string{"session": "main"},
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
	if envMap["CMDK_SESSION"] != "main" {
		t.Errorf("CMDK_SESSION = %q, want main", envMap["CMDK_SESSION"])
	}
	if envMap["CMDK_PANE_ID"] != "%3" {
		t.Errorf("CMDK_PANE_ID = %q, want %%3", envMap["CMDK_PANE_ID"])
	}
}

func TestRun_StripsExistingCMDKVars(t *testing.T) {
	t.Setenv("CMDK_STALE", "leftover")

	selected := item.Item{
		Cmd:  "echo hi",
		Data: map[string]string{"session": "main"},
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
	if envMap["CMDK_SESSION"] != "main" {
		t.Errorf("CMDK_SESSION = %q, want main", envMap["CMDK_SESSION"])
	}
	if envMap["CMDK_PANE_ID"] != "%1" {
		t.Errorf("CMDK_PANE_ID = %q, want %%1", envMap["CMDK_PANE_ID"])
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
