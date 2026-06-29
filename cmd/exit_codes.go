package cmd

import "github.com/spf13/cobra"

func newExitCodesCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "exit-codes",
		Short: "Exit codes and error categories",
		Long: `Exit codes returned by cmdk.

  0   Success. The TUI ran to completion and either executed a selected
      action that itself returned 0, or exited without a selection.

  1   cmdk error. An invalid config file passed via --config, an
      unrecoverable startup failure, an action launch error, or an internal
      error. Configuration problems discovered in the default config path
      appear as a failed source row inside the TUI rather than causing a
      non-zero exit. Logging setup failures produce a stderr warning and do
      not cause exit 1.

  *   Propagated for shell-mode actions. When a selected action runs in shell
      mode, cmdk replaces its own process with the action's command via an
      exec(2) syscall. The action's exit code becomes cmdk's exit code, so
      any value other than 0 or 1 originates from the action itself, not cmdk.

      Session-window actions create and switch to a tmux window, then cmdk
      exits 0 once that launch succeeds. The payload runs inside tmux and its
      eventual exit status is not propagated to the cmdk process.

Subcommands follow the same convention: 0 on success, 1 on
cmdk-detected errors. Empty results (e.g. "cmdk icons" with a filter
that matches nothing) print a message to stderr and exit 0; an empty
result is not treated as an error.
`,
	}
}
