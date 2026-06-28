# PLAN PHASE 3: Create and connect resolved sessions

## Goal

Use the phase 2 resolver to create or connect tmux sessions from directories and repositories.

Phase 3 turns path-like choices into session-aware navigation:

```text
path → session plan → create/switch session → create/switch default window
```

## Pre-step: Align resolver naming

Before adding connect behavior, align resolver and metadata field names so cmdk logical identity is consistently called a session key, not a session ID:

- Rename `DisplayLabel` to `SessionDisplay` in the resolver plan.
- Rename CLI/JSON output field `display_label` to `session_display`.
- Keep `SessionKey` / `session_key` as the name for cmdk's logical session identity.
- Keep session keys as canonical absolute paths from the phase 2 resolver. Examples may be abbreviated for readability, but implementation must not change identity semantics.
- Do not use `session_id` for cmdk's logical identity; reserve "session ID" for tmux runtime IDs like `$1`.
- Use `session_display` as the value for `@cmdk_session_display` in Phase 3 metadata.
- Use `session_key` as the value for `@cmdk_session_key` in Phase 3 metadata.
- Update resolver tests, `cmdk session resolve` help/output tests, and docs references accordingly.

This pre-step should not change resolver behavior, identity, or display formatting. It intentionally changes user-visible `cmdk session resolve` field names (`display_label` becomes `session_display`, including JSON output); that compatibility break is acceptable.

## Locked decisions

- Repo path creates/connects one session per repo.
- Repo worktree creates/connects a window inside that repo session.
- Directory path creates/connects one session for the directory.
- Directory first/default window name is the directory basename.
- Do not auto-open subdirectories as windows.
- Use minimal session metadata only.
- Set minimal session metadata only when cmdk creates a new tmux session; do not refresh it on reconnect.
- `cmdk session connect` requires an explicit `<path>`; do not default to cwd or pane cwd.
- Phase 3 is CLI-only; picker integration is deferred.
- Defer git metadata, window-scoped metadata, agent metadata, picker integration, and sidebar.

## User experience

Phase 3 exposes the core operation as a CLI command only:

```sh
cmdk session connect <path>
```

The path is required. It must resolve to an existing directory, matching the explicit-path behavior of `cmdk session resolve <path>`. There is no implicit current-directory or pane-cwd default in Phase 3.

Picker integration is deferred. Directory selections and existing built-in dir actions remain unchanged for now; a later phase can add a `Connect session` action beside `New window` or make session-aware navigation the default.

## Command behavior

Add:

```sh
cmdk session connect <path>
```

The path is required.

Behavior:

1. Resolve path using phase 2 resolver.
2. Validate resolver outputs before tmux operations. Reject values with control characters in identity, metadata, cwd, session name, or window name fields.
3. Find an existing cmdk-managed tmux session by matching `@cmdk_session_key` to the resolver's `session_key`.
4. If no matching cmdk-managed session exists, create a new tmux session using the planned tmux-safe session name, capture tmux `session_id` and initial `window_id`, and set minimal session metadata on that newly created session only.
5. If a matching cmdk-managed session exists, use its tmux `session_id` and leave existing metadata unchanged.
6. Ensure the target/default window exists. Match existing windows by exact name; if creating a missing window, capture its tmux `window_id`.
7. Switch the current client to the chosen window using tmux `session_id` + `window_id`.

The session key is the identity. The tmux session name is a creation/display handle, not the source of truth. Do not adopt old/manual tmux sessions that only happen to have the planned name but lack matching `@cmdk_session_key`; if such a name collision prevents creation, surface a clear error.

`connect` requires a current tmux client for the final `switch-client`. It does not preflight this requirement and does not fall back to `attach-session` outside tmux. If switching fails, surface tmux's stderr clearly; the session/window creation steps may already have succeeded.

## Repo connect behavior

For a worktree path:

```text
~/Code/github.com/jmcampanini/dotfiles/main
```

Resolve (shown with canonical absolute identity paths; `session_display` may be shortened by display config):

