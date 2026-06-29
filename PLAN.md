# Plan: Wrapped, Scrollable TUI Error Details

## Problem

Long error messages that are kept inside cmdk's TUI are currently rendered as normal list items and truncated to one line. This makes picker/source failures with long stderr difficult to read.

Current source-resolved behavior:

- `internal/tui/delegate.go` truncates list item display text with `ansi.Truncate(...)`.
- Existing TUI errors are represented as `item.Item{Type: "error", Display: ...}` in root lists and picker lists.
- Error items are styled with the error icon/color and are non-executable.
- Pressing Enter on an error item is currently a no-op; this behavior will change.
- There is no existing reusable wrapped error view. Prompt validation uses only inline `stageError` text.

## Goals

1. Make long TUI-held errors readable by humans.
2. Preserve compact one-line list rows for normal browsing/filtering.
3. Open a dedicated error details screen when the user presses Enter on a highlighted error item.
4. Word-wrap details to the terminal width.
5. Allow scrolling when the wrapped details exceed the terminal height.
6. Support all existing `Type: "error"` list items in both root and picker lists.
7. Provide deterministic automated and manual tmux-pane repros, including a very long error that proves scrolling works.

## Non-goals

- Do not wrap normal list rows inline.
- Do not introduce a config option for this first pass.
- Do not change prompt validation errors such as `required`.
- Do not catch every post-selection command failure after `syscall.Exec`; those remain outside the TUI for this pass.
- Do not require a tmux popup for verification; a normal tmux pane/session is sufficient.

## User story

1. User launches cmdk in a normal tmux pane or popup.
2. A source/picker/navigation error appears as a red error row.
3. User highlights the error row.
4. User presses Enter.
5. cmdk opens a dedicated `Error details` screen.
6. The full error message is wrapped to the current width.
7. User scrolls with Up/Down, `j`/`k`, PageUp/PageDown, Home/End.
8. User presses Esc to return to the original list or picker.
9. User can continue normal navigation or Esc out as before.

## Error categories in scope

These already become `Type: "error"` list items and should open the details screen:

1. **Root/source errors**
   - Examples: config source error shown in the TUI when the default config is invalid, zoxide/tmux source failures, async source failures.
2. **Picker-stage template errors**
   - `execute.RenderCmd(stage.Source, data)` fails before running the picker command.
3. **Picker-stage command errors**
   - `runPickerSource` command exits non-zero, times out, or writes stderr.
   - This is the primary long-error repro path.
4. **Navigation/registry errors**
   - Drill-down cannot resolve a generator and emits `navigation error: ...`.
5. **Tmux parse error items**
   - Existing tmux parse helpers can produce `Type: "error"` rows.

Out of scope for this pass:

- Prompt inline validation errors (`stageError`, e.g. `required`).
- Errors returned after the TUI exits to render/exec the selected action.
- Cobra/root command errors printed directly to stderr before the TUI starts.

## TUI/API shape

This is an internal TUI behavior change; no public CLI/config API is planned.

### Model state

Extend `internal/tui.Model` with error-details state similar to:

```go
type viewMode int

const (
    viewList viewMode = iota
    viewPrompt
    viewPicker
    viewErrorDetails
)

type Model struct {
    // existing fields...
    errorReturnMode viewMode
    errorDetailItem item.Item
    errorDetailScroll int
}
```

Notes:

- `errorReturnMode` should only be `viewList` or `viewPicker`.
- Keep the underlying `list`/`pickerList` state intact while details are open.
- Do not mutate the selected error item.

### Opening details

Add a small helper:

```go
func (m Model) openErrorDetails(it item.Item) Model
```

Behavior:

- Stores the error item.
- Stores the current return mode (`viewList` or `viewPicker`).
- Resets scroll to `0`.
- Sets `mode = viewErrorDetails`.

Update Enter handling in both `updateList` and `updatePicker`:

- Refresh active filters as today.
- If the highlighted item is `Type: "error"`, open details.
- This should work even while the list/picker is in filtering mode, so a single picker error can be opened with Enter without requiring Esc first.
- Normal item Enter behavior should remain unchanged.

### Updating details

Add:

```go
func (m Model) updateErrorDetails(msg tea.Msg) (tea.Model, tea.Cmd)
```

Key behavior:

- `esc`: return to `errorReturnMode`.
- `ctrl+c`: quit.
- `down`/`j`: scroll down one line.
- `up`/`k`: scroll up one line.
- `pgdown`/`space`: scroll down one page.
- `pgup`: scroll up one page.
- `home`: scroll to top.
- `end`: scroll to bottom.
- Ignore other keys.

Clamp scroll between `0` and max scroll for the current wrapped content height.

### Rendering details

Add:

```go
func (m Model) errorDetailsView() string
```

Rendering intent:

- Reuse existing theme/error color via `m.errorStyle` where appropriate.
- Header line: `Error details`.
- Optional metadata line when present: `Source: <source>`.
- Body: full `errorDetailItem.Display`, preserving embedded newlines and wrapping each line to available width.
- Footer/hint: `Esc back • ↑/↓ scroll • PgUp/PgDn page`.
- Width should be based on `m.winWidth` minus horizontal padding.
- Height should be based on `m.winHeight` minus header/footer lines.
- Body should be clipped to the visible window according to `errorDetailScroll`.
- Do not append ellipses to the details body; scrolling should expose hidden content.

Implementation can use existing ANSI-aware wrapping support from `github.com/charmbracelet/x/ansi` rather than manual rune-width logic.

## Automated verification plan

Use `make` targets for verification.

### Unit tests

Add/update tests in `internal/tui/model_test.go` and `internal/tui/delegate_test.go`.

Suggested test coverage:

1. **Root error opens details**
   - Given a root list containing a `Type: "error"` item.
   - Press Enter.
   - Assert `mode == viewErrorDetails`.
   - Assert details view contains the full error text.

2. **Picker error opens details**
   - Use a picker stage that fails.
   - Press Enter on the picker error item.
   - Assert details mode opens.
   - This should pass even when picker starts in filter mode.

3. **Esc returns to root list**
   - Open details from root list.
   - Press Esc.
   - Assert `mode == viewList` and list selection/filter state still exists.

4. **Esc returns to picker**
   - Open details from picker list.
   - Press Esc.
   - Assert `mode == viewPicker` and picker list still contains the error row.

5. **Details wrap without truncation**
   - Use a long message with spaces and a narrow window.
   - Assert details view has multiple body lines.
   - Assert details view does not contain `…` from truncation.
   - Assert tail text appears after scrolling.

6. **Scrolling clamps correctly**
   - Use a long multi-line error.
   - Press PageDown/End.
   - Assert later lines appear.
   - Press Home.
   - Assert first lines appear again.

7. **Existing list truncation remains for normal rows**
   - Keep/update existing delegate truncation tests for non-error list rows.
   - Error rows in the list may remain one-line/truncated; details is where full text appears.

Run:

```sh
make test
```

### E2E tmux-pane test

Add an e2e test that drives the full user story in a detached normal tmux session using existing helpers (`tmux new-session`, `send-keys`, `capture-pane`). This does not require a popup.

Suggested flow:

1. Build/test binary via the existing test harness or `make build` for manual runs.
2. Create a temp `XDG_CONFIG_HOME/cmdk/config.toml` with one root action named `Long Picker Error`.
3. The action has a picker stage whose `source` runs a temp shell script.
4. The script writes 100+ long stderr lines and exits non-zero.
5. Start `cmdk` in a normal tmux pane/session with that temp config home.
6. Type/filter to `Long Picker Error` and press Enter.
7. Wait for the picker error row.
8. Press Enter on the error row.
9. Assert the pane shows `Error details`, early stderr lines, and wrapped text.
10. Send PageDown/End.
11. Assert later stderr lines are visible, proving scrolling.
12. Press Esc.
13. Assert the picker error row is visible again.

Run through the normal project target:

```sh
make test
```

If a focused/manual e2e run is needed, prefer adding a make target later rather than documenting raw `go test` commands.

## Long-error repro design

Use a generated shell script in tests/manual repros to avoid brittle TOML quoting.

Example script body:

```sh
#!/bin/sh
i=1
while [ "$i" -le 140 ]; do
  printf 'ERR %03d this is a deliberately very long stderr line for cmdk wrapping and scrolling verification with enough words to wrap naturally across a narrow pane; token=%s\n' "$i" "abcdefghijklmnopqrstuvwxyz0123456789abcdefghijklmnopqrstuvwxyz0123456789" >&2
  i=$((i + 1))
done
exit 7
```

Config action shape:

```toml
[[actions]]
name = "Long Picker Error"
matches = "root"
cmd = "true"
stages = [
  { type = "picker", key = "choice", source = "/absolute/path/to/long-picker-error.sh" },
]
```

Expected picker row before opening details:

- Starts with something like `command error: command failed: exit status 7`.
- May be one-line/truncated in the list.

Expected details screen:

- Shows `Error details`.
- Shows the command error and stderr content.
- Shows early lines such as `ERR 001` initially.
- After scrolling/end, shows later lines such as `ERR 140`.
- Does not rely on horizontal truncation/ellipsis for readability.

## Manual verification in a normal tmux pane

After implementation:

```sh
make build
```

Then create a temp repro directory:

```sh
tmp="$(mktemp -d)"
mkdir -p "$tmp/xdg/cmdk"
cat > "$tmp/long-picker-error.sh" <<'SH'
#!/bin/sh
i=1
while [ "$i" -le 140 ]; do
  printf 'ERR %03d this is a deliberately very long stderr line for cmdk wrapping and scrolling verification with enough words to wrap naturally across a narrow pane; token=%s\n' "$i" "abcdefghijklmnopqrstuvwxyz0123456789abcdefghijklmnopqrstuvwxyz0123456789" >&2
  i=$((i + 1))
done
exit 7
SH
chmod +x "$tmp/long-picker-error.sh"
cat > "$tmp/xdg/cmdk/config.toml" <<EOF
[[actions]]
name = "Long Picker Error"
matches = "root"
cmd = "true"
stages = [
  { type = "picker", key = "choice", source = "$tmp/long-picker-error.sh" },
]
EOF
```

Launch in a normal tmux session/pane:

```sh
tmux new-session -s cmdk-error-repro -x 100 -y 30 "XDG_CONFIG_HOME=$tmp/xdg ./build/cmdk --pane-id=%0"
```

Manual steps:

1. Type enough of `Long Picker Error` to highlight the action.
2. Press Enter.
3. The picker should show an error row.
4. Press Enter again.
5. The wrapped `Error details` screen should open.
6. Use PageDown/End to verify later `ERR NNN` lines appear.
7. Press Esc to return to the picker error row.

Optional capture artifacts:

```sh
tmux capture-pane -ep -t cmdk-error-repro > "$tmp/error-details.ansi"
tmux capture-pane -p  -t cmdk-error-repro > "$tmp/error-details.txt"
```

## Completion criteria

- `make test` passes.
- `make check` passes before final handoff if lint tooling is available.
- Manual tmux-pane repro opens the details screen and shows wrapped long stderr.
- Scrolling exposes both early and late long-error lines.
- Esc returns to the original list/picker.
- Normal item selection, filtering, prompt stages, and picker selection remain unchanged.
