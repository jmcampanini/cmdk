# Plan: Keep Filter Textbox Visible During Filter Navigation

## Problem

When the user types a filter query and presses Down, the search textbox disappears even though the list remains filtered. The filter text should remain visible while filtered results are being navigated.

## Observed Cause

- `cmdk` uses Bubble's `list.Model` filtering behavior.
- Bubble maps `down`, `up`, `enter`, `tab`, `shift+tab`, `ctrl+j`, and `ctrl+k` to "accept/apply filter" while filtering.
- Applying the filter changes the list state from `Filtering` to `FilterApplied`.
- `cmdk` renders the filter textbox in the header only when `FilterState() == list.Filtering`.
- Result: pressing Down while filtering applies the filter, exits active filter mode, and hides the textbox.

## Desired Behavior

### Main list and drill-down lists

When the filter query is non-empty:

- `Down` / `Ctrl+J` should move to the next visible filtered result.
- `Up` / `Ctrl+K` should move to the previous visible filtered result.
- The list should remain in active filter mode.
- The filter textbox should remain visible and editable.
- `Enter` should activate the currently highlighted filtered item.

When the effective filter query is empty or whitespace-only:

- Preserve existing behavior.
- `Up`, `Down`, and `Enter` reset/exit filter mode without moving or selecting.

### Picker-stage lists

Apply the same behavior to picker-stage lists:

- Filter textbox remains visible during filtered navigation.
- `Up` / `Down` navigate visible filtered picker results.
- `Enter` chooses the highlighted visible picker result.
- Empty or whitespace-only filter behavior remains unchanged.

## Non-goals

- Do not add a new user configuration option.
- Do not change `start_in_filter`; it should continue to only control whether lists initially open in filter mode.
- Do not change fuzzy matching or ranking behavior.
- Do not change the visual theme except as needed to keep the existing textbox visible.

## Implementation Sketch

1. Add a small helper for active-filter navigation.
   - Detect `FilterState() == list.Filtering`.
   - Require `strings.TrimSpace(FilterInput.Value()) != ""`.
   - Intercept navigation keys before Bubble's default filter-accept behavior runs.

2. Handle keys while filtering:
   - `down`, `ctrl+j`: move the list cursor to the next visible filtered item.
   - `up`, `ctrl+k`: move the list cursor to the previous visible filtered item.
   - `enter`: resolve and activate the currently highlighted visible item.

3. Keep filter mode active:
   - Do not let Bubble's default `AcceptWhileFiltering` path convert the state to `FilterApplied` for these keys.
   - Leave `FilterInput` focused and `FilterState() == list.Filtering`.

4. Apply the helper in both:
   - `updateList`
   - `updatePicker`

5. Preserve current empty/whitespace reset behavior:
   - Keep `resetWhitespaceFilter` behavior for `up`, `down`, and `enter`.
   - Ensure this runs before non-empty filter navigation handling.

## Test Plan

Add/update unit tests under `internal/tui/model_test.go`:

- `Down` during a non-empty filter keeps `FilterState() == list.Filtering`.
- `Down` during a non-empty filter keeps the header textbox visible.
- `Down` moves selection through visible filtered results.
- `Up` moves selection through visible filtered results.
- `Enter` during a non-empty filter with multiple visible items activates the highlighted item.
- Existing single-match Enter behavior still works.
- Existing empty-filter and whitespace-only filter reset tests still pass.
- Picker-stage list has the same Down/Up/Enter behavior.

## Verification

Use project Makefile targets, per project instructions:

```sh
make test
make fmt-check
```

Optionally run the full check suite:

```sh
make check
```