```text
session_kind=repo
session_key=/Users/jmcampanini/Code/github.com/jmcampanini/dotfiles
session_display=~/Code/github.com/jmcampanini/dotfiles
session_name=Users/jmcampanini/Code/github_com/jmcampanini/dotfiles
window_name=main
window_path=/Users/jmcampanini/Code/github.com/jmcampanini/dotfiles/main
```

Then:

- Reuse an existing cmdk-managed tmux session whose `@cmdk_session_key` matches `session_key`.
- Create tmux session if no matching cmdk-managed session exists.
- Initial window should use `window_name` and `window_path`.
- If session exists but window is missing, create the window in that session.
- Switch to that window.

For another worktree:

```text
~/Code/github.com/jmcampanini/dotfiles/wt-feature
```

Use the same session, with window `wt-feature`.

## Directory connect behavior

For a non-repo directory:

```text
~/Downloads/scratch
```

Resolve:

```text
session_kind=directory
session_key=/Users/jmcampanini/Downloads/scratch
session_display=~/Downloads/scratch
session_name=<tmux-safe directory session name>
window_name=scratch
window_path=/Users/jmcampanini/Downloads/scratch
```

Then:

- Reuse an existing cmdk-managed tmux session whose `@cmdk_session_key` matches `session_key`.
- Create tmux session if no matching cmdk-managed session exists.
- Initial/default window is named `scratch`.
- Switch to the session/window.
- Do not create windows for subdirectories.

## Minimal metadata

Set these session-scoped tmux options on cmdk-managed sessions:

```text
@cmdk_session_kind
@cmdk_session_key
@cmdk_session_display
```

Do not set these yet:

```text
@cmdk_git_*
@cmdk_agent_*
window-scoped @cmdk_*
```

Metadata is for classification, display, and cmdk-managed session identity. `@cmdk_session_key` is required to recognize an existing cmdk-managed session. Tmux runtime IDs/names are still used for precise targeting after a matching session is found or created.

Set metadata only when cmdk creates a new tmux session. When reconnecting to an existing cmdk-managed session found by `@cmdk_session_key`, do not refresh or repair `@cmdk_session_kind`, `@cmdk_session_key`, or `@cmdk_session_display` in Phase 3.

Do not set `@cmdk_session_path` in Phase 3. It is intentionally omitted because `session_key` is the stable identity path/root and `session_display` is the user-facing label; a separate session path field would be ambiguous.

## Tmux commands

Prefer session-key identity and ID-based targeting. Command examples show `\t` / `\n` for readability; implementation should use the existing tmux format helpers/argument arrays rather than shell-assembled strings.

Find an existing cmdk-managed session by listing sessions with metadata and matching `@cmdk_session_key` exactly:

```sh
tmux list-sessions -F '#{session_id}\t#{s|\t|⇥|:#{s|\n|↵|:#{@cmdk_session_key}}}'
```

If exactly one session has matching `@cmdk_session_key`, use its tmux `#{session_id}` for follow-up commands. If more than one session matches the same key, fail clearly rather than choosing arbitrarily. If identity-sensitive tmux output cannot be parsed safely, fail clearly rather than guessing.

Do not use exact planned-name lookup as an adoption path. Planned names are used when creating new sessions; a pre-existing manual session with the same name but no matching key is a collision/error, not a reusable cmdk session.

Create session and capture tmux IDs:

```sh
tmux new-session -d -P -F '#{session_id}\t#{window_id}' -s '<session_name>' -n '<window_name>' -c '<window_path>'
```

Set metadata only on newly created sessions:

```sh
tmux set-option -t '<tmux-session-id>' @cmdk_session_kind '<session_kind>'
tmux set-option -t '<tmux-session-id>' @cmdk_session_key '<session_key>'
tmux set-option -t '<tmux-session-id>' @cmdk_session_display '<session_display>'
```

List windows in an existing session, match exact window name, and choose the first by numeric index when duplicates exist:

```sh
tmux list-windows -t '<tmux-session-id>' -F '#{window_index}\t#{window_id}\t#{s|\t|⇥|:#{s|\n|↵|:#{window_name}}}'
```

Create a missing window and capture its tmux window ID:

