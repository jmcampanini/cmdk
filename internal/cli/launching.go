package cli

import "github.com/spf13/cobra"

func newLaunchingCommand(_ *rootOptions) *cobra.Command {
	return newHelpTopic("launching", "Launch modes, managed tmux sessions, and output limits", `Launch modes, managed tmux sessions, and command output limits.

Launch modes

An action's effective launch mode is session-window or shell. launch_mode =
"detect" (the default) picks session-window for dir actions and for actions
with launch_path or launch_path_cmd, and shell otherwise. See "cmdk config
--help" for the keys.

In session-window mode cmdk resolves the launch path, renders cmd, creates a
window in the cmdk-managed session for that path, runs cmd there through
sh -lc, and exits 0. The command's exit status stays in tmux. cmdk passes no
action data to tmux through -e or the session environment; use template
variables in cmd ("cmdk help templates"). {{.launch_path}} is the final
directory; {{.path}} is the directory selected before launch_path applied.

In shell mode cmdk replaces its own process with "sh -c <cmd>" through
exec(2) in the current pane, so the command's exit status becomes cmdk's.
When the action has a launch path, cmdk changes to that directory first;
otherwise the command inherits the directory cmdk started in, and relative
paths in cmd resolve from there.

Managed sessions

cmdk recognizes a managed session by its @cmdk_session_key tmux session
option, which holds the canonical path the session is for. When cmdk creates
a session it sets only @cmdk_session_kind and @cmdk_session_key, and it finds
a session by exact key match. A repository, its worktrees, and its worktree
container share one managed session keyed by the repository; any other
directory gets one managed session keyed by its canonical path. "cmdk session
resolve" shows the key for a path without touching tmux. Each launch creates
a fresh window; cmdk never reuses a window by name.

In the launcher, tmux windows sort above sessions, directories, and actions.
Inside a selected session, its windows sort above the built-in "Switch to
session" action and configured session actions.

Command output limits

launch_path_cmd and picker source commands run through sh -c under
timeout.picker. launch_path_cmd must print exactly one absolute path to an
existing directory. When either command fails (nonzero exit, timeout, or
invalid output), the launcher stays open and shows an error screen with the
rendered command, exit status, and captured stdout and stderr; the same
details go to the log file. Captured output is bounded: 8 KiB of stdout and
32 KiB of stderr for launch_path_cmd, 4 MiB and 64 KiB for a picker source.
stdout beyond its cap fails the command instead of being parsed partially;
stderr beyond its cap is truncated with a note.`)
}
