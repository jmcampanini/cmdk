# Plan: Session-ed tmux windows

## Goal

Replace the current `cmdk session connect` direction with a focused command that creates a **new tmux window in a cmdk-managed session derived from a directory**, optionally running a command in that window.

A **session-ed window** means:

- The tmux **session** is determined from the selected directory/path using cmdk's existing session resolver.
- The tmux **window** is created inside that managed session.
- The window's working directory is the resolver's launch path.
- The new window is selected by stable tmux IDs, not by display names.

This is not about connecting to a session. Existing root window listing already supports switching to windows across all sessions.

## Existing behavior and reusable pieces

### Already exists

- Root TUI lists windows from all sessions via `tmux list-windows -a`.
- Window selection switches with:

  ```sh
  tmux switch-client -t <session_id>:<window_id>
  ```

  Therefore selecting a window from another session should switch the current tmux client to that session/window.

- Root TUI lists sessions via `tmux list-sessions`.
- Selecting a session drills into:
  - built-in `Connect`
  - configured session actions
  - windows in that session

- `cmdk session resolve <path>` already resolves path identity into:
  - `session_kind`
  - `session_key`
  - `session_display`
  - `launch_path`
  - `planned_tmux_session_name`
  - `planned_tmux_window_name`

- `internal/tmux/connect.go` already has most tmux primitives needed:
  - find managed session by `@cmdk_session_key`
  - create managed session
  - set session metadata
  - create a tmux window
  - switch current client to a target

### Existing behavior to preserve/add/remove

- `cmdk session connect <path>` is no longer a primary feature and should be removed as a user-facing command.
- Connect-specific behavior should be deleted or simplified where possible.
- Preserve the current built-in dir action unchanged:

  ```sh
  tmux new-window -c {{sq .path}}
  ```

  This keeps the existing plain tmux behavior: create a new window in the current tmux session using the selected directory as cwd.
- Add a second built-in dir action for session-ed windows:

  ```sh
  cmdk session window {{sq .path}} --new
  ```

  This creates a new window in the cmdk-managed session derived from the selected directory.

## User-facing API

### Commands

Keep:

```sh
cmdk session resolve <path>
```

Add:

```sh
cmdk session window <path> --new
cmdk session window <path> [--name <name>] -- <command> [args...]
```

Remove:

```sh
cmdk session connect <path>
```

### Examples

Create a fresh shell window in the managed session for the current directory:

```sh
cmdk session window . --new
```

Create a fresh window named from the directory and run `claude`:

```sh
cmdk session window ~/Code/project -- claude
```

Create a fresh window with an explicit name and run a command:

```sh
cmdk session window ~/Code/project --name test -- npm test
```

Use shell features explicitly by invoking a shell:

```sh
cmdk session window . --name tests -- sh -lc 'npm test | tee test.log'
```

## CLI contract

### Positional arguments

```sh
cmdk session window <path>
```

- `<path>` is required.
- `<path>` must exist.
- `<path>` must be a directory.
- Path classification follows the existing session resolver:
  - repo/worktree paths group into repo/container sessions
  - non-repo directories get one managed session per canonical directory

### Modes

Exactly one mode is required:

1. New interactive shell window:

   ```sh
   --new
   ```

2. Command window:

   ```sh
   -- <command> [args...]
   ```

Invalid:

```sh
cmdk session window .
cmdk session window . --new -- echo hi
```

Expected errors:

- neither `--new` nor command provided → error
- both `--new` and command provided → error
- invalid/missing path → resolver error
- tmux failure → tmux command error with stderr context where available

### Command handling

Command args after `--` are treated as argv-style input.

Example:

```sh
cmdk session window . -- echo 'hello $HOME'
```

cmdk treats this as conceptual argv:

```go
[]string{"echo", "hello $HOME"}
```

Then cmdk safely shell-quotes each argv part when passing the command string to tmux.

Resulting command string conceptually resembles:

```sh
'echo' 'hello $HOME'
```

This means:

- shell metacharacters in arguments are not interpreted by default
- spaces are preserved
- `$HOME`, `|`, `>`, `;`, etc. are literal unless the user explicitly invokes a shell
- shell behavior remains available via `sh -lc '...'`

### Window naming

Default window name:

- `plan.PlannedTmuxWindowName`
- usually the directory/worktree basename

Optional override:

```sh
--name <name>
```

Rules:

- `--name` applies to both `--new` and command mode.
- Empty `--name` is invalid.
- tmux duplicate window names are allowed.
- Correctness must not depend on window name uniqueness.
- New windows must be tracked by returned `window_id`.

### Switching behavior

