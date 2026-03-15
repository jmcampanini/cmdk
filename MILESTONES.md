# cmdk — Implementation Milestones

## Requirements for Every Implementation Plan

**These requirements are non-negotiable. Every implementation plan derived from a milestone below MUST satisfy all of them.**

1. **All verification criteria listed in the milestone MUST be fully implemented, tested, and validated.** No milestone is complete until every verification item has a corresponding passing test or demonstrated scenario.

2. **Unit tests** must cover all core logic: struct construction, template rendering, key normalization, generator output, config parsing, etc. Tests must assert behavior, not just "no panic."

3. **Integration tests** must verify that components work together: generators producing correct items from real or realistic inputs, selection chains accumulating state correctly, execute actions producing correct command strings with correct env vars.

4. **End-to-end (E2E) tmux scenarios** must drive the live binary inside a real tmux session. These scenarios use `tmux send-keys` to type, navigate, and select items, and `tmux capture-pane` to assert on visible output. Every milestone that changes user-facing behavior must include at least one E2E scenario that proves the behavior works against the compiled binary. These tests should be automated and runnable via `go test` (or a test script).

5. **Build verification**: the binary must compile cleanly (`go build`) and all tests must pass (`go test ./...`) before a milestone is considered complete.

6. **No mocks for external commands in E2E tests.** Unit tests may mock external commands (tmux, zoxide), but E2E tests must run against real tmux. If zoxide is not available in the test environment, E2E tests for zoxide-specific behavior should be skipped with a clear message, not faked.

7. **E2E tests must invoke the built binary directly** using `./cmdk --pane-id=<id>` (not through `go run` or indirect means). The `--pane-id` value should come from `tmux display-message -p '#{pane_id}'` or be set to a known test value.

8. **Each milestone must be implemented on a dedicated branch** (e.g. `milestone-1`), created before work begins.

9. **`/finalizePhase` must be run at the end of each implementation step** (it handles committing and cleanup).

---

## - [x] Milestone 1: Project Bootstrap + TUI Shell

**Goal**: Establish the Go project, CLI skeleton, and a working bubbletea TUI that displays a fuzzy-filterable list of hardcoded items, with Escape to quit.

### Scope
- `go mod init github.com/jmcampanini/cmdk`
- Cobra root command (`cmdk`) that launches the bubbletea TUI
- `--pane-id` flag accepted and stored (not yet used)
- `Item` struct defined with all fields: Type, Source, Display, Data, Action, Cmd
- Bubbles `list.Model` displaying a hardcoded set of test items (e.g., 3 fake window items, 2 fake dir items)
- Items grouped by Type in display order (windows, dirs, cmds)
- Fuzzy filtering via bubbles built-in filtering
- Escape quits the TUI cleanly (exit code 0)
- Enter on an item is a no-op for now (logged or ignored)
- Makefile with `build`, `test`, and `lint` targets
- `golangci-lint` configured and passing (standard Go linter)

### Verification Criteria
- `make build` compiles the binary
- `make test` runs `go test ./...`
- `make lint` runs `golangci-lint run` with no findings
- Unit tests: Item struct construction, field access, Action enum values
- Unit tests: grouping/ordering logic produces correct group order
- E2E tmux scenario: launch binary, verify hardcoded items visible via `capture-pane`
- E2E tmux scenario: send filter keystrokes, verify filtered results
- E2E tmux scenario: send Escape, verify process exits
- All tests pass via `go test ./...`

---

## - [x] Milestone 2: Generator Framework + Tmux Window Source + Execute

**Goal**: Introduce the generator abstraction, implement the root generator with live tmux window querying, and wire up the Execute action so selecting a window switches to it.

### Scope
- Generator function signature: `func(accumulated []Item, ctx Context) []Item`
- `Context` struct with PaneID and Config fields
- Generator registry (name → generator function) and type map (item type → generator name)
- Root generator registered as `"root"`, mapped from `""` (empty type)
- Root generator queries tmux for all windows across all sessions (`tmux list-windows -a`)
- Produces window items: type=window, source=tmux, action=Execute, with Cmd template and Data. Display format: `session:window-index window-name`
- Execute action flow: push selected item onto accumulated state, render Cmd template with full accumulated Data, shut down bubbletea, `syscall.Exec` into `sh -c <rendered_cmd>`
- File logging: initialize charmbracelet/log at startup, write errors and warnings to `~/.local/state/cmdk/cmdk.log` (create directory if needed). Log execute actions (rendered command, env vars).
- Replace hardcoded items from Milestone 1 with generator output
- Display ordering: windows sorted by session name then window index

