package cmd

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"

	"github.com/jmcampanini/cmdk/internal/config"
	"github.com/jmcampanini/cmdk/internal/execute"
	"github.com/jmcampanini/cmdk/internal/item"
	resolver "github.com/jmcampanini/cmdk/internal/session"
	"github.com/jmcampanini/cmdk/internal/tmux"
)

type grammarScenario struct {
	name string
	// args is the full invocation through the real root; "{dir}" is replaced
	// with a fresh temp directory per scenario.
	args []string
	// wantErr substrings mark a rejected form; empty means the form must
	// succeed. Rejected forms run with every boundary forbidden and a poisoned
	// config file, so any pre-rejection command work surfaces as a mismatched
	// error or a boundary failure.
	wantErr []string
	wantOut []string
	setup   func(t *testing.T, dir string) func(t *testing.T)
}

// grammarScenarios owns only positional grammar and pre-work rejection;
// command semantics stay with each command's focused tests. The root's
// interactive TUI flow is owned by the e2e suite, so its valid rows cover the
// framework short-circuits.
var grammarScenarios = map[string][]grammarScenario{
	"cmdk": {
		{name: "help flag", args: []string{"--help"}, wantOut: []string{"Keyboard-driven tmux launcher"}},
		{name: "version flag", args: []string{"--version"}, wantOut: []string{"cmdk version"}},
		{name: "rejects operand", args: []string{"stray-operand"}, wantErr: []string{`unknown command "stray-operand" for "cmdk"`}},
		{name: "suggests near miss", args: []string{"sessio"}, wantErr: []string{"Did you mean this?", "session"}},
	},
	"cmdk action": {
		{name: "bare shows help", args: []string{"action"}, wantOut: []string{"Run configured actions without opening the interactive picker"}},
		{name: "rejects operand", args: []string{"action", "bogus"}, wantErr: []string{`unknown command "bogus" for "cmdk action"`}},
		{name: "suggests near miss", args: []string{"action", "runn"}, wantErr: []string{"Did you mean this?", "run"}},
	},
	"cmdk action run": {
		{
			name: "runs configured action",
			args: []string{"action", "run", "grammar demo"},
			setup: func(t *testing.T, dir string) func(t *testing.T) {
				writeActionRunConfig(t, `
[[actions]]
name = "grammar demo"
matches = "root"
launch_path = "`+dir+`"
cmd = "true"
`)
				stubActionRunTmux(t, "%9")
				stubPreparedActionLaunch(t, func(execute.Launch) (execute.LaunchResult, error) {
					return execute.LaunchResult{
						LaunchPath: dir,
						SessionID:  "$1",
						SessionKey: dir,
						WindowID:   "@1",
						WindowName: "grammar",
						PaneID:     "%1",
					}, nil
				})
				return nil
			},
			wantOut: []string{`"action": "grammar demo"`},
		},
		{name: "rejects missing name", args: []string{"action", "run"}, wantErr: []string{"accepts 1 arg(s), received 0"}},
		{name: "rejects extra operand", args: []string{"action", "run", "grammar demo", "extra"}, wantErr: []string{"accepts 1 arg(s), received 2"}},
	},
	"cmdk attach": {
		{
			name: "attaches to path",
			args: []string{"attach", "{dir}"},
			setup: func(t *testing.T, _ string) func(t *testing.T) {
				stubTmuxPrerequisite(t, func(context.Context) error { return nil })
				called := useAttachTestHooks(t, nil)
				return func(t *testing.T) {
					if !called() {
						t.Error("attachResolvedSession was not reached")
					}
				}
			},
		},
		{name: "rejects extra operand", args: []string{"attach", "first", "second"}, wantErr: []string{"accepts at most 1 arg(s), received 2"}},
	},
	"cmdk config": {
		{
			name: "shows resolved config",
			args: []string{"config"},
			setup: func(t *testing.T, _ string) func(t *testing.T) {
				writeActionRunConfig(t, "[display]\nshorten_home = \"~\"\n")
				return nil
			},
			wantOut: []string{"shorten_home"},
		},
		{name: "rejects operand", args: []string{"config", "stray"}, wantErr: []string{`unknown command "stray" for "cmdk config"`}},
	},
	"cmdk docs": {
		{name: "shows reference", args: []string{"docs"}, wantOut: []string{"display"}},
		{name: "rejects operand", args: []string{"docs", "stray"}, wantErr: []string{`unknown command "stray" for "cmdk docs"`}},
	},
	"cmdk exit-codes": {
		{name: "bare shows help", args: []string{"exit-codes"}, wantOut: []string{"Exit codes returned by cmdk"}},
		{name: "rejects operand", args: []string{"exit-codes", "0"}, wantErr: []string{`unknown command "0" for "cmdk exit-codes"`}},
	},
	"cmdk icons": {
		{name: "bare shows icon help", args: []string{"icons"}, wantOut: []string{"ICON ALIASES"}},
		{name: "rejects operand", args: []string{"icons", "stray"}, wantErr: []string{`unknown command "stray" for "cmdk icons"`}},
	},
	"cmdk session": {
		{name: "bare shows help", args: []string{"session"}, wantOut: []string{"Resolve and manage cmdk sessions"}},
		{name: "rejects operand", args: []string{"session", "bogus"}, wantErr: []string{`unknown command "bogus" for "cmdk session"`}},
	},
	"cmdk session resolve": {
		{name: "resolves path", args: []string{"session", "resolve", "{dir}"}, wantOut: []string{"session_kind:", "directory"}},
		{name: "rejects missing path", args: []string{"session", "resolve"}, wantErr: []string{"accepts 1 arg(s), received 0"}},
		{name: "rejects extra operand", args: []string{"session", "resolve", "a", "b"}, wantErr: []string{"accepts 1 arg(s), received 2"}},
	},
	"cmdk session window": {
		{
			name: "new shell mode",
			args: []string{"session", "window", "{dir}", "--new"},
			setup: func(t *testing.T, _ string) func(t *testing.T) {
				got, called := stubSessionWindowCreation(t)
				return func(t *testing.T) {
					if !*called {
						t.Error("createResolvedSessionWindow was not reached")
						return
					}
					if !got.NewShell || len(got.Command) != 0 {
						t.Errorf("options = %+v, want new-shell mode", got)
					}
				}
			},
		},
		{
			name: "command mode",
			args: []string{"session", "window", "{dir}", "--", "echo", "hi"},
			setup: func(t *testing.T, _ string) func(t *testing.T) {
				got, called := stubSessionWindowCreation(t)
				return func(t *testing.T) {
					if !*called {
						t.Error("createResolvedSessionWindow was not reached")
						return
					}
					if got.NewShell || !slices.Equal(got.Command, []string{"echo", "hi"}) {
						t.Errorf("options = %+v, want command mode [echo hi]", got)
					}
				}
			},
		},
		// Rejected rows use nonexistent paths: reaching path validation would
		// yield a "path does not exist" error instead of the grammar message.
		{name: "rejects missing path", args: []string{"session", "window"}, wantErr: []string{"path is required"}},
		{name: "rejects missing mode", args: []string{"session", "window", "missing-dir"}, wantErr: []string{"--new or command args after --"}},
		{name: "rejects bare delimiter", args: []string{"session", "window", "missing-dir", "--"}, wantErr: []string{"--new or command args after --"}},
		{name: "rejects command without delimiter", args: []string{"session", "window", "missing-dir", "stray"}, wantErr: []string{"command args must follow --"}},
		{name: "rejects multiple paths", args: []string{"session", "window", "one", "two", "--", "true"}, wantErr: []string{"exactly one path before --"}},
		{name: "rejects mode conflict", args: []string{"session", "window", "missing-dir", "--new", "--", "echo"}, wantErr: []string{"--new cannot be used with command args"}},
		{name: "rejects empty name", args: []string{"session", "window", "--name=", "missing-dir", "--new"}, wantErr: []string{"--name cannot be empty"}},
	},
	"cmdk shorten": {
		{name: "shortens operand", args: []string{"shorten", "/usr/local/bin/tool"}, wantOut: []string{"/usr/local/bin/tool"}},
		{
			name: "shortens stdin",
			args: []string{"shorten"},
			setup: func(t *testing.T, _ string) func(t *testing.T) {
				swapStdin(t, "/opt/tool\n")
				return nil
			},
			wantOut: []string{"/opt/tool"},
		},
		{
			name: "rejects extra operand without reading stdin",
			args: []string{"shorten", "a", "b"},
			setup: func(t *testing.T, _ string) func(t *testing.T) {
				remaining := swapStdin(t, "stdin-marker\n")
				return func(t *testing.T) {
					if got := remaining(t); got != "stdin-marker\n" {
						t.Errorf("stdin remaining = %q, want untouched marker", got)
					}
				}
			},
			wantErr: []string{"accepts at most 1 arg(s), received 2"},
		},
	},
	"cmdk window": {
		{name: "bare shows help", args: []string{"window"}, wantOut: []string{"Switch between tmux windows"}},
		{name: "rejects operand", args: []string{"window", "bogus"}, wantErr: []string{`unknown command "bogus" for "cmdk window"`}},
	},
	"cmdk window next": {
		{
			name:  "switches next",
			args:  []string{"window", "next"},
			setup: expectWindowSwitch(tmux.WindowNext),
		},
		{name: "rejects operand", args: []string{"window", "next", "stray"}, wantErr: []string{`unknown command "stray" for "cmdk window next"`}},
	},
	"cmdk window previous": {
		{
			name:  "switches previous",
			args:  []string{"window", "previous"},
			setup: expectWindowSwitch(tmux.WindowPrevious),
		},
		{
			name:  "prev alias switches previous",
			args:  []string{"window", "prev"},
			setup: expectWindowSwitch(tmux.WindowPrevious),
		},
		{name: "rejects operand", args: []string{"window", "previous", "stray"}, wantErr: []string{`unknown command "stray" for "cmdk window previous"`}},
		{name: "prev alias rejects operand", args: []string{"window", "prev", "stray"}, wantErr: []string{`unknown command "stray" for "cmdk window previous"`}},
	},
}