`cmdk session window` creates the window and switches the current tmux client to it by default.

Contract:

1. Resolve path to plan.
2. Find or create the cmdk-managed tmux session for `plan.SessionKey`.
3. Create a fresh window in that session.
4. Switch current tmux client to the created window using `session_id:window_id`.

No detached/background mode in this phase.

Implication:

- This command expects to run from inside an active tmux client.
- If the final `switch-client` fails because there is no current client, the command should return a tmux error.
- Future optional extension: `--detach`, but do not implement now.

## Managed session contract

Managed sessions continue to be identified by metadata, not by tmux session name.

Session options:

- `@cmdk_session_kind`
- `@cmdk_session_key`
- `@cmdk_session_display`

Rules:

- On session creation, set the metadata above.
- Existing managed sessions are found by exact `@cmdk_session_key` match.
- If multiple sessions have the same `@cmdk_session_key`, fail rather than guessing.
- tmux session names are creation/display handles only; they are not identity.

## Window creation contract

For `--new`:

- create a new tmux window
- working directory: `plan.LaunchPath`
- window name: `--name` if set, else `plan.PlannedTmuxWindowName`
- no explicit command is passed to tmux; tmux starts the user's default shell

For command mode:

- create a new tmux window
- working directory: `plan.LaunchPath`
- window name: `--name` if set, else `plan.PlannedTmuxWindowName`
- command is the safely shell-quoted argv command string

Implementation shape:

```sh
tmux new-window -P -F '#{window_id}' -t '<session_id>:' -n '<window_name>' -c '<launch_path>' [shell-command]
tmux switch-client -t '<session_id>:<window_id>'
```

Use internal argv execution for tmux calls, not shell strings.

## TUI behavior

### Built-in dir actions

Keep the existing built-in dir action exactly as the plain tmux action:

```sh
tmux new-window -c {{sq .path}}
```

Add a new built-in dir action for session-ed windows:

```sh
cmdk session window {{sq .path}} --new
```

or equivalent internal command invocation if direct internal execution becomes available later.

Expected display and ordering:

- Existing action remains `New window`.
- New action should be named `New session window` unless a better label is chosen during implementation.
- Order should preserve the existing action first, then the session-ed action, then user-configured dir actions.
- The existing `New window` behavior must continue to create a window in the current tmux session.
- The new `New session window` behavior creates a window inside the cmdk-managed session for the directory.

TUI implication:

- With two default dir actions, `auto_select_single` will no longer auto-execute a dir selection in the default config because the dir action list is no longer a single item.
- That is acceptable for this plan because users need to choose between plain `New window` and `New session window`.

### Config actions

No config schema change is required in this phase.

Users can define command-running dir actions themselves, e.g.:

```toml
[[actions]]
name = "Claude"
matches = "dir"
cmd = "cmdk session window {{sq .path}} --name claude -- claude"
```

Future extension could add first-class action helpers, but do not introduce config schema now.

## Code organization plan

### `cmd/session.go`

- Keep `session resolve`.
- Remove `session connect` command registration and documentation.
- Add `session window` command.
- Define window options:

  ```go
  type sessionWindowOptions struct {
      newShell bool
      name     string
  }
  ```

- Cobra command shape should allow arbitrary command args after `--`.
- Validate:
  - exact required path
  - mode exclusivity
  - non-empty `--name` when present
- Resolve the session plan using existing `resolveSessionPlanForCommand`.
- Call a tmux package function such as:

  ```go
  tmux.CreateResolvedSessionWindow(ctx, plan, tmux.SessionWindowOptions{
      Name: opts.name,
      NewShell: opts.newShell,
      Command: commandArgs,
      Switch: true,
  })
  ```

### `internal/generator/actions.go`

- Preserve the existing built-in `New window` item and command.
- Add a new built-in `New session window` item after `New window` and before config-provided dir actions.
- The new item should execute:

  ```sh
  cmdk session window {{sq .path}} --new
  ```

- Continue to include `pane_id` in item data when present.
- Existing config dir actions should remain appended after built-in actions in config order.

### `internal/tmux`

Refactor `connect.go` into reusable session/window operations.

Suggested exported API:

```go
type SessionWindowOptions struct {
    Name    string
    NewShell bool
    Command []string
    Switch  bool
}

func CreateResolvedSessionWindow(ctx context.Context, plan resolver.Plan, opts SessionWindowOptions) error
```

Internal helpers:

- `validateConnectionPlan` can become `validateSessionPlan`.
- `findManagedSession` remains useful.
- `createSession` remains useful, but should return the created session ID and initial window ID.
- For this new command, if a session must be created, decide whether to use tmux's initial window as the requested new window or create an extra window:
  - Preferred: when session is missing, create the session with the desired initial window and command.
  - When session exists, create a new window.
