# Plan: action launch modes and session-window defaults

This plan is the implementation contract for making cmdk path-based actions launch in cmdk-managed session windows by default, while preserving an explicit shell escape hatch.

The goal is that an agent can implement the feature with minimal ambiguity.

## Goals

- Make path-based actions session-aware by default.
- Keep non-path/root actions safe and unsurprising.
- Let directory-generating workflows, such as worktree/PR/restore scripts, produce a final directory and let cmdk own tmux session/window creation.
- Preserve shell-template ergonomics for action commands.
- Validate aggressively so implicit behavior stays understandable.
- Keep config structs value-based: no pointer fields.

## Non-goals

- Do not remove arbitrary shell actions.
- Do not infer action intent by parsing `cmd` strings such as `tmux new-window ...`.
- Do not make every root action sessioned by default.
- Do not implement unsafe shell expansion for path fields: no command substitution, backticks, globbing, or word splitting.

## Current baseline

Relevant current behavior:

- Config actions have `name`, `matches`, `cmd`, `icon`, and optional `stages`.
- `matches` is one of `root`, `dir`, or `session`.
- Config action commands are rendered as Go templates and executed via `sh -c`.
- Dir actions receive `{{.path}}` from the selected zoxide/directory item.
- Current execution cwd is inherited from where `cmdk` was launched.
- Existing dir built-ins are:
  - `New window` -> `tmux new-window -c {{sq .path}}`
  - `New session window` -> `cmdk session window {{sq .path}} --new`
- Existing session primitive:
  - `cmdk session window <path> --new` creates an interactive shell window in the managed session for that path.
  - `cmdk session window <path> --name <name> -- <command> [args...]` creates a command window in that managed session.
- Existing session resolver behavior:
  - repo/worktree paths share a managed repo/container session.
  - non-repo directories get one managed session per canonical directory.

## New action fields

Add these fields to `config.Action` and carry them through to `item.Item` as needed:

```go
type Action struct {
    Name          string        `toml:"name"`
    Matches       string        `toml:"matches"`
    Cmd           string        `toml:"cmd"`
    Icon          string        `toml:"icon"`
    LaunchMode    string        `toml:"launch_mode"`
    LaunchPath    string        `toml:"launch_path"`
    LaunchPathCmd string        `toml:"launch_path_cmd"`
    WindowName    string        `toml:"window_name"`
    Stages        []StageConfig `toml:"stages"`
}
```

No pointers. Empty string means unset.

### Field meanings

| Field | Configured values | Unset meaning |
|---|---|---|
| `launch_mode` | `detect`, `session-window`, `shell` | same as `detect` |
| `launch_path` | a path template | no configured path source |
| `launch_path_cmd` | a shell command template that prints a path | no path-producing command |
| `window_name` | a tmux window name template | for session-window, use `{{.launch_basename}}`; for shell, not applicable |

### Important unset distinction

Unset `launch_path` does **not** mean no effective launch path.

An effective launch path may still come from:

- a dir action's selected `{{.path}}`,
- `launch_path_cmd` stdout,
- explicit root/session `launch_mode = "session-window"` using current cwd fallback.

Unset `launch_path` only means the user did not configure a static/template path source.

## Launch modes

### Configured values

- `""` -> detect
- `"detect"`
- `"session-window"`
- `"shell"`

Any other value is a config validation error.

### Effective launch mode detection

If `launch_mode` is unset or `detect`:

1. If `matches = "dir"`, effective launch mode is `session-window`.
2. Else, if `launch_path` is set, effective launch mode is `session-window`.
3. Else, if `launch_path_cmd` is set, effective launch mode is `session-window`.
4. Else, effective launch mode is `shell`.

If `launch_mode` is explicit:

- `session-window` always means `session-window`.
- `shell` always means `shell`.

Conceptual helper:

```go
func effectiveLaunchMode(a Action) LaunchMode {
    switch a.LaunchMode {
    case "", "detect":
        switch {
        case a.Matches == "dir":
            return LaunchModeSessionWindow
        case a.LaunchPath != "" || a.LaunchPathCmd != "":
            return LaunchModeSessionWindow
        default:
            return LaunchModeShell
        }
    case "session-window":
        return LaunchModeSessionWindow
    case "shell":
        return LaunchModeShell
    default:
        // validate before this point
    }
}
```

