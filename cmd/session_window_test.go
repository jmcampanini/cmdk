package cmd

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	resolver "github.com/jmcampanini/cmdk/internal/session"
	"github.com/jmcampanini/cmdk/internal/tmux"
)

func TestSessionWindowCommandUseDocumentsRequiredPath(t *testing.T) {
	cmd := newSessionWindowCommand()
	if !strings.Contains(cmd.Use, "<path>") {
		t.Errorf("Use = %q, want required <path>", cmd.Use)
	}
	if strings.Contains(cmd.Use, "[path]") {
		t.Errorf("Use = %q, should not document optional [path]", cmd.Use)
	}
}

// TestParseSessionWindowGrammar covers the operand rules no cobra validator
// expresses: one path, exactly one of --new or a command after --, and a
// leading-dash path placed after its own -- so pflag does not read it as a
// flag. It parses flags the way cobra does before calling Args, without
// running the tmux prerequisite or the runner.
func TestParseSessionWindowGrammar(t *testing.T) {
	tests := []struct {
		name    string
		argv    []string
		want    sessionWindowRequest
		wantErr string
	}{
		{name: "new shell", argv: []string{"dir", "--new"}, want: sessionWindowRequest{path: "dir"}},
		{name: "command after delimiter", argv: []string{"dir", "--", "echo", "hi"}, want: sessionWindowRequest{path: "dir", commandArgs: []string{"echo", "hi"}}},
		{name: "explicit name", argv: []string{"dir", "--name", "tests", "--new"}, want: sessionWindowRequest{path: "dir", nameSet: true}},
		{name: "leading-dash path with command", argv: []string{"--", "-dir", "--", "echo", "hi"}, want: sessionWindowRequest{path: "-dir", commandArgs: []string{"echo", "hi"}}},
		{name: "leading-dash path with new shell", argv: []string{"--new", "--", "-dir"}, want: sessionWindowRequest{path: "-dir"}},
		{name: "missing path", argv: nil, wantErr: "path is required"},
		{name: "missing mode", argv: []string{"dir"}, wantErr: "--new or command args after --"},
		{name: "bare delimiter", argv: []string{"dir", "--"}, wantErr: "--new or command args after --"},
		{name: "command without delimiter", argv: []string{"dir", "echo", "hi"}, wantErr: "command args must follow --"},
		{name: "two paths before delimiter", argv: []string{"one", "two", "--", "true"}, wantErr: "exactly one path before --"},
		{name: "new shell and command", argv: []string{"dir", "--new", "--", "echo"}, wantErr: "--new cannot be used with command args"},
		{name: "empty name", argv: []string{"dir", "--name=", "--new"}, wantErr: "--name cannot be empty"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cmd := &cobra.Command{}
			var options sessionWindowOptions
			options.bindFlags(cmd)
			if err := cmd.ParseFlags(test.argv); err != nil {
				t.Fatalf("ParseFlags(%q): %v", test.argv, err)
			}

			got, err := parseSessionWindowGrammar(cmd, cmd.Flags().Args(), options)

			if test.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), test.wantErr) {
					t.Fatalf("parseSessionWindowGrammar(%q) error = %v, want substring %q", test.argv, err, test.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseSessionWindowGrammar(%q): %v", test.argv, err)
			}
			if got.path != test.want.path || got.nameSet != test.want.nameSet || !slices.Equal(got.commandArgs, test.want.commandArgs) {
				t.Errorf("parseSessionWindowGrammar(%q) = %+v, want %+v", test.argv, got, test.want)
			}
		})
	}
}

func TestRunSessionWindowCommandNewShellDefaultsToBackground(t *testing.T) {
	useTempConfigHome(t)
	dir := filepath.Join(t.TempDir(), "scratch")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}

	oldCreate := createResolvedSessionWindow
	t.Cleanup(func() { createResolvedSessionWindow = oldCreate })

	called := false
	createResolvedSessionWindow = func(ctx context.Context, plan resolver.Plan, _ string, opts tmux.SessionWindowOptions) (tmux.SessionWindowResult, error) {
		called = true
		if _, ok := ctx.Deadline(); ok {
			return tmux.SessionWindowResult{}, errors.New("window context unexpectedly inherited resolve timeout")
		}
		if err := ctx.Err(); err != nil {
			return tmux.SessionWindowResult{}, err
		}
		if plan.SessionKind != resolver.KindDirectory {
			t.Errorf("SessionKind = %q, want %q", plan.SessionKind, resolver.KindDirectory)
		}
		if !opts.NewShell {
			t.Error("NewShell = false, want true")
		}
		if len(opts.Command) != 0 {
			t.Errorf("Command = %q, want empty", opts.Command)
		}
		if opts.Switch {
			t.Error("Switch = true, want false")
		}
		return tmux.SessionWindowResult{}, nil
	}

	cmd := &cobra.Command{}
	if err := runSessionWindowCommand(cmd, sessionWindowRequest{path: dir}, sessionWindowOptions{newShell: true}); err != nil {
		t.Fatal(err)
	}
	if !called {
		t.Fatal("createResolvedSessionWindow was not called")
	}
}

