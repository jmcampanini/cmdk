package cli

import "github.com/spf13/cobra"

func newTemplatesCommand(_ *rootOptions) *cobra.Command {
	return newHelpTopic("templates", "Template variables for action commands and stages", `Template variables available to action commands and stage fields.

cmd, launch_path, launch_path_cmd, window_name, and the stage fields text,
default, and source are Go text/template strings. See "cmdk config --help"
for the keys and "cmdk help launching" for when each one is rendered.

Variables from the selection:
  {{.path}}             selected directory (dir actions)
  {{.pane_id}}          invoking tmux pane: --pane-id in the launcher, the
                        current client's pane for "cmdk action run", and
                        unavailable with --no-switch
  {{.session_id}}       stable tmux session ID (window and session items)
  {{.session_key}}      canonical path key of a cmdk-managed session, when
                        the session has one (window and session items)
  {{.session_name}}     display-safe tmux session name (window and session
                        items)
  {{.session_attached}} attached client count (session items)
  {{.session_display}}  display string of the selected session
  {{.session_kind}}     session classification; "external" today
  {{.session_windows}}  window count (session items)
  {{.window_id}}        stable tmux window ID (window items)
  {{.window_index}}     tmux window index (window items)
  {{.window_name}}      display-safe tmux window name (window items)
  {{.launch_path}}      validated launch directory (cmd and window_name only)
  {{.launch_basename}}  base name of launch_path (cmd and window_name only)
  {{.<key>}}            the action input a stage with that key collected

Functions:
  {{sq .path}}          shell-safe single quoting

Security: path, session_name, picker values, and prompt input come from tmux,
zoxide, and the person typing, and may contain shell metacharacters. Wrap
every value that reaches a command in {{sq .var}}. session_name renders tabs
and newlines as display glyphs and is neither a stable tmux target nor shell
safe; session_key is a grouping key, not a tmux target. Target tmux by
session_id, as the built-in session switch and child-window actions do.

Command environment: shell-mode commands and launch_path_cmd run with a clean
CMDK_* namespace. cmdk removes inherited CMDK_* variables and sets CMDK_PATH,
CMDK_PANE_ID, and CMDK_LAUNCH_PATH when each value is available. Session-
window payloads receive no action inputs through the environment: use template
variables such as {{.launch_path}} in cmd. An interactive session-window shell
receives no CMDK_* variables from cmdk.`)
}
