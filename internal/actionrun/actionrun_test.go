package actionrun

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jmcampanini/cmdk/internal/config"
	"github.com/jmcampanini/cmdk/internal/execute"
	"github.com/jmcampanini/cmdk/internal/item"
)

func TestPrepareExactActionLookup(t *testing.T) {
	cfg := config.Config{Actions: []config.Action{
		{Name: "build", Matches: "root", LaunchMode: config.LaunchModeSessionWindow, Cmd: "lower"},
		{Name: "Build", Matches: "root", LaunchMode: config.LaunchModeSessionWindow, Cmd: "upper"},
	}}

	prepared, err := prepare(cfg, "Build", "", nil)
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	if prepared.Selected.Cmd != "upper" {
		t.Fatalf("selected action = %#v, want exact-case Build", prepared.Selected)
	}

	_, err = prepare(cfg, "BUILD", "", nil)
	assertErrorContains(t, err, `configured action "BUILD" not found`)
}

func TestPrepareRejectsDuplicateExactNames(t *testing.T) {
	cfg := config.Config{Actions: []config.Action{
		{Name: "same", Matches: "root", LaunchMode: config.LaunchModeSessionWindow, Cmd: "one"},
		{Name: "other", Matches: "root", LaunchMode: config.LaunchModeSessionWindow, Cmd: "other"},
		{Name: "same", Matches: "dir", Cmd: "two"},
	}}

	_, err := prepare(cfg, "same", "", nil)
	assertErrorContains(t, err, `2 exact matches`)
	assertErrorContains(t, err, `actions[0] matches="root"`)
	assertErrorContains(t, err, `actions[2] matches="dir"`)
}

func TestPrepareRejectsUnsupportedActionsBeforeInputs(t *testing.T) {
	tests := []struct {
		name   string
		action config.Action
		want   string
	}{
		{
			name:   "session",
			action: config.Action{Name: "run", Matches: "session", LaunchMode: config.LaunchModeSessionWindow, Cmd: "true"},
			want:   "matches session",
		},
		{
			name:   "effective shell",
			action: config.Action{Name: "run", Matches: "root", Cmd: "true"},
			want:   `effective launch_mode "shell"`,
		},
		{
			name:   "explicit shell dir",
			action: config.Action{Name: "run", Matches: "dir", LaunchMode: config.LaunchModeShell, Cmd: "true"},
			want:   `effective launch_mode "shell"`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := prepare(config.Config{Actions: []config.Action{tt.action}}, "run", "", []string{"malformed"})
			assertErrorContains(t, err, tt.want)
			if strings.Contains(err.Error(), "key=value") {
				t.Fatalf("unsupported action resolved inputs first: %v", err)
			}
		})
	}
}

