package cli

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/spf13/cobra"
)

// TestRootHelpEndsWithTaskListAndGroupBlock checks the root long
// description's shape: the contract text, then a common task list with a
// full invocation per line, then a block that ends the description with one
// line per first-level command group and its --help invocation.
func TestRootHelpEndsWithTaskListAndGroupBlock(t *testing.T) {
	root := newRootCommand()
	long := root.Long
	flat := strings.Join(strings.Fields(long), " ")

	for _, want := range []string{
		"interactive only",
		"--timings",
		"Diagnostics go to stderr",
		"$HOME/.local/state/cmdk/cmdk.log",
		"--color=auto",
		"NO_COLOR",
		"--theme=light or --theme=dark",
		"tmux 3.2 or newer",
		"git",
		"zoxide",
		"Nerd Font",
		"Windows is unsupported",
		configLoadingHelp,
	} {
		if !strings.Contains(flat, strings.Join(strings.Fields(want), " ")) {
			t.Errorf("root help missing %q", want)
		}
	}

	tasksAt := strings.Index(long, "\nCommon tasks:\n")
	groupsAt := strings.LastIndex(long, "\nCommand groups:\n")
	if tasksAt < 0 || groupsAt < 0 || tasksAt > groupsAt {
		t.Fatalf("root help must list common tasks before the command group block:\n%s", long)
	}
	taskLine := regexp.MustCompile(`^  [^:]+: +cmdk .*\S$`)
	for _, line := range strings.Split(strings.TrimRight(long[tasksAt+len("\nCommon tasks:\n"):groupsAt], "\n"), "\n") {
		if !taskLine.MatchString(line) {
			t.Errorf("task line %q must name the goal and its full cmdk invocation", line)
		}
	}

	var groups []string
	for _, child := range root.Commands() {
		if child.HasSubCommands() {
			groups = append(groups, child.Name())
		}
	}
	lines := strings.Split(long[groupsAt+len("\nCommand groups:\n"):], "\n")
	if len(lines) != len(groups) {
		t.Fatalf("group block has %d lines, want one per group %v:\n%s", len(lines), groups, strings.Join(lines, "\n"))
	}
	for i, group := range groups {
		if !strings.HasPrefix(lines[i], "  "+group+" ") || !strings.HasSuffix(lines[i], "  cmdk "+group+" --help") {
			t.Errorf("group line %q must name %s, what it covers, and \"cmdk %s --help\"", lines[i], group, group)
		}
	}
}

func TestRootHelpListsEveryHelpTopic(t *testing.T) {
	help := executeRootForTest(t, "--help")
	topicsAt := strings.Index(help, "\nHelp topics:\n")
	if topicsAt < 0 {
		t.Fatalf("root help has no help topics section:\n%s", help)
	}
	for _, topic := range []string{"exit-codes", "launching", "templates"} {
		if !strings.Contains(help[topicsAt:], "\n  "+topic+" ") {
			t.Errorf("root help does not list topic %q:\n%s", topic, help)
		}
	}
}

func TestExitCodesTopicListsEveryStatus(t *testing.T) {
	help := executeRootForTest(t, "help", "exit-codes")
	for _, want := range []string{"\n  0  ", "\n  1  ", "\n  2  ", "\n  *  ", `"cmdk: <message>"`, "unknown help topic", "exec(2)"} {
		if !strings.Contains(help, want) {
			t.Errorf("exit-codes help missing %q:\n%s", want, help)
		}
	}
}

func TestHelpTopicsCoverTheConceptsNoCommandOwns(t *testing.T) {
	tests := map[string][]string{
		"templates": {"{{.path}}", "{{.pane_id}}", "{{.session_key}}", "{{.launch_basename}}", "{{.<key>}}", "{{sq .path}}", "CMDK_PANE_ID", "shell metacharacters"},
		"launching": {"session-window", "exec(2)", "@cmdk_session_key", "worktree container", "8 KiB", "4 MiB", "error screen"},
	}
	for topic, wants := range tests {
		help := executeRootForTest(t, "help", topic)
		for _, want := range wants {
			if !strings.Contains(help, want) {
				t.Errorf("%s help missing %q", topic, want)
			}
		}
	}
}

