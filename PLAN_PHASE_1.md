# PLAN PHASE 1: Existing tmux sessions in cmdk

## Goal

Add existing tmux sessions to the current `cmdk` fuzzy picker without creating or resolving new sessions yet.

Phase 1 makes `session` a first-class item type and allows session-scoped actions. It should work for every tmux session, including sessions created outside of `cmdk`.

## Locked decisions

- User-facing top-level object: **tmux session**.
- Config match type: `session`.
- Session classification for this phase: `external` only.
- Session child order:
  1. `Connect`
  2. user-defined session actions
  3. windows in that session
- The only built-in session child action is `Connect`.
- Use tmux session IDs for targeting when available.
- Preserve slashes in session names.
- Resolver, metadata writing, repo sessions, directory sessions, agents, and sidebar/tree views are out of scope.

## User experience

Root picker adds a `Sessions` group:

```text
Sessions
  tmux: dotfiles
  tmux: cmdk
  tmux: scratch
Windows
  tmux: dotfiles:1 main
Directories
  ~/Code/github.com/jmcampanini/dotfiles/main
Actions
  dotfiles claude
```

Selecting a session drills into a child list:

```text
Connect
<user-defined session actions>
window 1 main
window 2 wt-feature
window 3 pr-123
```

Quick path:

```text
Enter → Enter
```

selects a session, then immediately selects `Connect`.

## Data model

Add item type:

```text
session
```

Session item data:

```text
session_attached
session_display
session_id
session_kind=external
session_name
session_windows
```

`session_name` is the tmux session name.
`session_id` is the tmux session ID, e.g. `$1`, and should be preferred for targeting.
`session_kind=external` means cmdk has not classified the session as repo/directory yet.

## Tmux source

Add a tmux session source using:

```sh
tmux list-sessions -F '#{session_id}	#{session_name}	#{session_windows}	#{session_attached}'
```

Parsing rules:

- Skip empty lines.
- Skip malformed lines defensively.
- Preserve session names exactly as tmux reports them.
- Sort sessions by session name for now.
- Display can start as `tmux: <session_name>`.

## Session child generator

Add a generator mapped from item type `session`.

It should produce:

1. Built-in `Connect` item.
2. User-defined actions where `matches = "session"`.
3. Windows belonging to the selected session.

### Built-in Connect item

Display:

```text
Connect
```

Command should prefer `session_id`:

```sh
tmux switch-client -t '{{.session_id}}'
```

If `session_id` is missing, exact-match by session name:

```sh
tmux switch-client -t '={{.session_name}}'
```

Implementation can generate one command template that handles both cases, or guarantee `session_id` from the source.

### User-defined session actions

Extend config validation so `matches = "session"` is valid.

Session action template variables should include:

```text
session_attached
session_display
session_id
session_kind
session_name
session_windows
pane_id
```

### Windows in selected session

Reuse or extend tmux window listing logic so the session child generator can list only windows for the selected session.

Window child items should execute the existing switch-window behavior.

## Documentation updates

Update `internal/config/docs.go`:

- `actions.matches` includes `session`.
- Template variables include session variables.
- Add a small session action example.

Do not expand the README unless installation/quickstart changes.

## Tests

Add or update tests for:

- Parsing `tmux list-sessions` output.
- Empty/malformed session output.
- Session item data fields.
- Config validation accepts `matches = "session"`.
- Config validation rejects unknown match types as before.
- Session child generator ordering:
  1. Connect
  2. configured session actions
  3. windows
- Connect command targets `session_id`.
- Group ordering includes sessions before windows/directories/actions.

## Acceptance criteria

- Running `cmdk` shows existing tmux sessions in a `Sessions` group.
- Selecting a session shows `Connect` first.
- Selecting `Connect` switches to the session.
- User-defined `matches = "session"` actions appear after `Connect`.
- Windows for the selected session appear after session actions.
- Existing window, directory, and root action behavior is preserved.
- `make check` passes.

## Non-goals

- No new session creation.
- No repo/directory resolver.
- No `@cmdk_session_*` metadata writing.
- No git metadata.
- No agent metadata.
- No sidebar/tree panel.
- No previews.