func TestRunSessionWindowCommandCommandModePassesArgvUnchanged(t *testing.T) {
	useTempConfigHome(t)
	dir := filepath.Join(t.TempDir(), "scratch")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}

	oldCreate := createResolvedSessionWindow
	t.Cleanup(func() { createResolvedSessionWindow = oldCreate })

	wantCommand := []string{"echo", "hello $HOME", "|", "tee", "x"}
	createResolvedSessionWindow = func(_ context.Context, _ resolver.Plan, _ string, opts tmux.SessionWindowOptions) (tmux.SessionWindowResult, error) {
		if opts.NewShell {
			t.Error("NewShell = true, want false")
		}
		if !slices.Equal(opts.Command, wantCommand) {
			t.Errorf("Command = %q, want %q", opts.Command, wantCommand)
		}
		return tmux.SessionWindowResult{}, nil
	}

	cmd := &cobra.Command{}
	if err := runSessionWindowCommand(cmd, sessionWindowRequest{path: dir, commandArgs: wantCommand}, sessionWindowOptions{}); err != nil {
		t.Fatal(err)
	}
}

func TestRunSessionWindowCommandNameOverride(t *testing.T) {
	useTempConfigHome(t)
	dir := filepath.Join(t.TempDir(), "scratch")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}

	oldCreate := createResolvedSessionWindow
	t.Cleanup(func() { createResolvedSessionWindow = oldCreate })

	createResolvedSessionWindow = func(_ context.Context, _ resolver.Plan, _ string, opts tmux.SessionWindowOptions) (tmux.SessionWindowResult, error) {
		if opts.Name != "tests" {
			t.Errorf("Name = %q, want tests", opts.Name)
		}
		return tmux.SessionWindowResult{}, nil
	}

	cmd := &cobra.Command{}
	if err := runSessionWindowCommand(cmd, sessionWindowRequest{path: dir, nameSet: true}, sessionWindowOptions{newShell: true, name: "tests"}); err != nil {
		t.Fatal(err)
	}
}

func TestSessionWindowCommandParsesFlagsOnlyBeforeDashDash(t *testing.T) {
	useTempConfigHome(t)
	dir := filepath.Join(t.TempDir(), "scratch")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}

	oldCreate := createResolvedSessionWindow
	t.Cleanup(func() { createResolvedSessionWindow = oldCreate })
	stubTmuxPrerequisite(t, func(context.Context) error { return nil })

	var got tmux.SessionWindowOptions
	createResolvedSessionWindow = func(_ context.Context, _ resolver.Plan, _ string, opts tmux.SessionWindowOptions) (tmux.SessionWindowResult, error) {
		got = opts
		return tmux.SessionWindowResult{}, nil
	}

	tests := []struct {
		name        string
		args        []string
		wantSwitch  bool
		wantCommand []string
	}{
		{name: "payload flag", args: []string{dir, "--", "--flag", "value"}, wantCommand: []string{"--flag", "value"}},
		{name: "switch before delimiter", args: []string{dir, "--switch", "--", "echo", "hello"}, wantSwitch: true, wantCommand: []string{"echo", "hello"}},
		{name: "switch after delimiter", args: []string{dir, "--", "--switch"}, wantCommand: []string{"--switch"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got = tmux.SessionWindowOptions{}
			cmd := newSessionWindowCommand()
			cmd.SetArgs(test.args)
			if err := cmd.Execute(); err != nil {
				t.Fatal(err)
			}
			if got.Switch != test.wantSwitch {
				t.Errorf("Switch = %t, want %t", got.Switch, test.wantSwitch)
			}
			if !slices.Equal(got.Command, test.wantCommand) {
				t.Errorf("Command = %q, want %q", got.Command, test.wantCommand)
			}
		})
	}
}

func TestRunSessionWindowCommandThreadsConfiguredWindowNameMaxLength(t *testing.T) {
	xdg := useTempConfigHome(t)
	cfgDir := filepath.Join(xdg, "cmdk")
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cfgDir, "config.toml"), []byte("[behavior]\nwindow_name_max_length = 7\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(t.TempDir(), "a-very-long-directory-name")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}

	oldCreate := createResolvedSessionWindow
	t.Cleanup(func() { createResolvedSessionWindow = oldCreate })

	called := false
	createResolvedSessionWindow = func(_ context.Context, _ resolver.Plan, _ string, opts tmux.SessionWindowOptions) (tmux.SessionWindowResult, error) {
		called = true
		if opts.Name != "a-very-long-directory-name" {
			t.Errorf("Name = %q, want untruncated basename", opts.Name)
		}
		if opts.MaxNameLength != 7 {
			t.Errorf("MaxNameLength = %d, want configured 7", opts.MaxNameLength)
		}
		return tmux.SessionWindowResult{}, nil
	}

	cmd := &cobra.Command{}
	if err := runSessionWindowCommand(cmd, sessionWindowRequest{path: dir}, sessionWindowOptions{newShell: true}); err != nil {
		t.Fatal(err)
	}
	if !called {
		t.Fatal("createResolvedSessionWindow was not called")
	}
}

func TestRunSessionWindowCommandDefaultsWindowNameMaxLength(t *testing.T) {
	useTempConfigHome(t)
	dir := filepath.Join(t.TempDir(), "scratch")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}

	oldCreate := createResolvedSessionWindow
	t.Cleanup(func() { createResolvedSessionWindow = oldCreate })

	called := false
	createResolvedSessionWindow = func(_ context.Context, _ resolver.Plan, _ string, opts tmux.SessionWindowOptions) (tmux.SessionWindowResult, error) {
		called = true
		if opts.MaxNameLength != 20 {
			t.Errorf("MaxNameLength = %d, want default 20", opts.MaxNameLength)
		}
		return tmux.SessionWindowResult{}, nil
	}

	cmd := &cobra.Command{}
	if err := runSessionWindowCommand(cmd, sessionWindowRequest{path: dir}, sessionWindowOptions{newShell: true}); err != nil {
		t.Fatal(err)
	}
	if !called {
		t.Fatal("createResolvedSessionWindow was not called")
	}
}