### Verification Criteria
- Unit tests: generator registry — register, lookup by name, lookup by type
- Unit tests: type map dispatch — empty accumulated → root, unknown type → error/fallback
- Unit tests: tmux output parsing — given raw `tmux list-windows` output, verify correct Item list
- Unit tests: window item Display string matches format `session:window-index window-name` (e.g. "main:1 zsh")
- Unit tests: Cmd template rendering with Data map
- Unit tests: execute flow produces correct command and args (mock syscall.Exec)
- Unit tests: execute action pushes item onto accumulated state before rendering Cmd template
- Integration test: root generator with mock tmux output produces correctly ordered window items
- E2E tmux scenario: launch binary in tmux, verify real tmux windows appear in the list
- E2E tmux scenario: select a window, verify tmux switches to it
- All tests pass via `go test ./...`

---

## - [ ] Milestone 3: Selection Chain + Directory Flow

**Goal**: Implement the accumulated state (selection chain), NextList action, back navigation with Escape, the zoxide directory source, and the dir-actions generator — enabling the full two-level drill-down flow.

### Scope
- Accumulated state: a stack of selected Items maintained by the TUI model
- On selecting a NextList item: push item onto stack, look up generator via typeMap using the item's Type, generate and display the new list
- On Escape: pop the last item from the stack, regenerate the previous list. If stack is empty, quit.
- Zoxide directory source: run `zoxide query --list --score`, parse output into dir items (type=dir, source=zoxide, action=NextList)
- Dir items carry `path` in Data
- Dir-actions generator registered as `"dir-actions"`, mapped from type `"dir"`
- Dir-actions generator produces: "New window" item (type=cmd, action=Execute, Cmd=`tmux new-window -c {{.path}}`)
- Display ordering: within root list, directories appear after windows. Zoxide dirs sorted by score descending.

### Verification Criteria
- Unit tests: accumulated state — push, pop, empty check, Data flattening across stack
- Unit tests: NextList dispatch — verify correct generator selected based on item type
- Unit tests: zoxide output parsing with correct ordering
- Unit tests: dir-actions generator produces correct items given accumulated state
- Integration test: full two-level chain — select dir → verify dir-actions list → select action → verify rendered command
- Integration test: back navigation — push, pop, verify correct list regenerated
- E2E tmux scenario: verify both tmux windows and zoxide directories appear in root list
- E2E tmux scenario: select directory → verify dir-actions list with "New window"
- E2E tmux scenario: Escape from dir-actions → verify return to root list
- E2E tmux scenario: select directory → "New window" → verify new tmux window opens at correct path
- E2E tmux scenario: if zoxide unavailable, cmdk still launches with other items (skip if zoxide present)
- All tests pass via `go test ./...`

---

## - [ ] Milestone 4: Config File + Custom Commands + Remaining Directory Sources

**Goal**: Add TOML config parsing, custom command items, and the CWD directory source.

### Scope
- Config file location: `~/.config/cmdk/config.toml`
- Config struct with `Commands` slice (each has `Name` and `Cmd` fields)
- Config passed into Context, available to all generators
- Missing config file: Config is nil, no error, no custom commands shown
- Malformed config file: root generator inserts a non-selectable error item showing the parse error
- Custom command items: type=cmd, source=config, action=Execute, Cmd from config
- CWD directory source: single dir item from `os.Getwd()`, type=dir, source=cwd, action=NextList
- Root generator now aggregates: tmux windows, zoxide dirs, CWD dir, custom commands
- Display ordering: windows → directories (zoxide by score, then CWD) → custom commands