```sh
tmux new-window -P -F '#{window_id}' -t '<tmux-session-id>:' -n '<window_name>' -c '<window_path>'
```

Switch by tmux IDs, not names:

```sh
tmux switch-client -t '<tmux-session-id>:<tmux-window-id>'
```

Implementation should avoid shell string construction where possible and use argument arrays. Refactor the shared tmux command runner so tmux stderr is included in non-timeout command errors across tmux operations.

## Window matching

For phase 3, window matching can be simple:

- Prefer exact window name match within the session.
- If multiple windows have the same name, choose the first by numeric index.
- After choosing or creating a window, switch by tmux `window_id`, not by name.
- Later phases can add window-scoped metadata if exact names become insufficient.

## Picker integration

No picker integration in Phase 3.

Keep existing picker behavior unchanged:

- Directory selection still opens the existing dir action list.
- Built-in `New window` remains the only built-in dir action.
- Configured dir actions continue to work as before.

Possible later refinement:

- Add a built-in dir action: `Connect session`.
- Make directory item selection default to session-aware drilldown.
- Add inline session-aware actions if desired.

## Safety and validation

Before any tmux create/match/switch operation, validate the resolver outputs and metadata values that cmdk will write:

- `session_key`
- `session_display`
- `launch_path` / window path
- planned tmux session name
- planned tmux window name

Reject values containing control characters, including tabs, newlines, and carriage returns, with a clear error. This keeps tmux format parsing and metadata matching unambiguous in Phase 3. For tmux list formats, escape tabs/newlines in non-ID fields before field splitting and fail clearly on malformed output rather than guessing.

Keep the existing `TmuxSafeSessionName` naming policy for Phase 3. Add GoDoc documenting that it cleans/slash-normalizes the path, trims leading slashes, replaces `.` and `:` with `_`, falls back to `_` for empty/root names, and is not a uniqueness or identity guarantee. Cmdk identity comes from `@cmdk_session_key`, not the tmux session name.

## Documentation updates

Update config docs/help to describe:

- `cmdk session connect <path>`.
- Repo connect behavior.
- Directory connect behavior.
- Minimal session metadata fields.
- Required current tmux client for `switch-client`; no outside-tmux attach fallback in Phase 3.

Keep README as landing page unless quickstart changes.

## Tests

Add tests for:

- Connect creates a missing repo session.
- Connect creates a missing directory session.
- Connect reuses existing cmdk-managed session by matching `@cmdk_session_key`.
- Connect creates missing worktree window in existing repo session.
- Connect switches to existing window if present.
- Duplicate window names choose the first by numeric index and switch by `window_id`.
- Directory session initial window uses basename.
- Metadata set commands are generated correctly for newly created sessions.
- Metadata is not refreshed when reconnecting to an existing cmdk-managed session.
- Tmux-safe names are used for session creation, with GoDoc documenting their exact behavior and limits.
- Control-character resolver outputs are rejected before tmux operations.
- Tmux `new-session -P -F` and `new-window -P -F` IDs are parsed and used for follow-up targeting.
- Tmux session IDs are used for follow-up targeting after a session-key match or creation.
- Errors from tmux include stderr and are surfaced clearly.

E2E tests should cover at least:

- A temporary non-git directory session.
- A temporary git repo with worktree-like sibling layout, if practical.

## Acceptance criteria

- `cmdk session connect <repo-worktree-path>` creates/switches to a repo session and worktree window.
- `cmdk session connect <non-git-dir>` creates/switches to a directory session and basename window.
- Re-running connect is idempotent for cmdk-managed sessions: it matches `@cmdk_session_key` and switches instead of duplicating sessions/windows.
- Minimal `@cmdk_session_*` metadata is set on cmdk-created sessions only.
- Reconnect does not refresh metadata on existing cmdk-managed sessions.
- Switching targets the chosen tmux `window_id`.
- Existing phase 1 session listing and actions continue to work.
- `make check` passes.

## Non-goals

- No sidebar/tree view.
- No previews.
- No agent session restoration changes.
- No outside-tmux `attach-session` fallback.
- No git branch/PR metadata in tmux options.
- No window-scoped metadata unless simple name matching is proven insufficient.
