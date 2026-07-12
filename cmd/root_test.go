package cmd

import (
	"bytes"
	"strings"
	"testing"
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
		"cmdk session window <path> [--switch] --new",
		"cmdk session window <path> [--switch] -- <command> [args...]",
		"Windows are created in the background by default",
		"The path determines the managed session",
		"Resolve paths and launch windows in managed sessions",
	} {
		if !strings.Contains(help, want) {
			t.Errorf("root help missing %q\n%s", want, help)
		}
	}
}