func TestPreparePathRules(t *testing.T) {
	t.Run("root rejects path", func(t *testing.T) {
		cfg := config.Config{Actions: []config.Action{{
			Name: "run", Matches: "root", LaunchMode: config.LaunchModeSessionWindow, Cmd: "true",
		}}}
		_, err := prepare(cfg, "run", ".", nil)
		assertErrorContains(t, err, "--path is not valid")
	})

	t.Run("dir requires path", func(t *testing.T) {
		cfg := dirConfig(nil)
		_, err := prepare(cfg, "run", "", nil)
		assertErrorContains(t, err, "--path is required")
	})

	t.Run("dir rejects missing and file paths", func(t *testing.T) {
		dir := t.TempDir()
		file := filepath.Join(dir, "file")
		if err := os.WriteFile(file, []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
		for _, path := range []string{filepath.Join(dir, "missing"), file} {
			_, err := prepare(dirConfig(nil), "run", path, nil)
			if err == nil {
				t.Fatalf("prepare(path=%q) succeeded", path)
			}
		}
	})

	t.Run("dir rejects invalid UTF-8 before launch resolution", func(t *testing.T) {
		dir := filepath.Join(t.TempDir(), string([]byte{0xff}))
		_, err := prepare(dirConfig(nil), "run", dir, nil)
		assertErrorContains(t, err, "not valid UTF-8")
	})

	t.Run("dir rejects control characters before launch resolution", func(t *testing.T) {
		dir := filepath.Join(t.TempDir(), "bad\npath")
		if err := os.Mkdir(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		_, err := prepare(dirConfig(nil), "run", dir, nil)
		assertErrorContains(t, err, "contains control characters")
	})

	t.Run("dir provides absolute clean context", func(t *testing.T) {
		dir := t.TempDir()
		unclean := filepath.Join(dir, ".", "child", "..")
		prepared, err := prepare(dirConfig(nil), "run", unclean, nil)
		if err != nil {
			t.Fatalf("Prepare: %v", err)
		}
		want, err := filepath.Abs(dir)
		if err != nil {
			t.Fatal(err)
		}
		if len(prepared.Accumulated) != 1 {
			t.Fatalf("accumulated len = %d, want 1", len(prepared.Accumulated))
		}
		base := prepared.Accumulated[0]
		if base.Type != "dir" || base.Action != item.ActionNextList || base.Display != want || base.Data["path"] != want {
			t.Fatalf("base context = %#v, want interactive dir context for %q", base, want)
		}
	})
}

func TestPrepareRawInputParsing(t *testing.T) {
	stages := []config.StageConfig{
		{Type: "prompt", Key: "Target", Text: "Target:"},
		{Type: "picker", Key: "value", Source: "unused"},
	}
	prepared, err := prepare(dirConfig(stages), "run", t.TempDir(), []string{
		"Target=a,'b'.c[d]", "value=x=y=",
	})
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	data := execute.FlattenData(prepared.Accumulated)
	if got := data["Target"]; got != "a,'b'.c[d]" {
		t.Errorf("Target = %q", got)
	}
	if got := data["value"]; got != "x=y=" {
		t.Errorf("value = %q", got)
	}

	tests := []struct {
		name string
		raw  []string
		want string
	}{
		{name: "missing equals", raw: []string{"Target"}, want: "must use key=value"},
		{name: "empty key", raw: []string{"=x"}, want: "input key cannot be empty"},
		{name: "unknown exact-case key", raw: []string{"target=x"}, want: `unknown input key "target"`},
		{name: "duplicate exact key", raw: []string{"Target=x", "Target=y"}, want: `input key "Target" was supplied more than once`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := prepare(dirConfig(stages), "run", t.TempDir(), tt.raw)
			assertErrorContains(t, err, tt.want)
			assertErrorContains(t, err, "accepted keys: Target, value")
		})
	}
}

func TestPrepareRendersPromptDefaultFromPaneID(t *testing.T) {
	stages := []config.StageConfig{{
		Type: "prompt", Key: "origin", Text: "Origin:", Default: "{{.pane_id}}",
	}}
	prepared, err := Prepare(dirConfig(stages), "run", t.TempDir(), "%17", nil)
	if err != nil {
		t.Fatal(err)
	}
	if got := execute.FlattenData(prepared.Accumulated)["origin"]; got != "%17" {
		t.Errorf("origin = %q, want %%17", got)
	}
}

func TestPrepareStagesInDeclarationOrder(t *testing.T) {
	dir := t.TempDir()
	stages := []config.StageConfig{
		{Type: "prompt", Key: "first", Text: "First:", Default: "{{.path}}/one"},
		{Type: "prompt", Key: "second", Text: "Second:", Default: "{{.first}}/two"},
		{Type: "picker", Key: "third", Source: "unused"},
	}
	prepared, err := prepare(dirConfig(stages), "run", dir, []string{"third=literal"})
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	absolute, err := filepath.Abs(dir)
	if err != nil {
		t.Fatal(err)
	}
	preparedData := execute.FlattenData(prepared.Accumulated)
	if got, want := preparedData["first"], absolute+"/one"; got != want {
		t.Errorf("first = %q, want %q", got, want)
	}
	if got, want := preparedData["second"], absolute+"/one/two"; got != want {
		t.Errorf("second = %q, want %q", got, want)
	}
	if len(prepared.Accumulated) != 4 {
		t.Fatalf("accumulated len = %d, want directory plus three results", len(prepared.Accumulated))
	}
	for i, key := range []string{"first", "second", "third"} {
		result := prepared.Accumulated[i+1]
		if result.Type != "stage-result" || result.Data[key] != preparedData[key] {
			t.Errorf("result %d = %#v", i, result)
		}
	}
	if prepared.Selected.Action != item.ActionStaged {
		t.Errorf("selected action type = %q, want staged", prepared.Selected.Action)
	}

	_, data, err := execute.ResolveLaunch(prepared.Accumulated, prepared.Selected, "", config.DefaultConfig())
	if err != nil {
		t.Fatalf("ResolveLaunch with prepared context: %v", err)
	}
	for _, key := range []string{"first", "second", "third"} {
		if data[key] != preparedData[key] {
			t.Errorf("ResolveLaunch data[%q] = %q, want %q", key, data[key], preparedData[key])
		}
	}
	if data["path"] != absolute {
		t.Errorf("ResolveLaunch path = %q, want %q", data["path"], absolute)
	}
}

func TestPrepareMissingStageInputs(t *testing.T) {
	tests := []struct {
		name  string
		stage config.StageConfig
		want  string
	}{
		{
			name:  "prompt without default",
			stage: config.StageConfig{Type: "prompt", Key: "name", Text: "Name:"},
			want:  "prompt has no default",
		},
		{
			name:  "picker",
			stage: config.StageConfig{Type: "picker", Key: "name", Source: "unused"},
			want:  "picker input must be supplied",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := prepare(dirConfig([]config.StageConfig{tt.stage}), "run", t.TempDir(), nil)
			assertErrorContains(t, err, tt.want)
			assertErrorContains(t, err, "accepted inputs: name")
		})
	}
}