## Launch path resolution

The effective launch path is resolved after stages complete and before the final action command runs.

### Resolution order

1. If `launch_path_cmd` is set:
   - render `launch_path_cmd` as a Go template using accumulated action/stage data,
   - run it via `sh -c`,
   - parse stdout as exactly one absolute directory path,
   - validate it.
2. Else, if `launch_path` is set:
   - safely expand shell-like path syntax in the configured field text,
   - render it as a Go template,
   - validate the rendered path.
3. Else, if `matches = "dir"`:
   - use the selected directory `{{.path}}`.
4. Else, if effective launch mode is `session-window`:
   - use current working directory from `os.Getwd()`.
   - This is the explicit root/session `launch_mode = "session-window"` fallback.
5. Else:
   - no effective launch path is set.

### Safe expansion for `launch_path`

Implement safe shell-like expansion for the configured `launch_path` field only:

- support leading `~` and `~/`,
- support `$VAR` and `${VAR}` environment variables,
- do **not** support command substitution,
- do **not** support backticks,
- do **not** support globbing,
- do **not** support word splitting.

Recommended order:

1. Expand `~`, `$VAR`, and `${VAR}` in the configured `launch_path` template text.
2. Render the expanded text as a Go template.

This avoids expanding `$` characters that appear in selected paths, stage outputs, or other template-derived data.

Example:

```toml
launch_path = "$HOME/Code/github.com/jmcampanini/dotfiles/main"
```

This should work.

This should **not** execute anything:

```toml
launch_path = "$(touch /tmp/nope)"
```

It should remain literal/invalid rather than being evaluated by a shell.

### `launch_path_cmd` execution contract

`launch_path_cmd` is a shell command template, not a path field. It is rendered and run via `sh -c`, so normal shell semantics apply to the command itself. Use `{{sq ...}}` around template variables.

Execution details:

- It runs after all stages have completed.
- It runs before `cmd` and before `window_name` rendering.
- It runs with the same inherited cwd behavior as current picker source commands.
  - For dir actions, use `{{.path}}` explicitly or `cd {{sq .path}} && ...` if the command needs to run in the selected directory.
- It should use `[timeout].picker` as its timeout.
  - Existing picker source commands already use this timeout category.
  - `0` should mean no timeout, consistent with picker sources.
- It should receive `CMDK_*` env vars for data available at that point, but `CMDK_LAUNCH_PATH` and `CMDK_LAUNCH_BASENAME` are not available until after it returns.

Stdout parsing:

- stdout must contain one path line.
- allow one trailing newline/CRLF for normal command output.
- reject empty output.
- reject more than one line.
- reject relative paths.
- reject paths that do not resolve to an existing directory.
- do not shell-expand stdout.

Nonzero exit behavior:

- return an action execution error.
- include stderr in the error when available, following picker source style.
- timeout should be reported clearly.

## Template variables

Add these runtime-provided variables after effective launch path is known:

- `{{.launch_path}}` - final validated launch directory.
- `{{.launch_basename}}` - `filepath.Base(filepath.Clean(launch_path))`.

Availability:

| Template field | Has `launch_path`? | Has `launch_basename`? |
|---|---:|---:|
| stage prompt text/default | no | no |
| stage picker source | no | no |
| `launch_path` | no | no |
| `launch_path_cmd` | no | no |
| final `cmd` | yes, if effective launch path exists | yes, if effective launch path exists |
| `window_name` | yes | yes |

`launch_path` and `launch_basename` must be reserved stage keys so stage output cannot overwrite them.

## Execution semantics

### `session-window`

For effective `launch_mode = "session-window"`:

1. Resolve effective launch path.
2. Resolve the managed session plan using the existing session resolver.
3. Render final `cmd` using accumulated data plus `launch_path`/`launch_basename`.
4. Render `window_name`, or use `{{.launch_basename}}` when unset.
5. Create a fresh tmux command window in the managed session for the launch path.
6. Switch the current tmux client to the new window.

The final command should preserve existing shell-snippet semantics by running through a shell in the new tmux window, conceptually:

