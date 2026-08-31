package cmd

import (
	"bytes"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestRootHelpMakesSessionWindowLaunchingDiscoverable(t *testing.T) {
	cmd := newRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Help(); err != nil {
		t.Fatal(err)
	}

	help := out.String()
	for _, want := range []string{
		"cmdk action run <exact-name> --path <dir> --input key=value [--no-switch]",
		"Run configured actions noninteractively",
		"cmdk session window <path> [--switch] --new",
		"cmdk session window <path> [--switch] -- <command> [args...]",
		"Windows are created in the background by default",
		"The path determines the managed session",
		"tmux 3.2 or newer",
		"Linux or macOS",
		"zoxide is optional",
		"Nerd Font is optional",
		"Resolve paths and launch windows in managed sessions",
	} {
		if !strings.Contains(help, want) {
			t.Errorf("root help missing %q\n%s", want, help)
		}
	}
}

func TestExitCodesTopicPrintsSameHelpFromBothEntryPoints(t *testing.T) {
	direct := executeRootForTest(t, "exit-codes")
	viaHelp := executeRootForTest(t, "help", "exit-codes")

	if direct != viaHelp {
		t.Fatalf("exit-codes output differs between entry points:\n%s\n---\n%s", direct, viaHelp)
	}
	for _, want := range []string{"\n  0   ", "\n  1   ", "\n  *   "} {
		if !strings.Contains(direct, want) {
			t.Errorf("exit-codes help missing %q:\n%s", want, direct)
		}
	}
}

func TestEveryApplicationCommandHasWrappedLongHelp(t *testing.T) {
	commands := map[string]*cobra.Command{}
	collectApplicationCommands(newRootCommand(), commands)

	for path, command := range commands {
		if strings.TrimSpace(command.Long) == "" {
			t.Errorf("%s has no long help", path)
		}
		if command.Name() == "docs" {
			// The docs Long is rendered from internal/config/docs.go, which
			// fleet#20 leaves out of scope.
			continue
		}
		for field, text := range map[string]string{"Long": command.Long, "Example": command.Example} {
			for i, line := range strings.Split(text, "\n") {
				if len(line) > 80 {
					t.Errorf("%s %s line %d is %d columns, want at most 80: %q", path, field, i+1, len(line), line)
				}
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
