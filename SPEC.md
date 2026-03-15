# cmdk — Specification

## Context

cmdk is a keyboard-driven tmux launcher. It runs as a TUI inside a tmux popup, presenting fuzzy-filterable lists of items and letting the user drill down through selection chains to execute actions like switching windows, opening directories, or running shell commands.

The motivation is a single, fast `Ctrl-K`-style launcher for all tmux workflows — replacing ad-hoc scripts and manual tmux commands with a composable, extensible tool.

---

## Core Abstractions

### Item Schema

Every item in the system has these fields:

```
Item {
  Type     string              // "window", "dir", "cmd"
  Source   string              // "tmux", "zoxide", "config", "cwd", "hardcoded"
  Display  string              // human-readable, shown in picker, searchable
  Data     map[string]string   // arbitrary KV carried through selection chain
                               // e.g. path=/foo, session=main, window_index=2
  Action   ActionType          // "next-list" | "execute"
  Cmd      string              // Go template string (only when Action=execute)
}
```

- **Type** identifies what kind of thing the item represents. Generators dispatch on this. Also used as the display grouping key — items are grouped by Type in the list.
- **Source** identifies where the item came from (provenance). Multiple sources can produce the same type (e.g. type=dir can come from source=zoxide, source=cwd, or source=hardcoded).
- **Data** is the bag of KV pairs that accumulates through the selection chain and ultimately becomes available as environment variables during execution.
- **Cmd** is a Go `text/template` string rendered with the accumulated Data before execution. For example: `tmux select-window -t {{.session}}:{{.window_index}}`.

### Actions

**NextList**: selecting this item pushes it onto the accumulated state and triggers a generator to produce the next list.

**Execute**: terminal action. The selected item is first pushed onto the accumulated state, then its Cmd template is rendered with the accumulated Data, and cmdk replaces itself (`syscall.Exec`) with `sh -c <rendered command>`. Environment variables from accumulated state are set in the exec environment.

### Selection Chain (Accumulated State)

Every selected item — whether NextList or Execute — is pushed onto the accumulated stack. This stack is available to:
- Generators (to decide what items to produce next)
- Execute actions (as environment variables and template data)

Navigation: **Escape** pops the last item and returns to the previous list. Escape from the top-level list quits cmdk.

---

## Generators

A **Generator** is a function that takes the accumulated items and produces a list of new items to display.

```
Generator: func(accumulated []Item, ctx Context) []Item

Context {
  PaneID string    // from --pane-id flag
  Config *Config   // parsed config.toml, nil if file not found
}
```

Generators are registered by **name**. A separate mapping resolves **item type → generator name**, so selecting an item of type "dir" dispatches to the generator named "dir-actions".

### Generator Registry

```
generators = {
  "root"        → rootGenerator
  "dir-actions" → dirActionsGenerator
}

typeMap = {
  ""    → "root"         // empty accumulated state → root
  "dir" → "dir-actions"  // last item type=dir → dir-actions
}
```

### Root Generator

When accumulated state is empty (base case), the root generator aggregates items from all sources:

1. Collect tmux window items (type=window, source=tmux)
2. Collect zoxide directory items (type=dir, source=zoxide)
3. Collect custom command items (type=cmd, source=config)
4. Merge, grouped by item Type

### Dir-Actions Generator

When the last accumulated item has type=dir, this generator produces action items for that directory:

- v1: "New window" → Execute, Cmd: `tmux new-window -c {{.path}}`
- Roadmap: "New session", "New pane", "Open yazi", etc.

### Extending Generators

Adding a new generator means:
1. Write a function matching the Generator signature
2. Register it by name
3. Map an item type to it (or have items reference it explicitly in the future)

---

## Item Sources

### Tmux Windows
- Query: all windows across all tmux sessions
- Type: `window`
- Source: `tmux`
- Display: `session:window-index window-name`
- Action: **Execute**
- Cmd: `tmux select-window -t {{.session}}:{{.window_index}}`
- Data: `session=<name>`, `window_index=<n>`
- Ordering: by session, then window index

### Directories (type=dir)

Directories can come from multiple sources:

| Source | How collected | Ordering |
|--------|--------------|----------|
| `zoxide` | `zoxide query --list --score` | by score (descending) |
| `cwd` | current working directory (derived via `os.Getwd`) | n/a (single item) |
| `hardcoded` | listed in config.toml | config file order |