```sh
sh -lc '<rendered cmd>'
```

Implementation can use existing `tmux.CreateResolvedSessionWindow` with:

```go
tmux.SessionWindowOptions{
    Name:    renderedWindowName,
    Command: []string{"sh", "-lc", renderedCmd},
    Switch:  true,
}
```

Configured actions should still require non-empty `cmd`.

The built-in interactive shell action can remain an internal exception using the existing `cmdk session window <path> --new` behavior or an internal `NewShell` path.

### `shell`

For effective `launch_mode = "shell"`:

1. Resolve effective launch path if one exists.
2. If an effective launch path exists, `chdir` to it before executing the final shell command.
3. Render final `cmd` using accumulated data plus `launch_path`/`launch_basename` when path exists.
4. Execute via `sh -c` using the existing exec replacement behavior.

`launch_mode = "shell"` means "do not create a session window". It does **not** mean "ignore directory context".

Examples:

```toml
[[actions]]
name = "intellij"
matches = "dir"
launch_mode = "shell"
cmd = "idea ."
```

This runs `idea .` with cwd set to the selected dir.

```toml
[[actions]]
name = "pbtools unwrap"
matches = "root"
cmd = "pbpaste | pbtools-unwrap.sh | pbcopy"
```

This has no effective launch path and runs in inherited cwd.

## Window names

For `session-window` actions:

- If `window_name` is unset, use `{{.launch_basename}}`.
- If `window_name` is set, render it as a Go template after launch path resolution.
- `window_name` may use `{{.launch_path}}`, `{{.launch_basename}}`, stage outputs, and other action data.

Examples:

```toml
window_name = "{{.launch_basename}}"
```

```toml
window_name = "pi:{{.launch_basename}}"
```

Validation:

- rendered window name cannot be empty,
- rendered window name cannot contain control characters.

For `shell` actions:

- `window_name` is not applicable.
- Setting `window_name` when the effective launch mode is `shell` is a config validation error.

## Effective value matrix

| Case | Configured fields | Effective `launch_mode` | Effective `launch_path` | Effective `launch_path_cmd` | Effective `window_name` |
|---|---|---:|---|---|---|
| dir minimal | no launch fields | `session-window` | selected `{{.path}}` | unset | `{{.launch_basename}}` |
| dir explicit detect | `launch_mode = "detect"` | `session-window` | selected `{{.path}}` | unset | `{{.launch_basename}}` |
| dir explicit session | `launch_mode = "session-window"` | `session-window` | selected `{{.path}}` | unset | `{{.launch_basename}}` |
| dir path override | `launch_path = "..."` | `session-window` | rendered/expanded `launch_path` | unset | `{{.launch_basename}}` |
| dir generated path | `launch_path_cmd = "..."` | `session-window` | stdout path | configured command | `{{.launch_basename}}` |
| dir name override | `window_name = "..."` | `session-window` | selected/generated path | maybe set | rendered `window_name` |
| dir shell escape | `launch_mode = "shell"` | `shell` | selected `{{.path}}` as cwd | unset | n/a |
| dir shell elsewhere | `launch_mode = "shell"`, `launch_path = "..."` | `shell` | rendered/expanded `launch_path` as cwd | unset | n/a |
| dir shell generated | `launch_mode = "shell"`, `launch_path_cmd = "..."` | `shell` | stdout path as cwd | configured command | n/a |
| root/session minimal | no path fields | `shell` | unset; inherited cwd | unset | n/a |
| root/session explicit session | `launch_mode = "session-window"` | `session-window` | current cwd fallback | unset | `{{.launch_basename}}` |
| root/session path-aware | `launch_path = "..."` | `session-window` | rendered/expanded `launch_path` | unset | `{{.launch_basename}}` |
| root/session generated path | `launch_path_cmd = "..."` | `session-window` | stdout path | configured command | `{{.launch_basename}}` |
| root/session explicit shell | `launch_mode = "shell"` | `shell` | unset unless path field set | maybe set | n/a |
| root/session shell path-aware | `launch_mode = "shell"`, path field set | `shell` | rendered/generated path as cwd | maybe set | n/a |

Invalid combinations are listed below.

## Validation and errors

### Config-load validation errors

