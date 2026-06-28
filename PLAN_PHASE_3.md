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
- Do not use `session_id` for cmdk's logical identity; reserve "session ID" for tmux runtime IDs like `$1`.
- Use `session_display` as the value for `@cmdk_session_display` in Phase 3 metadata.
- Use `session_key` as the value for `@cmdk_session_key` in Phase 3 metadata.
- Update resolver tests, `cmdk session resolve` help/output tests, and docs references accordingly.

This is a naming-only pre-step; it should not change resolver behavior, identity, or display formatting.

## Locked decisions

- Repo path creates/connects one session per repo.
- Repo worktree creates/connects a window inside that repo session.
- Directory path creates/connects one session for the directory.
- Directory first/default window name is the directory basename.
- Do not auto-open subdirectories as windows.
- Use minimal session metadata only.
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
2. Find an existing cmdk-managed tmux session by matching `@cmdk_session_key` to the resolver's `session_key`.
3. If no matching cmdk-managed session exists, create a new tmux session using the planned tmux-safe session name.
4. Ensure the target/default window exists.
5. Set minimal session metadata.
6. Switch the current client to the session/window.

The session key is the identity. The tmux session name is a creation/display handle, not the source of truth. Do not adopt old/manual tmux sessions that only happen to have the planned name but lack matching `@cmdk_session_key`; if such a name collision prevents creation, surface a clear error.

## Repo connect behavior

For a worktree path:

```text
~/Code/github.com/jmcampanini/dotfiles/main
```

Resolve:

```text
session_key=github.com/jmcampanini/dotfiles
session_name=github_com/jmcampanini/dotfiles
window_name=main
window_path=~/Code/github.com/jmcampanini/dotfiles/main
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

Do not set `@cmdk_session_path` in Phase 3. It is intentionally omitted because `session_key` is the stable identity path/root and `session_display` is the user-facing label; a separate session path field would be ambiguous.

## Tmux commands

Prefer session-key identity and ID-based targeting.

Find an existing cmdk-managed session by listing sessions with metadata and matching `@cmdk_session_key` exactly:

```sh
tmux list-sessions -F '#{session_id}\t#{session_name}\t#{@cmdk_session_key}'
```

If exactly one session has matching `@cmdk_session_key`, use its tmux `#{session_id}` for follow-up commands. If more than one session matches the same key, fail clearly rather than choosing arbitrarily.

Do not use exact planned-name lookup as an adoption path. Planned names are used when creating new sessions; a pre-existing manual session with the same name but no matching key is a collision/error, not a reusable cmdk session.

Create session:

```sh
tmux new-session -d -s '<session_name>' -n '<window_name>' -c '<window_path>'
```

Create window in existing session:

```sh
tmux new-window -t '<tmux-session-id>:' -n '<window_name>' -c '<window_path>'
```

Switch:

```sh
tmux switch-client -t '<tmux-session-id>:<window-target>'
```

Implementation should avoid shell string construction where possible and use argument arrays.

## Window matching

For phase 3, window matching can be simple:

- Prefer exact window name match within the session.
- If multiple windows have the same name, choose the first by index.
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

## Documentation updates

Update config docs/help to describe:

- `cmdk session connect <path>`.
- Repo connect behavior.
- Directory connect behavior.
- Minimal session metadata fields.

Keep README as landing page unless quickstart changes.

## Tests

Add tests for:

- Connect creates a missing repo session.
- Connect creates a missing directory session.
- Connect reuses existing cmdk-managed session by matching `@cmdk_session_key`.
- Connect creates missing worktree window in existing repo session.
- Connect switches to existing window if present.
- Directory session initial window uses basename.
- Metadata set commands are generated correctly.
- Tmux-safe names are used for session creation.
- Tmux session IDs are used for follow-up targeting after a session-key match or creation.
- Errors from tmux are surfaced clearly.

E2E tests should cover at least:

- A temporary non-git directory session.
- A temporary git repo with worktree-like sibling layout, if practical.

## Acceptance criteria

- `cmdk session connect <repo-worktree-path>` creates/switches to a repo session and worktree window.
- `cmdk session connect <non-git-dir>` creates/switches to a directory session and basename window.
- Re-running connect is idempotent for cmdk-managed sessions: it matches `@cmdk_session_key` and switches instead of duplicating sessions/windows.
- Minimal `@cmdk_session_*` metadata is set on cmdk-created sessions.
- Existing phase 1 session listing and actions continue to work.
- `make check` passes.

## Non-goals

- No sidebar/tree view.
- No previews.
- No agent session restoration changes.
- No git branch/PR metadata in tmux options.
- No window-scoped metadata unless simple name matching is proven insufficient.