All directory items have type=dir and action=NextList, which dispatches to the "dir-actions" generator.

### Custom Commands
- Defined in TOML config file
- Type: `cmd`
- Source: `config`
- Display: user-defined name
- Action: **Execute** — runs the shell command string
- Ordering: config file order

### Top-Level Ordering
Items are grouped by Type: windows first, then directories, then custom commands. Each group determines its own sort order (e.g. windows by session then index, directories by zoxide score).

---

## Environment Variables

When cmdk executes a command, it passes context as environment variables with the `CMDK_` prefix.

### Key Normalization
All Data keys are normalized to valid shell variable names before becoming env vars:
1. Replace any non-alphanumeric character with `_`
2. Uppercase all alphabetic characters
3. Prefix with `CMDK_`

Examples: `path` → `CMDK_PATH`, `window-index` → `CMDK_WINDOW_INDEX`, `my.key` → `CMDK_MY_KEY`

### Collision Semantics
Data from all accumulated items is flattened into a single env var namespace. Later items in the stack override earlier ones (last-write-wins).

### Context Variables (from wrapper script)
- `CMDK_PANE_ID` — current tmux pane ID (from `--pane-id`)

### Accumulated Item Data
Data from accumulated items is flattened into env vars using the normalization rules above. For example:
- A dir item with `path=/foo` → `CMDK_PATH=/foo`
- A window item with `session=main`, `window_index=2` → `CMDK_SESSION=main`, `CMDK_WINDOW_INDEX=2`

### Arbitrary Pass-Through
The wrapper script can pass arbitrary KV pairs (mechanism TBD — likely additional flags or env vars) which cmdk threads through to executed commands.

## Execution

### Shell
Commands are executed via `sh -c <rendered_cmd>`. The shell is `sh` by default but should be easy to make configurable in the future.

### Lifecycle
cmdk runs inside `tmux display-popup -E`. When an execute action is selected:

1. Push the selected item onto the accumulated state
2. Flatten accumulated Data into env vars (with normalization)
3. Render the Cmd template with accumulated Data
4. Shut down the bubbletea TUI
5. `syscall.Exec` to replace the cmdk process with `sh -c <rendered_cmd>`

This means the popup's pseudo-terminal is handed directly to the executed command. For non-interactive commands (e.g. `tmux select-window`), sh runs the command and exits, closing the popup. For interactive commands, the command owns the terminal until it exits.

### Exit Codes
cmdk propagates the executed command's exit code. Under `tmux display-popup -E`, a non-zero exit keeps the popup open showing the error output.

---

## Tech Stack

