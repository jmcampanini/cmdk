# PLAN PHASE 2: Session resolver and naming

## Goal

Add a resolver that turns a filesystem path into a `cmdk` session plan.

Phase 2 does not need to create tmux sessions. It should make repo/directory classification, cmdk grouping, and planned tmux naming explicit, testable, and inspectable.

## Locked decisions

- Duplicate Grove-style detection in `cmdk`; do not shell out to `grove`.
- Use user-facing vocabulary `session`, not `workspace`.
- Use `session kind`, not `session type`, because `type` is already overloaded by `item.Type`.
- Do not shadow tmux field names:
  - `session_id` means tmux `#{session_id}` only, e.g. `$0`.
  - `session_name` means tmux `#{session_name}` only.
  - `window_id` means tmux `#{window_id}` only, e.g. `@1`.
  - `window_name` means tmux `#{window_name}` only. Before creation, use `planned_tmux_window_name`.
- Use `session_key` for cmdk's non-tmux grouping key.
- Session kinds:
  - `repo`
  - `directory`
- Existing unclassified tmux sessions remain `external` from phase 1.
- Repo session keys are path-based, not remote-based.
- Directory session keys are absolute directory paths.
- Directory session first/default tmux window name is the directory basename.
- Preserve slashes in planned tmux session names.
- Normalize tmux-problematic characters explicitly instead of relying on tmux silent normalization.

## Terminology

### Existing tmux fields

These names are reserved for actual tmux concepts and should not be reused for cmdk-only identities:

```text
session_id    tmux #{session_id}, such as $0
session_name  tmux #{session_name}
window_id     tmux #{window_id}, such as @1
window_index  tmux #{window_index}
window_name   tmux #{window_name}
session_path  tmux #{session_path}; do not use for cmdk's grouping path
```

### cmdk resolver fields

```text
session_kind                 repo | directory
session_key                  cmdk grouping key; not a tmux ID/name
display_label                human/debug display label derived from path formatting
launch_path                  filesystem path for the initial/current window cwd
planned_tmux_session_name    tmux-safe name cmdk would pass to tmux -s later
planned_tmux_window_name     tmux window name cmdk would pass to tmux -n later
```

`session_key` is the important logical identity for cmdk. It is not a tmux field.

For Grove-style repo/worktree layouts, `session_key` is the absolute directory that contains the worktree directories:

```text
/Users/me/Code/github.com/me/dotfiles/main    -> session_key /Users/me/Code/github.com/me/dotfiles
/Users/me/Code/github.com/me/dotfiles/develop -> session_key /Users/me/Code/github.com/me/dotfiles
```

For standalone git repos where no Grove-style worktree container is detected, `session_key` may be the git worktree top-level path itself.

For non-repo directories, `session_key` is the absolute directory path.

## Resolver command

Add a human-readable debug command:

```sh
cmdk session resolve [path]
```

Default output should be for humans, for example:

```text
session_kind:               repo
session_key:                /Users/jmcampanini/Code/github.com/jmcampanini/dotfiles
display_label:              ~/Code/github.com/jmcampanini/dotfiles
launch_path:                /Users/jmcampanini/Code/github.com/jmcampanini/dotfiles/main
planned_tmux_session_name:  Users/jmcampanini/Code/github_com/jmcampanini/dotfiles
planned_tmux_window_name:   main
```

Optional later flag:

```sh
cmdk session resolve [path] --json
```

`--json` is for tests, scripts, and future automation. It is not required for the initial resolver if it slows the phase down.

## Resolver behavior

Given an input path:

1. Resolve it to an absolute path.
2. Confirm it exists and is a directory.
3. Try to detect if it is inside a git worktree:

   ```sh
   git -C <path> rev-parse --show-toplevel
   ```

4. If inside git, classify as `repo` using that worktree as the anchor.
5. If not inside git, probe Grove-style workspace/container root children:

   ```text
   main
   develop
   master
   ```

6. If any primary-branch child is a valid git worktree, classify as `repo` using that child as the anchor and the input directory as the `session_key`.
7. Otherwise classify as `directory`.

## Repo resolution

For a repo/worktree path, produce:

```text
session_kind=repo
session_key=<absolute cmdk grouping path>
display_label=<display label derived from session_key>
launch_path=<worktree root>
planned_tmux_session_name=<tmux-safe name derived from session_key>
planned_tmux_window_name=<worktree basename>
```

Example:

```text
input:                      ~/Code/github.com/jmcampanini/dotfiles/main
session_key:                /Users/jmcampanini/Code/github.com/jmcampanini/dotfiles
display_label:              ~/Code/github.com/jmcampanini/dotfiles
launch_path:                /Users/jmcampanini/Code/github.com/jmcampanini/dotfiles/main
planned_tmux_session_name:  Users/jmcampanini/Code/github_com/jmcampanini/dotfiles
planned_tmux_window_name:   main
```