func TestApplicationCommandGrammarInventory(t *testing.T) {
	commands := map[string]*cobra.Command{}
	collectApplicationCommands(newRootCommand(), commands)

	for path, command := range commands {
		if command.Args == nil {
			t.Errorf("%s has no explicit Args validator", path)
		}
		var valid, rejected bool
		for _, scenario := range grammarScenarios[path] {
			if len(scenario.wantErr) > 0 {
				rejected = true
			} else {
				valid = true
			}
		}
		if !valid {
			t.Errorf("%s has no valid grammar scenario", path)
		}
		if !rejected {
			t.Errorf("%s has no rejected grammar scenario", path)
		}
	}
	for path := range grammarScenarios {
		if _, ok := commands[path]; !ok {
			t.Errorf("grammar scenarios cover %q, which is not in the command tree", path)
		}
	}
}

func TestCommandGrammarMatrix(t *testing.T) {
	for path, scenarios := range grammarScenarios {
		for _, scenario := range scenarios {
			t.Run(path+"/"+scenario.name, func(t *testing.T) {
				runGrammarScenario(t, scenario)
			})
		}
	}
}

func TestFrameworkFlowsRemainIntact(t *testing.T) {
	scenarios := []grammarScenario{
		{name: "help command", args: []string{"help", "session", "window"}, wantOut: []string{"Create a fresh tmux window"}},
		{name: "completion script", args: []string{"completion", "bash"}, wantOut: []string{"cmdk"}},
	}
	for _, scenario := range scenarios {
		t.Run(scenario.name, func(t *testing.T) {
			runGrammarScenario(t, scenario)
		})
	}
}