Add validation for:

- invalid `launch_mode`,
- both `launch_path` and `launch_path_cmd` set,
- `window_name` set when effective launch mode is `shell`,
- stage key `launch_path` or `launch_basename`,
- existing action validations still apply:
  - `name` cannot be empty,
  - `cmd` cannot be empty,
  - `matches` must be `root`, `dir`, or `session`,
  - icon validation,
  - stage validation.

Note: `window_name` under detect/root/no-path should be rejected because effective mode is `shell`.

Examples of invalid config:

```toml
[[actions]]
name = "bad"
matches = "root"
launch_mode = "background"
cmd = "echo nope"
```

```toml
[[actions]]
name = "ambiguous"
matches = "dir"
launch_path = "~/foo"
launch_path_cmd = "make-path"
cmd = "echo nope"
```

```toml
[[actions]]
name = "bad name"
matches = "root"
window_name = "unused"
cmd = "echo shell by detect"
```

### Runtime/action execution errors

Add runtime errors for:

- `launch_path` renders empty,
- `launch_path` renders a non-existing path,
- `launch_path` renders a non-directory path,
- `launch_path_cmd` times out,
- `launch_path_cmd` exits nonzero,
- `launch_path_cmd` outputs empty stdout,
- `launch_path_cmd` outputs multiple lines,
- `launch_path_cmd` outputs a relative path,
- `launch_path_cmd` outputs a non-existing or non-directory path,
- `window_name` renders empty,
- `window_name` renders with control characters,
- session resolver errors,
- tmux creation/switch errors,
- shell `chdir` errors.

Error messages should name the field when possible, e.g.:

- `launch_path rendered empty`
- `launch_path is not a directory: /path`
- `launch_path_cmd output must be an absolute path`
- `window_name contains control characters`

If new exit behavior or categories are introduced, update `cmd/exit_codes.go` per project policy. If execution still exits `1` through the existing command error path, document that no exit-code update is needed.

## Surprise prevention rules

Implement these as hard errors or docs-backed constraints as noted:

1. Reserve `launch_path` and `launch_basename` as stage keys.
2. Reject `window_name` for effective `shell` actions.
3. Reject both `launch_path` and `launch_path_cmd` being set.
4. Strongly document that for `session-window`, `cmd` is the payload only.
   - Do not include `tmux new-window` in normal session-window action commands.
5. Document that `shell` means "do not create a session window" while still using the effective launch path as cwd when one exists.
6. Keep recommending `{{sq ...}}` around all path/template variables in shell snippets.
7. Make `launch_path_cmd` stdout strict: one absolute directory path.

## Built-in action behavior

Update dir built-ins so session-window behavior is primary.

Recommended UX:

1. `New window` -> create an interactive shell window in the managed session for the selected directory.
2. `New tmux window` or `New plain tmux window` -> create a normal tmux window in the current session for the selected directory.

This replaces the old presentation where `New window` meant plain tmux and `New session window` meant managed session.

Implementation options:

- Keep the sessioned built-in as a shell command calling `cmdk session window {{sq .path}} --new`, and mark it internally as shell/direct to avoid double wrapping.
- Or extend internal item execution to support a session-window action with `NewShell = true` and no config `cmd`.

Configured actions should still require `cmd`.

Update tests that assert built-in names/order.

## Examples

### Minimal dir action: session-window by default

```toml
[[actions]]
name = "claude"
matches = "dir"
cmd = "direnv exec {{sq .launch_path}} claude"
```

Effective values:

- `launch_mode` -> `session-window`
- `launch_path` -> selected `{{.path}}`
- `window_name` -> `{{.launch_basename}}`

### Dir action with custom window name

```toml
[[actions]]
name = "pi"
matches = "dir"
window_name = "pi:{{.launch_basename}}"
cmd = "direnv exec {{sq .launch_path}} pi"
```

### Dir shell escape hatch

```toml
[[actions]]
name = "intellij"
matches = "dir"
launch_mode = "shell"
cmd = "idea ."
```

Effective values:

- `launch_mode` -> `shell`
- cwd -> selected `{{.path}}`
- no tmux window is created

### Root fixed path action

