package cli

import "github.com/spf13/cobra"

func newExitCodesCommand(_ *rootOptions) *cobra.Command {
	return newHelpTopic("exit-codes", "Exit statuses and the stderr failure format", `Exit statuses cmdk returns and the failures that keep status 0.

Every failure is one diagnostic line on stderr in the form "cmdk: <message>"
with no usage text; captured output that spans lines is folded into it.
Payloads go to stdout, so a script reads stdout and branches on the status.

  0  Success. The launcher quit without a selection or launched the
     selection, or a subcommand did its work. These failures also keep 0:
     a launch resolution error inside the launcher (a failing
     launch_path_cmd or picker source, or a template error) is shown on an
     error screen and quitting afterwards exits 0; an empty "cmdk icons"
     result prints a hint on stderr.

  1  Failure during command work: a malformed or invalid configuration file
     from the discovered path or --config; a missing or too-old tmux; a tmux
     query or mutation that fails or times out; a path that is not an
     existing directory; an "action run" rejection (unknown, ambiguous, or
     unsupported action; missing, unknown, duplicate, or empty inputs; no
     attached client or no running server); a launch that fails after the
     launcher accepted a selection; a launcher started without a terminal;
     or an internal error. A logging setup failure is a stderr warning and
     does not change the status.

  2  Usage error, reported before any command work: an unknown command, an
     unknown help topic, an unknown flag, a flag value that does not parse
     (a non-numeric --truncate, a --color other than auto, always, or never,
     or a --theme other than light or dark), an unexpected operand, or
     operands that fail a command's positional grammar.

  *  Propagated from the command when the launcher runs a shell-mode action.
     cmdk replaces its own process with "sh -c <cmd>" through exec(2), so the
     command's exit status becomes cmdk's; any status other than 0, 1, or 2
     comes from the command. Session-window actions and "cmdk session
     window" run their command inside tmux, and its status never propagates.

See "cmdk action run --help" for the diagnostic it prints when the window was
created but switching the client failed, and "cmdk help launching" for the
launch modes.`)
}