func runGrammarScenario(t *testing.T, scenario grammarScenario) {
	t.Helper()
	rejected := len(scenario.wantErr) > 0
	if rejected {
		usePoisonedConfigHome(t)
	} else {
		useTempConfigHome(t)
	}
	forbidGrammarBoundaries(t)

	dir := t.TempDir()
	args := make([]string, len(scenario.args))
	for i, arg := range scenario.args {
		args[i] = strings.ReplaceAll(arg, "{dir}", dir)
	}

	var after func(t *testing.T)
	if scenario.setup != nil {
		after = scenario.setup(t, dir)
	}

	readStdout := captureProcessStdout(t)
	root := newRootCommand()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(io.Discard)
	root.SetArgs(args)
	err := root.Execute()
	combined := out.String() + readStdout(t)

	invocation := "cmdk " + strings.Join(args, " ")
	if rejected {
		if err == nil {
			t.Fatalf("%s: expected rejection, got success\noutput: %s", invocation, combined)
		}
		for _, want := range scenario.wantErr {
			if !strings.Contains(err.Error(), want) {
				t.Errorf("%s: error = %q, want substring %q", invocation, err.Error(), want)
			}
		}
	} else if err != nil {
		t.Fatalf("%s: %v", invocation, err)
	}
	for _, want := range scenario.wantOut {
		if !strings.Contains(combined, want) {
			t.Errorf("%s: output missing %q\n%s", invocation, want, combined)
		}
	}
	if after != nil {
		after(t)
	}
}

func collectApplicationCommands(command *cobra.Command, into map[string]*cobra.Command) {
	name := command.Name()
	if command.HasParent() && (name == "help" || name == "completion" || strings.HasPrefix(name, "__complete")) {
		return
	}
	into[command.CommandPath()] = command
	for _, child := range command.Commands() {
		collectApplicationCommands(child, into)
	}
}

func swapGrammarBoundary[T any](t *testing.T, target *T, replacement T) {
	t.Helper()
	old := *target
	*target = replacement
	t.Cleanup(func() { *target = old })
}