### Verification Criteria
- Unit tests: config parsing — valid TOML, missing file, malformed file
- Unit tests: custom command item construction from config entries
- Unit tests: CWD directory item construction
- Unit tests: root generator ordering with all source types present
- Unit tests: custom command items preserve config.toml definition order
- Integration test: root generator with config produces custom command items in correct position
- Integration test: root generator with 3+ custom commands, verify they appear in config-file order within the commands group
- Integration test: nil config → no custom commands, no errors
- Integration test: malformed config → error item present, other sources still work
- E2E tmux scenario: create temp config with custom commands, verify they appear in list
- E2E tmux scenario: select and execute a custom command
- E2E tmux scenario: launch with no config file, verify no errors
- E2E tmux scenario: launch with malformed config, verify error item but cmdk still usable
- E2E tmux scenario: verify CWD appears in directory list
- E2E tmux scenario: config with 3+ custom commands in specific order, verify they appear in that order via capture-pane
- All tests pass via `go test ./...`

---

## - [ ] Milestone 5: Environment Variables + Error Handling + Final Integration

**Goal**: Implement CMDK_ environment variable threading, source failure error items, exit code propagation, and comprehensive end-to-end validation of the complete system.

### Scope
- Key normalization: replace non-alphanumeric with `_`, uppercase, prefix `CMDK_`
- `--pane-id` value → `CMDK_PANE_ID` env var set during execution
- Accumulated item Data flattened into env vars using normalization (last-write-wins for collisions)
- Env vars set before `syscall.Exec`
- Source failure handling: if tmux query fails, zoxide fails, etc. → insert non-selectable error item in the position where that source's items would appear (e.g., zoxide error in the directories group). Other sources continue normally.
- Exit code propagation: cmdk's exit code matches the executed command's exit code
- Non-selectable items: error items cannot be selected (Enter is a no-op on them)

### Verification Criteria
- Unit tests: key normalization — `path` → `CMDK_PATH`, `window-index` → `CMDK_WINDOW_INDEX`, `my.key` → `CMDK_MY_KEY`
- Unit tests: collision semantics — last-write-wins
- Unit tests: env var map construction from accumulated state
- Unit tests: error item construction from source failures
- Unit tests: error item for a dir source appears in the directories group, not at the end
- Unit tests: pressing Enter on a non-selectable error item returns no action (no execute, no next-list)
- Integration test: full execute flow with env vars — verify complete CMDK_ var set
- Integration test: one source failing doesn't affect others
- E2E tmux scenario: execute command that prints `env | grep CMDK_`, verify expected vars present
- E2E tmux scenario: directory → "New window" → verify CMDK_PATH set in new window
- E2E tmux scenario: make zoxide unavailable via PATH manipulation → verify error item appears between windows and custom commands (in the directories position), other sources work
- E2E tmux scenario: with an error item visible, navigate to it, press Enter, verify list unchanged and cmdk still running
- E2E tmux scenario: full happy-path in `tmux display-popup -E` — filter, select window, verify switch; then select dir, "New window", verify path and env vars
- E2E tmux scenario: run cmdk inside `tmux display-popup -E` using exact production invocation: `tmux display-popup -E './cmdk --pane-id=$(tmux display-message -p "#{pane_id}")'`
- E2E tmux scenario: execute command that exits non-zero, verify exit code propagated
- All tests pass via `go test ./...`

---

## - [ ] Milestone 6: Future Additions

**Goal**: Add hardcoded directory entries from config and other deferred features.

### Scope
- Hardcoded directory entries in config.toml via `[[directories]]` with `path` field
- Hardcoded dir items: type=dir, source=hardcoded, action=NextList
- Ordering: hardcoded dirs appear after zoxide and CWD within the directories group, in config-file order
- Root generator updated to aggregate hardcoded dirs

### Verification Criteria
- Unit tests: parse `[[directories]]` entries from TOML config
- Unit tests: hardcoded dir items preserve config-file definition order
- Unit tests: hardcoded dir items placed after zoxide and CWD in root generator output
- Integration test: root generator with hardcoded dirs produces correctly ordered items
- E2E tmux scenario: config with hardcoded dirs, verify they appear in list in correct position
- E2E tmux scenario: select hardcoded dir → verify dir-actions list works
- All tests pass via `go test ./...`

---

## Milestone Dependency Graph

```
M1 → M2 → M3 → M4 → M5 → M6
```

Each milestone strictly depends on the previous one. No milestone should be started until the previous one is fully verified.
