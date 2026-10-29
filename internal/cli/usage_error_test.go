package cli

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// TestRunReportsFailuresAsOnePrefixedLineWithExitStatus checks the failure
// contract end to end through the root command: usage errors exit 2, other
// failures exit 1, and every failure is one "cmdk: <message>" stderr line
// with no usage text.
func TestRunReportsFailuresAsOnePrefixedLineWithExitStatus(t *testing.T) {
	useTempConfigHome(t)
	tests := []struct {
		name       string
		args       []string
		wantStatus int
		wantStderr string
	}{
		{name: "unknown command", args: []string{"nope"}, wantStatus: exitUsage, wantStderr: `cmdk: unknown command "nope" for "cmdk"`},
		{name: "unknown nested command", args: []string{"session", "nope"}, wantStatus: exitUsage, wantStderr: `cmdk: unknown command "nope" for "cmdk session"`},
		{name: "unknown help topic", args: []string{"help", "nope"}, wantStatus: exitUsage, wantStderr: `cmdk: unknown help topic "nope"; run "cmdk --help" to list commands and topics`},
		{name: "unknown nested help topic", args: []string{"help", "session", "nope"}, wantStatus: exitUsage, wantStderr: `cmdk: unknown help topic "session nope"; run "cmdk --help" to list commands and topics`},
		{name: "operand after help topic", args: []string{"help", "launching", "extra"}, wantStatus: exitUsage, wantStderr: `cmdk: unknown help topic "launching extra"; run "cmdk --help" to list commands and topics`},
		{name: "unknown flag", args: []string{"--bogus"}, wantStatus: exitUsage, wantStderr: "cmdk: unknown flag: --bogus"},
		{name: "unknown flag under action group", args: []string{"action", "run", "x", "--bogus"}, wantStatus: exitUsage, wantStderr: "cmdk: unknown flag: --bogus"},
		{name: "operand on help topic", args: []string{"exit-codes", "extra"}, wantStatus: exitUsage, wantStderr: `cmdk: unknown command "extra" for "cmdk exit-codes"`},
		{name: "operand on completion group", args: []string{"completion", "extra"}, wantStatus: exitUsage, wantStderr: `cmdk: unknown command "extra" for "cmdk completion"`},
		{name: "operand on completion leaf", args: []string{"completion", "bash", "extra"}, wantStatus: exitUsage, wantStderr: `cmdk: unknown command "extra" for "cmdk completion bash"`},
		{name: "too many operands", args: []string{"shorten", "a", "b"}, wantStatus: exitUsage, wantStderr: "cmdk: accepts at most 1 arg(s), received 2"},
		{name: "missing operand", args: []string{"action", "run"}, wantStatus: exitUsage, wantStderr: "cmdk: accepts 1 arg(s), received 0"},
		{name: "custom grammar", args: []string{"session", "window"}, wantStatus: exitUsage, wantStderr: "cmdk: path is required"},
		{name: "fixed-set flag value", args: []string{"--color=bogus"}, wantStatus: exitUsage, wantStderr: `cmdk: invalid argument "bogus" for "--color" flag: must be auto, always, or never`},
		{name: "fixed-set theme value", args: []string{"--theme=bogus"}, wantStatus: exitUsage, wantStderr: `cmdk: invalid argument "bogus" for "--theme" flag: must be light or dark`},
		{name: "flag value error", args: []string{"--config", "", "config"}, wantStatus: exitFailure, wantStderr: "cmdk: config path cannot be empty"},
		{name: "command work error", args: []string{"session", "resolve", t.TempDir() + "/missing"}, wantStatus: exitFailure},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := newRootCommand()
			var stdout, stderr bytes.Buffer
			root.SetOut(&stdout)
			root.SetErr(&stdout)
			root.SetArgs(tt.args)

			status := run(root, &stderr)

			if status != tt.wantStatus {
				t.Errorf("status = %d, want %d (stderr %q)", status, tt.wantStatus, stderr.String())
			}
			lines := strings.Split(strings.TrimSuffix(stderr.String(), "\n"), "\n")
			if len(lines) != 1 || !strings.HasPrefix(lines[0], "cmdk: ") {
				t.Errorf("stderr = %q, want one line starting with \"cmdk: \"", stderr.String())
			}
			if tt.wantStderr != "" && lines[0] != tt.wantStderr {
				t.Errorf("stderr = %q, want %q", lines[0], tt.wantStderr)
			}
			if stdout.Len() != 0 {
				t.Errorf("command output = %q, want none (no usage text on failure)", stdout.String())
			}
		})
	}
}

func TestRunReturnsZeroWithoutOutputOnSuccess(t *testing.T) {
	root := newRootCommand()
	root.SetOut(&bytes.Buffer{})
	root.SetArgs([]string{"--version"})
	var stderr bytes.Buffer
	if status := run(root, &stderr); status != exitSuccess || stderr.Len() != 0 {
		t.Fatalf("run() = %d with stderr %q, want 0 and no stderr", status, stderr.String())
	}
}

func TestRunFoldsMultiLineDiagnosticIntoOneLine(t *testing.T) {
	root := &cobra.Command{
		Use:           "cmdk",
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(*cobra.Command, []string) error {
			return errors.New("first line\n  second line")
		},
	}
	root.SetArgs(nil)
	var stderr bytes.Buffer
	if status := run(root, &stderr); status != exitFailure {
		t.Fatalf("status = %d, want %d", status, exitFailure)
	}
	if got := stderr.String(); got != "cmdk: first line; second line\n" {
		t.Fatalf("stderr = %q, want one folded line", got)
	}
}

func TestExitStatusMapping(t *testing.T) {
	plain := errors.New("plain")
	tests := []struct {
		name string
		err  error
		want int
	}{
		{name: "nil", err: nil, want: exitSuccess},
		{name: "plain", err: plain, want: exitFailure},
		{name: "marked", err: markUsageError(plain), want: exitUsage},
		{name: "marked and wrapped", err: terminalSafeActionRunError(markUsageError(plain)), want: exitUsage},
		{name: "unknown command from discovery", err: errors.New(`unknown command "x" for "cmdk"`), want: exitUsage},
	}
	for _, tt := range tests {
		if got := exitStatus(tt.err); got != tt.want {
			t.Errorf("%s: exitStatus() = %d, want %d", tt.name, got, tt.want)
		}
	}
	if markUsageError(nil) != nil {
		t.Error("markUsageError(nil) should stay nil")
	}
	if !errors.Is(markUsageError(plain), plain) {
		t.Error("markUsageError should preserve the cause")
	}
}