// forbidGrammarBoundaries fails the test if any command-work boundary is
// reached; valid scenarios override individual boundaries with permissive
// stubs in their setup.
func forbidGrammarBoundaries(t *testing.T) {
	t.Helper()
	fail := func(boundary string) error {
		t.Errorf("%s boundary reached before grammar acceptance", boundary)
		return errors.New(boundary + " boundary reached before grammar acceptance")
	}
	stubTmuxPrerequisite(t, func(context.Context) error { return fail("tmux prerequisite") })
	swapGrammarBoundary(t, &createResolvedSessionWindow, func(context.Context, resolver.Plan, string, tmux.SessionWindowOptions) (tmux.SessionWindowResult, error) {
		return tmux.SessionWindowResult{}, fail("session window creation")
	})
	swapGrammarBoundary(t, &switchRelativeWindow, func(context.Context, tmux.WindowDirection, tmux.WindowSwitchOptions) error {
		return fail("window switching")
	})
	swapGrammarBoundary(t, &attachResolvedSession, func(context.Context, resolver.Plan, string, tmux.AttachOptions) error {
		return fail("session attach")
	})
	swapGrammarBoundary(t, &currentActionRunClient, func(context.Context, time.Duration) (tmux.ClientTarget, error) {
		return tmux.ClientTarget{}, fail("tmux client lookup")
	})
	swapGrammarBoundary(t, &requireActionRunServer, func(context.Context, time.Duration) error {
		return fail("tmux server check")
	})
	swapGrammarBoundary(t, &resolveConfiguredActionLaunch, func([]item.Item, item.Item, string, config.Config) (execute.Launch, map[string]string, error) {
		return execute.Launch{}, nil, fail("action launch resolution")
	})
	swapGrammarBoundary(t, &executeConfiguredActionLaunch, func(execute.Launch) (execute.LaunchResult, error) {
		return execute.LaunchResult{}, fail("action launch execution")
	})
}

func stubSessionWindowCreation(t *testing.T) (*tmux.SessionWindowOptions, *bool) {
	t.Helper()
	stubTmuxPrerequisite(t, func(context.Context) error { return nil })
	got := &tmux.SessionWindowOptions{}
	called := new(bool)
	swapGrammarBoundary(t, &createResolvedSessionWindow, func(_ context.Context, _ resolver.Plan, _ string, opts tmux.SessionWindowOptions) (tmux.SessionWindowResult, error) {
		*called = true
		*got = opts
		return tmux.SessionWindowResult{}, nil
	})
	return got, called
}

func expectWindowSwitch(want tmux.WindowDirection) func(t *testing.T, dir string) func(t *testing.T) {
	return func(t *testing.T, _ string) func(t *testing.T) {
		var got tmux.WindowDirection
		called := false
		stubWindowSwitcher(t, func(_ context.Context, direction tmux.WindowDirection, _ tmux.WindowSwitchOptions) error {
			called = true
			got = direction
			return nil
		})
		return func(t *testing.T) {
			if !called {
				t.Error("switchRelativeWindow was not reached")
				return
			}
			if got != want {
				t.Errorf("direction = %v, want %v", got, want)
			}
		}
	}
}

// usePoisonedConfigHome plants an invalid config file at the discovered path
// so any pre-rejection configuration load surfaces as a distinguishable
// "loading config" error instead of the expected grammar message.
func usePoisonedConfigHome(t *testing.T) {
	t.Helper()
	xdg := useTempConfigHome(t)
	dir := filepath.Join(xdg, "cmdk")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "config.toml"), []byte("this is not [valid toml"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func swapStdin(t *testing.T, content string) func(t *testing.T) string {
	t.Helper()
	read, write, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := write.WriteString(content); err != nil {
		t.Fatal(err)
	}
	if err := write.Close(); err != nil {
		t.Fatal(err)
	}
	old := os.Stdin
	os.Stdin = read
	t.Cleanup(func() {
		os.Stdin = old
		_ = read.Close()
	})
	return func(t *testing.T) string {
		t.Helper()
		remaining, err := io.ReadAll(read)
		if err != nil {
			t.Fatal(err)
		}
		return string(remaining)
	}
}

// captureProcessStdout redirects os.Stdout to a file because several commands
// still print through the process stream rather than the command stream; #104
// owns that injection.
func captureProcessStdout(t *testing.T) func(t *testing.T) string {
	t.Helper()
	file, err := os.CreateTemp(t.TempDir(), "stdout")
	if err != nil {
		t.Fatal(err)
	}
	old := os.Stdout
	os.Stdout = file
	restored := false
	restore := func() {
		if !restored {
			os.Stdout = old
			restored = true
		}
	}
	t.Cleanup(restore)
	return func(t *testing.T) string {
		t.Helper()
		restore()
		if _, err := file.Seek(0, io.SeekStart); err != nil {
			t.Fatal(err)
		}
		content, err := io.ReadAll(file)
		if err != nil {
			t.Fatal(err)
		}
		_ = file.Close()
		return string(content)
	}
}
