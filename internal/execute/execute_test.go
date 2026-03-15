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

	err := Run(nil, selected, mockExec)
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

	err := Run(accumulated, selected, mockExec)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if capturedArgv[2] != "echo /tmp test" {
		t.Errorf("rendered cmd = %q, want %q", capturedArgv[2], "echo /tmp test")
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

	err := Run(nil, selected, mockExec)
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

	err := Run(nil, selected, mockExec)
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

	err := Run(nil, selected, mockExec)
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