- `setSessionMetadata` remains useful.
- Add command argv shell quoting helper for tmux's shell-command argument.
- `switchClient` remains useful.

Important distinction from old connect:

- Do not search for an existing window by name.
- Do not reuse existing windows.
- Always produce a fresh window for the requested operation.

### Initial window creation when session does not exist

When there is no managed session yet, creating the session with `tmux new-session -d` also creates its first window.

For `cmdk session window`, that first window should count as the requested new window.

Expected tmux call shape:

```sh
tmux new-session -d -P -F '<session_id>\t<window_id>' \
  -s '<planned session name>' \
  -n '<window name>' \
  -c '<launch path>' \
  [shell-command]
```

Then set session metadata and switch to the returned window.

When the managed session already exists:

```sh
tmux new-window -P -F '#{window_id}' \
  -t '<session_id>:' \
  -n '<window name>' \
  -c '<launch path>' \
  [shell-command]
```

Then switch to the returned window.

## Documentation updates

Update docs in all places that mention connect behavior:

- `README.md`
  - replace `cmdk session connect <path>` reference with `cmdk session window <path> --new`
- `cmd/session.go` help text
- `internal/config/docs.go`
  - remove `SESSION CONNECT`
  - add `SESSION WINDOWS`
  - document examples and command argv behavior
- `cmd/exit_codes.go`
  - likely no behavior change except references if any are added later

Project instruction reminder:

- README is a landing page only.
- Behavior reference belongs in command help and `cmdk docs`.

## Tests

### Unit tests: `cmd/session_test.go`

Add/adjust tests for:

- `session window` requires path.
- `session window <path>` errors with no mode.
- `session window <path> --new` succeeds and calls tmux window function with `NewShell`.
- `session window <path> -- command args` succeeds and passes argv unchanged.
- `--new` plus command errors.
- `--name ""` or equivalent empty name errors if Cobra allows it through.
- JSON/human resolve tests remain.
- Remove connect-specific tests.

### Unit tests: `internal/tmux`

Add tests for:

- Missing managed session creates session with desired window and switches.
- Existing managed session creates a fresh new window and switches.
- Command argv is shell-quoted correctly.
- Shell metacharacters in argv are literal by default.
- Explicit shell usage remains possible, e.g. `sh -lc 'echo hi | tee x'`.
- Duplicate managed sessions fail.
- Malformed tmux output fails.
- Control characters in plan/window name are rejected.
- Existing windows with same name are not searched/reused.

Remove or rewrite connect tests tied to reuse behavior:

- `TestConnectReusesManagedSessionAndExistingWindow`
- `TestConnectCreatesMissingWorktreeWindowInExistingRepoSession`
- duplicate-window-name-first-index behavior for connect reuse

Keep useful parser/runner tests where they still apply.

### Generator/TUI tests

Update tests for built-in dir actions:

- Existing `New window` item remains present with command:

  ```sh
  tmux new-window -c {{sq .path}}
  ```

- New `New session window` item is present with command:

  ```sh
  cmdk session window {{sq .path}} --new
  ```

- Built-in ordering is `New window`, then `New session window`, then config-provided dir actions.
- Tests that previously expected only one built-in dir action should now expect two.

### E2E tests

Update or add:

- `cmdk session window <dir> --new` creates managed session/window metadata and switches when run from active client.
- `cmdk session window <dir> -- command` creates window running command.
- Existing all-window switching remains valid.
- Built-in dir `New window` still creates a plain current-session tmux window.
- Built-in dir `New session window` creates a session-ed window.

Detached e2e tests may still assert creation and metadata before switch failure if needed, but primary expected behavior is inside an active tmux client.

## Security and robustness expectations

- Treat all paths, names, and command args as adversarial.
- Do not build tmux invocations through shell strings.
- Target tmux sessions/windows by IDs after creation/discovery.
- Do not target by session/window display name except for creation fields.
- Reject control characters in plan fields and window names.
- Command args after `--` must be shell-quoted before being used as tmux shell-command.
- Preserve existing template guidance: config commands should use `{{sq .var}}` for external values.

## Non-goals for this phase

- No detached/background mode.
- No automatic task-based window naming beyond optional `--name`.
- No config schema changes.
- No attach-session fallback outside tmux.
- No reuse of existing windows.
- No shell interpretation of command args unless user explicitly invokes a shell.

## Verification

Use project Makefile targets, not raw `go` commands.

Recommended after implementation:

```sh
make fmt
make test
```

For full validation if practical:

```sh
make check
```
