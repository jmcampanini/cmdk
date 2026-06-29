package cmd

import "github.com/spf13/cobra"

func newLogsCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "logs",
		Short: "Inspect cmdk log file",
		Long: `Inspect the cmdk log file.

cmdk appends events to a single log file at:

  $HOME/.local/state/cmdk/cmdk.log

The directory is created on first run. The file is never rotated or
truncated; remove it manually if it grows too large.

Logged events include configuration warnings, the command line of any
executed action, and errors from source fetches, async sources, and
TUI internals. Startup phase timings are emitted by --timings, not to
this log.

Subcommands:

  cmdk logs path        Print the absolute path to the log file
  cmdk logs tail [-n N] Print the last N lines (default 25, max 10000)

The log is plain text with one event per line and is safe to inspect
with any pager (` + "`tail -f`, `less`" + `, etc.).
`,
	}
	cmd.AddCommand(newLogsPathCommand(), newLogsTailCommand())
	return cmd
}