func TestPrepareEmptyInputSemantics(t *testing.T) {
	tests := []struct {
		name    string
		stage   config.StageConfig
		raw     []string
		wantErr bool
	}{
		{
			name:  "required prompt supplied whitespace",
			stage: config.StageConfig{Type: "prompt", Key: "value", Text: "Value:"},
			raw:   []string{"value=  \t"}, wantErr: true,
		},
		{
			name:  "optional prompt supplied empty",
			stage: config.StageConfig{Type: "prompt", Key: "value", Text: "Value:", AllowEmpty: true},
			raw:   []string{"value="},
		},
		{
			name:  "optional prompt default renders empty",
			stage: config.StageConfig{Type: "prompt", Key: "value", Text: "Value:", Default: "{{if .path}}{{end}}", AllowEmpty: true},
		},
		{
			name:    "required prompt default renders empty",
			stage:   config.StageConfig{Type: "prompt", Key: "value", Text: "Value:", Default: "{{if .path}}{{end}}"},
			wantErr: true,
		},
		{
			name:  "picker supplied empty",
			stage: config.StageConfig{Type: "picker", Key: "value", Source: "unused"},
			raw:   []string{"value="}, wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prepared, err := prepare(dirConfig([]config.StageConfig{tt.stage}), "run", t.TempDir(), tt.raw)
			if tt.wantErr {
				assertErrorContains(t, err, `input "value" cannot be empty`)
				return
			}
			if err != nil {
				t.Fatalf("Prepare: %v", err)
			}
			if got := execute.FlattenData(prepared.Accumulated)["value"]; got != "" {
				t.Errorf("value = %q, want literal empty", got)
			}
		})
	}
}

func TestPrepareDefaultTemplateError(t *testing.T) {
	stages := []config.StageConfig{{
		Type: "prompt", Key: "value", Text: "Value:", Default: "{{.missing}}",
	}}
	_, err := prepare(dirConfig(stages), "run", t.TempDir(), nil)
	assertErrorContains(t, err, `input "value" default template`)
	assertErrorContains(t, err, `map has no entry for key "missing"`)
}

func prepare(cfg config.Config, name, path string, rawInputs []string) (Prepared, error) {
	return Prepare(cfg, name, path, "", rawInputs)
}

func dirConfig(stages []config.StageConfig) config.Config {
	return config.Config{Actions: []config.Action{{
		Name: "run", Matches: "dir", Cmd: "printf '%s' {{sq .path}}", Stages: stages,
	}}}
}

func assertErrorContains(t *testing.T, err error, want string) {
	t.Helper()
	if err == nil {
		t.Fatalf("error = nil, want substring %q", want)
	}
	if !strings.Contains(err.Error(), want) {
		t.Fatalf("error = %q, want substring %q", err, want)
	}
}