// TestEveryApplicationCommandHelpIsCompleteAndFits is the one help test the
// documentation contract asks for: every application-owned command has short
// and long help, every rendered help line (flag usage included) fits 80
// columns, and a help topic prints its help when invoked directly.
func TestEveryApplicationCommandHelpIsCompleteAndFits(t *testing.T) {
	commands := map[string]*cobra.Command{}
	collectApplicationCommands(newRootCommand(), commands)

	for path, command := range commands {
		if strings.TrimSpace(command.Short) == "" {
			t.Errorf("%s has no short help", path)
		}
		if strings.TrimSpace(command.Long) == "" {
			t.Errorf("%s has no long help", path)
		}

		words := strings.Fields(path)[1:]
		help := executeRootForTest(t, append(words, "--help")...)
		for i, line := range strings.Split(help, "\n") {
			if width := ansi.StringWidth(line); width > 80 {
				t.Errorf("%s help line %d is %d columns, want at most 80: %q", path, i+1, width, line)
			}
		}

		if command.GroupID == topicGroupID {
			if direct := executeRootForTest(t, words...); direct != help {
				t.Errorf("%s run directly prints text other than its help:\n%s\n---\n%s", path, direct, help)
			}
		}
	}
}

func executeRootForTest(t *testing.T, args ...string) string {
	t.Helper()
	root := newRootCommand()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs(args)
	if err := root.Execute(); err != nil {
		t.Fatalf("cmdk %s: %v", strings.Join(args, " "), err)
	}
	return out.String()
}

func TestEmptyConfigFlagIsAnError(t *testing.T) {
	for _, args := range [][]string{
		{"--config", "", "config"},
		{"config", "--config="},
		{"--config=", "shorten", "/tmp"},
		{"config", "--validate", ""},
	} {
		root := newRootCommand()
		root.SetOut(io.Discard)
		root.SetErr(io.Discard)
		root.SetArgs(args)
		err := root.Execute()
		if err == nil || !strings.Contains(err.Error(), "config path cannot be empty") {
			t.Errorf("cmdk %s: err = %v, want config path cannot be empty", strings.Join(args, " "), err)
		}
	}
}

func TestPersistentConfigFlagReachesSubcommands(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "missing.toml")
	for _, args := range [][]string{
		{"--config", missing, "config"},
		{"--config", missing, "shorten", "/tmp"},
		{"--config", missing, "session", "resolve", t.TempDir()},
		{"session", "resolve", t.TempDir(), "--config", missing},
	} {
		root := newRootCommand()
		root.SetOut(io.Discard)
		root.SetErr(io.Discard)
		root.SetArgs(args)
		err := root.Execute()
		if err == nil || !strings.Contains(err.Error(), "config file not found: "+missing) {
			t.Errorf("cmdk %s: err = %v, want config file not found", strings.Join(args, " "), err)
		}
	}
}

func TestEveryCommandRejectsInvalidDiscoveredConfig(t *testing.T) {
	stubTmuxPrerequisite(t, func(context.Context) error { return nil })
	t.Setenv("TMUX", "")
	dir := t.TempDir()
	commands := [][]string{
		{},
		{"config"},
		{"shorten", dir},
		{"session", "resolve", dir},
		{"session", "window", dir, "--new"},
		{"attach", dir},
		{"action", "run", "example", "--no-switch"},
		{"window", "next"},
	}
	tests := []struct {
		name      string
		content   string
		wantInErr string
	}{
		{name: "malformed", content: "[[[broken", wantInErr: "toml"},
		{name: "invalid", content: "[timeout]\nfetch = '0s'", wantInErr: "timeout.fetch"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			xdg := useTempConfigHome(t)
			path := filepath.Join(xdg, "cmdk", "config.toml")
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, []byte(tt.content), 0o644); err != nil {
				t.Fatal(err)
			}

			for _, args := range commands {
				root := newRootCommand()
				var out bytes.Buffer
				root.SetOut(&out)
				root.SetErr(io.Discard)
				root.SetArgs(args)

				err := root.Execute()
				if err == nil || !strings.Contains(err.Error(), tt.wantInErr) {
					t.Errorf("cmdk %s: err = %v, want error mentioning %q", strings.Join(args, " "), err, tt.wantInErr)
				}
				if out.Len() != 0 {
					t.Errorf("cmdk %s: stdout = %q, want empty", strings.Join(args, " "), out.String())
				}
			}
		})
	}
}