```toml
[[actions]]
name = "dotfiles pi"
matches = "root"
launch_path = "$HOME/Code/github.com/jmcampanini/dotfiles/main"
cmd = "direnv exec {{sq .launch_path}} pi"
```

Effective values:

- `launch_mode` -> `session-window` because `launch_path` is set
- `launch_path` -> expanded/rendered dotfiles path
- `window_name` -> `main`

### Root current directory session window

```toml
[[actions]]
name = "claude here"
matches = "root"
launch_mode = "session-window"
cmd = "direnv exec {{sq .launch_path}} claude"
```

Effective values:

- `launch_mode` -> explicit `session-window`
- `launch_path` -> current cwd fallback
- `window_name` -> cwd basename

### Worktree creator action

```toml
[[actions]]
name = "pi worktree"
matches = "dir"
launch_path_cmd = "cmdk-pi-worktree-path.sh {{sq .path}} {{sq .phrase}}"
cmd = "direnv exec {{sq .launch_path}} pi"
icon = ":nf-dev-git_branch:"
stages = [
  { type = "prompt", text = "Worktree name:", key = "phrase" },
]
```

Path script contract:

```sh
#!/bin/sh
set -e

path="$1"
phrase="$2"

root=$(grove resolve "$path")
worktree=$(cd "$root" && grove create --from-remote-primary "$phrase")
printf '%s\n' "$worktree"
```

The script does not call `tmux new-window`.

### PR checkout action

```toml
[[actions]]
name = "pi pr"
matches = "dir"
launch_path_cmd = "cmdk-pi-pr-path.sh {{sq .path}} {{sq .picked}}"
cmd = "direnv exec {{sq .launch_path}} pi"
icon = ":nf-dev-github:"
stages = [
  { type = "picker", source = "cd {{sq .path}} && grove pr list --fzf", key = "picked", delimiter = "\t", display = 3, pass = 1 },
]
```

Path script:

```sh
#!/bin/sh
set -e

path="$1"
pr_num="$2"

root=$(grove resolve "$path")
worktree=$(cd "$root" && grove pr checkout "$pr_num")
printf '%s\n' "$worktree"
```

### Restore session action

```toml
[[actions]]
name = "restore claude session"
matches = "root"
launch_path_cmd = "cmdk-restore-session-path.sh {{sq .picked}}"
cmd = "direnv exec {{sq .launch_path}} claude --resume {{sq .picked}}"
icon = ":nf-cod-refresh:"
stages = [
  { type = "picker", source = "cmdk-restore-session-source.sh", key = "picked", delimiter = "\t", display = 1, pass = 2 },
]
```

Path script:

```sh
#!/bin/sh
set -e

session_id="$1"
SESSIONS_FILE="$HOME/.local/state/cc-hooks/sessions.tsv"

cwd=$(awk -F'\t' -v sid="$session_id" '$2 == sid {cwd=$3} END {print cwd}' "$SESSIONS_FILE")
[ -n "$cwd" ] || { echo "session not found: $session_id" >&2; exit 1; }
[ -d "$cwd" ] || { echo "directory no longer exists: $cwd" >&2; exit 1; }
printf '%s\n' "$cwd"
```

## Migration guide

### 1. Rename/avoid old `tmux new-window` wrappers for dir actions

Before:

```toml
[[actions]]
name = "claude"
matches = "dir"
cmd = "tmux new-window -c {{sq .path}} direnv exec {{sq .path}} claude"
```

After:

```toml
[[actions]]
name = "claude"
matches = "dir"
cmd = "direnv exec {{sq .launch_path}} claude"
```

### 2. Convert root fixed-project windows to `launch_path`

Before:

```toml
[[actions]]
name = "dotfiles pi"
matches = "root"
cmd = "tmux new-window -c \"$HOME/Code/github.com/jmcampanini/dotfiles/main\" pi"
```

After:

```toml
[[actions]]
name = "dotfiles pi"
matches = "root"
launch_path = "$HOME/Code/github.com/jmcampanini/dotfiles/main"
cmd = "pi"
```

or, if direnv is desired explicitly:

```toml
cmd = "direnv exec {{sq .launch_path}} pi"
```

### 3. Mark pane/current-context actions as shell

Before:

```toml
[[actions]]
name = "lazygit pane"
matches = "root"
cmd = "tmux split-window -h -t {{sq .pane_id}} -c \"$PWD\" lazygit"
```

After:

```toml
[[actions]]
name = "lazygit pane"
matches = "root"
launch_mode = "shell"
cmd = "tmux split-window -h -t {{sq .pane_id}} -c \"$PWD\" lazygit"
```

This explicit field is optional for root actions with no path fields, because detect already resolves to shell. It is recommended for readability.

### 4. Mark GUI/clipboard/root utility actions as shell when helpful

Root actions without path fields remain shell by detect, so this is optional:

```toml
[[actions]]
name = "pbtools unwrap"
matches = "root"
launch_mode = "shell"
cmd = "pbpaste | pbtools-unwrap.sh | pbcopy"
```

Dir GUI actions should become simpler:

```toml
[[actions]]
name = "intellij"
matches = "dir"
launch_mode = "shell"
cmd = "idea ."
```

### 5. Split worktree/PR scripts into path producers

Before script responsibilities:

1. resolve/create worktree,
2. create tmux window,
3. run tool.

After script responsibilities:

1. resolve/create worktree,
2. print final absolute directory path.

cmdk responsibilities:

1. validate the printed path,
2. resolve the managed session,
3. create the tmux window,
4. run the configured payload command.

### 6. Plain tmux window fallback

If a config action should intentionally create a normal tmux window in the current session, set shell mode explicitly:

```toml
[[actions]]
name = "plain tmux window"
matches = "dir"
launch_mode = "shell"
cmd = "tmux new-window -c {{sq .launch_path}}"
```

## Implementation checklist

### Config

- Add fields to `internal/config/config.go`.
- Add launch mode constants/validation.
- Add `launch_path` and `launch_basename` to reserved stage keys.
- Validate invalid field combinations.
- Update icon resolution if fields ever support inline icons; they should not by default.
- Update `internal/config/docs.go` and docs tests.

### Items/generator

- Add fields to `internal/item.Item` or otherwise carry launch metadata to execution.
- Update `config.Action.ToItem()` to copy launch fields.
- Update dir built-ins and tests:
  - sessioned interactive window should be primary,
  - plain tmux window should remain available as fallback.
- Ensure inline actions preserve launch fields when cloned/expanded.

### Execution

- Refactor `internal/execute.Run` so it:
  - flattens data as today,
  - resolves effective launch mode,
  - resolves effective launch path,
  - adds `launch_path` and `launch_basename` to data/env when available,
  - branches into session-window or shell execution.
- Add helper for safe expansion of `launch_path`.
- Add helper for running/parsing `launch_path_cmd`.
- For shell mode with path, call `os.Chdir(launchPath)` before `syscall.Exec`.
- For session-window mode, call existing session resolver and tmux session-window creation directly rather than shelling out to `cmdk`, except built-ins may keep using the existing CLI as an internal shortcut if desired.

### Documentation

- Update `cmdk docs` output in `internal/config/docs.go`.
- Keep README as landing page only; add only minimal reference pointer if needed.
- Update examples in docs from `tmux new-window -c {{sq .path}} ...` to payload-style session-window examples.
- If process exit behavior changes, update `cmd/exit_codes.go`.

## Testing plan

Use make targets only. Do not use raw `go test` directly.

### Main commands

```sh
make test
make check
```

For formatting after changes:

```sh
make fmt
make fmt-check
```

For module changes, if any:

```sh
make tidy
make tidy-check
```

### Unit tests to add/update

#### Config validation tests

Add tests for:

- default config has empty launch fields,
- valid `launch_mode` values: unset, `detect`, `session-window`, `shell`,
- invalid `launch_mode`,
- both `launch_path` and `launch_path_cmd` set,
- `window_name` rejected for effective shell actions:
  - root + detect + no path,
  - explicit `launch_mode = "shell"`,
- `window_name` allowed for:
  - dir + detect,
  - root + `launch_path`,
  - root + explicit `session-window`,
- stage key `launch_path` rejected,
- stage key `launch_basename` rejected.

#### Config item conversion tests

Add/update tests that `Action.ToItem()` carries:

