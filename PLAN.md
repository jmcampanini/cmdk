# Plan: Remove `cmd` as an Item Type

## Goal

Remove `cmd` as a launcher item type while keeping `cmd` / `Cmd` terminology where it means a shell command string.

The intended mental model after this change:

- `action` = user-visible launcher operation
- `Cmd` / `cmd` = shell command text run by an action/item
- unknown/default item type = explicit fallback presentation, not `cmd`

## Decisions

### In scope

- Remove `cmd` as an `Item.Type` category.
- Remove all `Type: "cmd"` references from production code and tests.
- Do not keep a `cmd` compatibility alias.
- Rename presentation internals:
  - `theme.TypeCmd` → `theme.TypeAction`
  - `iconCmd` → `iconAction`
- Add explicit unknown/default presentation:
  - icon: `` / `nf-cod-symbol_misc` / `\ueb63`
  - color: muted neutral
- Use the same `` glyph for default action icons, but with action color.
- Picker items should explicitly use unknown/default styling for now.
- Error rows should use a red error/X icon and `theme.Error` color.
- Loading rows should use `` / `nf-oct-hourglass` / `\uf4e3`, colored by source type when available.

### Out of scope

Do not rename or remove `cmd` when it means shell command text:

- TOML config field: `cmd = "..."`
- `config.Action.Cmd`
- `item.Item.Cmd`
- `execute.RenderCmd`
- docs/help text that refers to shell commands
- log fields such as `cmd`

Do not touch framework/convention names:

- `cmd/` package
- Cobra `cmd *cobra.Command`
- Bubble Tea `tea.Cmd`
- stdlib `exec.Cmd`
- `CMDK_*` environment variables

## Source-resolved facts

- `internal/config/items.go`: config actions already become `Item.Type = "action"` and carry shell text in `Item.Cmd`.
- `internal/generator/actions.go`: built-in and configured directory actions are emitted as `Type = "action"`.
- `internal/generator/root.go`: loading items carry `Data["source_type"]`, so loading presentation can derive color from the source type.
- Current grep results: remaining `Type: "cmd"` uses are in ordering/icons/test fixtures, not normal production emitters.
- `internal/config/docs.go`: `cmd` is documented as the shell command field, so it should remain.

## Implementation steps

### 1. Update theme naming

File: `internal/theme/theme.go`

- Rename `TypeCmd` to `TypeAction`.
- Add `TypeUnknown` with muted neutral values:
  - light: use the existing neutral overlay-ish color, likely `#8c8fa1`
  - dark: use the existing neutral overlay-ish color, likely `#7f849c`
- Preserve the existing action color values currently used by `TypeCmd`.

Expected shape:

```go
TypeWindow  color.Color
TypeDir     color.Color
TypeAction  color.Color
TypeUnknown color.Color
Bell        color.Color
Error       color.Color
```

### 2. Update TUI item presentation

File: `internal/tui/delegate.go`

- Rename `iconCmd` to `iconAction`.
- Change the action icon glyph to `\ueb63` (``, `nf-cod-symbol_misc`).
- Add:
  - `iconUnknown = "\ueb63"`
  - `iconLoading = "\uf4e3"`
  - `iconError = "\uea87"` or another red X/error glyph from the existing icon set
- Remove the `"cmd"` entry from the icon map.
- Map known types explicitly:
  - `window` → existing window icon, `TypeWindow`
  - `dir` → existing dir icon, `TypeDir`
  - `action` → `iconAction`, `TypeAction`
  - `pick` → `iconUnknown`, `TypeUnknown`
  - `error` → `iconError`, `Error`
  - `loading` → `iconLoading`, source-derived color
- Add explicit fallback for unknown item types:
  - icon: `iconUnknown`
  - color: `TypeUnknown`

For loading color:

- If `it.Type == "loading"`, inspect `it.Data["source_type"]`.
- Use the same color that source type would use:
  - `window` → `TypeWindow`
  - `dir` → `TypeDir`
  - `action` → `TypeAction`
  - unknown/missing source type → `TypeUnknown`

### 3. Update inline action fallback icon

File: `internal/tui/inline.go`

- Replace `iconCmd` with `iconAction`.
- Preserve current behavior: inline action children keep parent type for sorting, but get an action-ish icon when they do not define a custom icon.

### 4. Remove `cmd` from item ordering

File: `internal/item/group.go`

- Change:

```go
var TypeOrder = []string{"action", "cmd", "dir", "window"}
```

- To:

```go
var TypeOrder = []string{"action", "dir", "window"}
```

Unknown/custom types should still fall through to the existing unknown-type ordering behavior at the end.

### 5. Update tests

Replace all item-type `cmd` fixtures with current concepts.

Likely files:

- `internal/item/group_test.go`
  - Remove tests specifically asserting `cmd` ordering.
  - Replace generic command fixtures with `action` where testing current behavior.
  - Use `custom` / `alien` where testing unknown fallback ordering.
- `internal/tui/delegate_test.go`
  - Rename tests from cmd-icon language to action/default/unknown language.
  - Add/adjust tests for:
    - action icon/color path
    - unknown fallback uses `iconUnknown`
    - picker uses unknown/default styling without logging as unknown
    - error uses error icon/color
    - loading uses hourglass and source-derived color
- `internal/tui/model_test.go`
  - Replace test registry item `Type: "cmd"` with `Type: "action"`.
- `internal/generator/root_test.go`
  - Replace synthetic `commands`/`cmd` source fixtures with `actions`/`action`, or use `custom` only when testing generic source preservation.

After cleanup, `rg 'Type:\s*"cmd"|"cmd"\s*:' internal e2e` should not find item-type uses. Remaining `cmd` results should be shell-command fields, Go variables, package names, or config docs.

### 6. Documentation review

No major user-facing docs change is expected because `cmd` as an item type is not documented as public behavior.

Review docs/help only to ensure wording still distinguishes:

- action = launcher operation
- command/cmd = shell command text

Do not rename the config field in `internal/config/docs.go`.

### 7. Verification

Use make targets only.

- Run `make help` to confirm available targets.
- Run the project’s standard formatting/lint/test target(s), likely including unit tests.
- If available/appropriate, run e2e tests after unit-level changes pass.

## Acceptance criteria

- No production code treats `cmd` as an item type.
- No tests use `Type: "cmd"`.
- `action` rows render with `` and action color by default.
- `unknown`/custom rows render with `` and muted neutral color.
- `pick` rows render with unknown/default styling, without relying on unknown fallback logging.
- `loading` rows render with `` and source-derived color when possible.
- `error` rows render with red error/X styling.
- Config `cmd = "..."` and Go `Cmd` fields continue to work unchanged.
