package cmd

import "github.com/spf13/cobra"

var exitCodesTopic = &cobra.Command{
	Use:   "exit-codes",
	Short: "Exit codes and error categories",
	Long: `Exit codes returned by cmdk.

  0   Success. The TUI ran to completion and either executed a selected
      action that itself returned 0, or exited without a selection.

  1   cmdk error. An invalid config file passed via --config, an
      unrecoverable startup failure, or an internal error. Configuration
      problems discovered in the default config path appear as a failed
      source row inside the TUI rather than causing a non-zero exit.
      Logging setup failures produce a stderr warning and do not cause
      exit 1.

  *   Propagated. When you select an action, cmdk replaces its own process
      with the action's command via an exec(2) syscall. The action's exit
      code becomes cmdk's exit code, so any value other than 0 or 1
      originates from the action itself, not cmdk.

Subcommands follow the same convention: 0 on success, 1 on
cmdk-detected errors. Empty results (e.g. "cmdk icons" with a filter
that matches nothing) print a message to stderr and exit 0; an empty
result is not treated as an error.

For the log file location and how to inspect it, run "cmdk logs --help".
`,
}

func init() {
	rootCmd.AddCommand(exitCodesTopic)
}