| Component | Library |
|-----------|---------|
| Language | Go |
| CLI framework | [cobra](https://github.com/spf13/cobra) |
| TUI runtime | [bubbletea](https://github.com/charmbracelet/bubbletea) (required runtime for bubbles components) |
| Fuzzy list picker | [bubbles list.Model](https://github.com/charmbracelet/bubbles) |
| Config parsing | [BurntSushi/toml](https://github.com/BurntSushi/toml) |
| Module path | `github.com/jmcampanini/cmdk` |
| Binary name | `cmdk` |

---

## CLI Interface

```
cmdk              # launches the interactive TUI (root command)
cmdk config       # (future) manage configuration
cmdk debug        # (future) diagnostics
```

Cobra is used with subcommands for future extensibility. The root command launches the picker.

### Flags (passed by wrapper script)
- `--pane-id` — current tmux pane ID

The wrapper script (managed separately, not part of this project) is responsible for collecting tmux context and launching `cmdk` inside `tmux display-popup`.

---

## Config File

Location: `~/.config/cmdk/config.toml`

```toml
[[commands]]
name = "Yazi (home)"
cmd = "yazi ~"

[[commands]]
name = "Yazi (projects)"
cmd = "yazi ~/projects"

[[commands]]
name = "htop"
cmd = "htop"
```

v1 config is minimal — just custom command definitions. If the config file is missing, cmdk runs without custom commands (no error). If the config file exists but is malformed, an error item is shown in the list.

Future config may include:
- Source enable/disable and ordering
- Generator configuration (which actions appear for directory items)
- Display formatting
- Hardcoded directory entries

---

## UX Flow

1. User presses tmux keybinding → wrapper script launches `tmux display-popup -E` running `cmdk --pane-id=...`
2. Root generator aggregates items from all sources → single mixed list with fuzzy filtering
3. User types to filter, navigates with arrow keys, Enter to select
4. On selection:
   - If action is **NextList**: push item to accumulated state, look up generator by item type, render new list
   - If action is **Execute**: push item to accumulated state, render Cmd template with accumulated Data, set env vars, `syscall.Exec` into `sh -c <rendered_cmd>`
5. Escape: pop back to previous list, or quit if at top level

### Example Flow: Open directory in new window

```
[Root list]                          accumulated: []
  ├── main:1 zsh          (window)
  ├── main:2 vim          (window)
  ├── ~/projects/foo       (dir)    ← user selects this
  ├── ~/projects/bar       (dir)
  └── htop                 (cmd)

[Dir-actions list]                   accumulated: [{type=dir, path=~/projects/foo}]
  ├── New window                     ← user selects this
  └── (future: New session, Yazi)

→ Execute: syscall.Exec sh -c "tmux new-window -c ~/projects/foo"
  env: CMDK_PANE_ID=%3, CMDK_PATH=~/projects/foo
```

---

## V1 Scope

### In scope
- Root cobra command launching bubbletea TUI
- bubbles list.Model with fuzzy filtering
- Item schema with Type, Source, Display, Data, Action, Cmd
- Generator abstraction: named registry, type-to-name mapping, Context (PaneID + Config)
- Root generator aggregating sources: tmux windows, zoxide dirs, CWD dir, hardcoded dirs, custom commands
- Dir-actions generator producing "New window" for directory items
- Selection chain with accumulated state (arbitrary depth, tested at 2) — execute items push before executing
- Back navigation via Escape
- Cmd as Go template strings rendered with accumulated Data
- Execution via `syscall.Exec` into `sh -c <rendered_cmd>`
- Two execute actions: switch-to-window, new-window-at-dir
- Custom commands: shell exec from TOML config
- Environment variable threading: CMDK_PANE_ID plus accumulated item data (normalized keys)
- Config file at ~/.config/cmdk/config.toml (missing = no custom commands, malformed = error item)
- Error handling: source failures shown as non-selectable error items in the list

### Roadmap
- **Execute actions**: new-session-at-dir, new-pane-at-dir, open-yazi-at-dir
- **Dir-actions generator expansion**: configurable via TOML (which actions appear for directories)
- **Visual markers**: highlight current window, annotate items
- **Per-item generator override**: optional NextGenerator field on Item to override typeMap dispatch
- **Subcommands**: `cmdk config`, `cmdk debug`
- **Display**: columns, colors, icons via lipgloss
- **Config expansion**: source enable/disable, key bindings, generator configuration
- **Arbitrary KV pass-through**: wrapper script passes extra context to cmdk
- **Configurable shell**: allow overriding `sh` with another shell (e.g. `bash`)

---

## Error Handling

### Source Failures
When a source fails to produce items (e.g. zoxide not installed, tmux query fails), cmdk inserts a non-selectable error item in the list where that source's items would appear. Other sources continue to work normally.

```
[Root list]
  ├── main:1 zsh               (window)
  ├── main:2 vim               (window)
  ├── ⚠ zoxide: command not found  (error, not selectable)
  └── htop                     (cmd)
```

### Config Errors
- Missing config file: silently skip, no custom commands shown
- Malformed config file: show error item in list

### Execution Errors
cmdk propagates the exit code from the executed command. Under `tmux display-popup -E`, non-zero exit keeps the popup open showing the error output.

---

## Verification

1. Build: `go build -o cmdk .`
2. Run directly: `./cmdk --pane-id=$(tmux display-message -p '#{pane_id}')`
3. Verify tmux windows appear in list and are switchable
4. Verify zoxide directories appear (requires zoxide installed with history)
5. Verify selecting a directory shows sub-list with "New window" option
6. Verify custom commands from config file appear and execute
7. Verify Escape navigates back through list chain and quits from top level
8. Verify env vars are set during execution
9. Test in tmux popup: `tmux display-popup -E "./cmdk --pane-id=..."`
10. Verify missing zoxide shows error item but cmdk still works
11. Verify missing config file results in no custom commands (no error)