- `LaunchMode`,
- `LaunchPath`,
- `LaunchPathCmd`,
- `WindowName`,
- existing stages and icons unchanged.

#### Execute helper tests

Add tests for effective launch mode:

- dir minimal -> session-window,
- root minimal -> shell,
- session minimal -> shell,
- root + launch_path -> session-window,
- root + launch_path_cmd -> session-window,
- explicit shell wins,
- explicit session-window wins.

Add tests for launch path resolution:

- dir minimal -> selected path,
- root explicit session-window -> cwd fallback,
- launch_path with `~`, `$HOME`, and `${HOME}` expands safely,
- launch_path template renders after safe expansion,
- launch_path does not expand `$` introduced by template data,
- launch_path empty render errors,
- launch_path non-directory errors,
- launch_path_cmd success parses one absolute path,
- launch_path_cmd empty output errors,
- launch_path_cmd multiple lines errors,
- launch_path_cmd relative path errors,
- launch_path_cmd nonzero includes stderr,
- launch_path_cmd timeout errors.

Add tests for window name:

- unset -> launch basename,
- template can use `launch_basename`,
- empty rendered name errors,
- control characters error.

Add tests for shell execution:

- with launch path, exec function observes cwd changed before exec,
- without launch path, cwd remains inherited,
- env includes `CMDK_LAUNCH_PATH` and `CMDK_LAUNCH_BASENAME` when set,
- env omits those when no launch path.

Add tests for session-window execution with a fake tmux/session creation function:

- resolves final path,
- renders command with `launch_path`,
- passes `[]string{"sh", "-lc", rendered}` as command payload,
- passes rendered/default window name,
- uses existing resolver display options.

Implementation may require dependency injection around session-window creation similar to existing command tests.

#### Generator/built-in tests

Update `internal/generator/actions_test.go` expectations:

- primary built-in name/order should reflect new UX.
- built-in sessioned action creates managed session window.
- plain tmux fallback remains present.
- config dir actions remain after built-ins.
- launch fields on config actions are preserved.

Update e2e tests that currently assert both `New window` and `New session window` names.

#### TUI/inline tests

Update inline action tests to confirm inline-expanded actions preserve launch metadata and `InlineParent` behavior still gives dir path data.

#### Docs tests

Update `internal/config/docs_test.go` for new fields, validation rules, template vars, and examples.

### E2E tests to add/update

Existing e2e tests already exercise tmux and session-window creation. Add/update tests for:

1. Dir config action default session-window:
   - config action with `matches = "dir"`, no `launch_mode`, command writes marker from `pwd`,
   - selecting action creates/uses managed session for selected dir,
   - command runs with cwd = selected dir.
2. Dir shell action cwd:
   - `launch_mode = "shell"`, command writes `pwd` to marker,
   - marker shows selected dir,
   - no new managed session is created for that action.
3. Root `launch_path` default session-window:
   - root action with `launch_path = <temp dir>`, command writes marker,
   - managed session exists for temp dir.
4. Root explicit session-window cwd fallback:
   - launch cmdk from a known cwd,
   - root action with `launch_mode = "session-window"`, no path,
   - managed session exists for cwd.
5. `launch_path_cmd`:
   - action has a path command that prints a temp dir,
   - payload runs there,
   - managed session exists for printed dir.
6. `window_name`:
   - action sets `window_name = "xq-{{.launch_basename}}"`,
   - tmux window name matches.
7. Invalid runtime path:
   - action path command prints relative path or missing path,
   - command exits nonzero with useful error.

E2E tests are part of `make test` today; if any require external tools like zoxide/git/tmux, follow existing test skip patterns.

## Acceptance criteria

The feature is complete when:

- `make test` passes.
- `make check` passes.
- `cmdk docs` accurately documents all new fields, defaults, errors, template variables, and examples.
- Existing config examples can be migrated using the migration guide.
- Dir actions without explicit launch fields create managed session windows by default.
- Root/session actions without path fields remain shell actions by default.
- `launch_mode = "shell"` with a launch path runs in that directory without creating a session window.
- `launch_path_cmd` supports two-phase worktree/PR/restore flows by printing a final absolute directory path.
- `window_name` defaults to launch basename and supports templates.