### Repo grouping

The resolver should keep all worktrees in the same Grove-style container grouped under one `session_key`.

If the input is already inside a worktree, determine whether the worktree belongs to a detectable container. Detection can use Grove-style sibling/child probing (`main`, `develop`, `master`) and linked-worktree metadata where useful.

If no container can be detected, fall back to the git worktree top-level path as `session_key`. This fallback should be explicit in tests.

Git remotes are not the session identity in Phase 2. Remote URL normalization is not required for the resolver key.

## Directory resolution

For a non-repo directory path, produce:

```text
session_kind=directory
session_key=<absolute directory path>
display_label=<shortened/display path>
launch_path=<absolute directory path>
planned_tmux_session_name=<tmux-safe name derived from session_key>
planned_tmux_window_name=<basename>
```

Example:

```text
input:                      ~/Downloads/scratch
session_key:                /Users/jmcampanini/Downloads/scratch
display_label:              ~/Downloads/scratch
launch_path:                /Users/jmcampanini/Downloads/scratch
planned_tmux_session_name:  Users/jmcampanini/Downloads/scratch
planned_tmux_window_name:   scratch
```

Use existing `cmdk shorten` / `pathfmt` logic for `display_label` only, not canonical identity.

## Tmux-safe naming

Known tmux behavior from local testing on tmux 3.6b:

```text
requested: github.com/jmcampanini/dotfiles
actual:    github_com/jmcampanini/dotfiles

requested: foo.bar
actual:    foo_bar

requested: foo:bar
actual:    foo_bar
```

Rules:

- Preserve slashes.
- Normalize `.` to `_`.
- Normalize `:` to `_`.
- Consider normalizing other target-syntax-problematic characters only when tests prove the need.
- Derive `planned_tmux_session_name` from `session_key`.
- Keep `session_key` separate from `planned_tmux_session_name`.
- Do not use `session_id` for cmdk's key; `session_id` is tmux-only.

## Internal API shape

A possible resolver result:

```go
type Plan struct {
    DisplayLabel           string
    InputPath              string
    LaunchPath             string
    PlannedTmuxSessionName string
    PlannedTmuxWindowName  string
    SessionKey             string
    SessionKind            string
}
```

Field names can change, but these concepts should remain distinct. Avoid fields named `SessionID`, `SessionName`, or `SessionPath` unless they represent actual tmux fields.

## Documentation updates

Add command help for:

```sh
cmdk session resolve [path]
```

Document:

- `session_key` is cmdk's grouping key, not tmux `session_id`.
- Repo sessions use a path-based `session_key` so sibling worktrees share one cmdk session.
- Directory sessions use absolute path identity and basename planned tmux window names.
- `display_label` may be shortened, but identity is not.
- `planned_tmux_session_name` is what cmdk would pass to tmux later; Phase 2 does not create tmux sessions.

Keep config docs aligned with tmux naming:

- Template data from tmux window items should use `session_name`, not `session`.
- Reserved stage keys should include `session_id`, `session_name`, `window_id`, `window_index`, and `window_name`.
- Reserved stage keys should not include bare `session`.

## Tests

Add tests for:

- Path does not exist.
- Path exists but is not a directory.
- Inside normal git repo.
- Inside linked git worktree.
- Grove-style container root containing `main`.
- Non-git directory.
- Standalone repo fallback where no Grove-style container is detected.
- `session_key` grouping for `main` and `develop` under the same container.
- Tmux-safe naming preserves slash and normalizes `.`/`:`.
- Directory basename planned tmux window naming.
- Display label shortening uses existing path formatting rules where applicable.
- Resolver output does not use `session_id` for cmdk identity.

## Acceptance criteria

- `cmdk session resolve [path]` prints a correct human-readable session plan.
- Repo/worktree paths resolve to a stable `session_key`.
- Worktree paths produce the repo session key and planned tmux window name.
- Workspace/container paths are detected with Grove-style child probing.
- Non-repo directories resolve as directory sessions with basename planned tmux window names.
- No tmux sessions are created in this phase.
- Existing picker behavior from phase 1 is preserved.
- Existing tmux template data uses `session_name` for tmux `#{session_name}` and `session_id` only for tmux `#{session_id}`.
- `make check` passes.

## Non-goals

- No tmux session creation.
- No metadata writing.
- No automatic palette integration for creating repo/directory sessions.
- No git remote normalization for session identity.
- No git branch/window metadata.
- No agent metadata.
- No sidebar/tree view.
