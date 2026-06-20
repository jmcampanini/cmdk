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

Use picker-style behavior, matching list pickers such as Snacks picker:

- While the filter textbox is focused, `Down` / `Ctrl+J` move to the next visible result.
- While the filter textbox is focused, `Up` / `Ctrl+K` move to the previous visible result.
- The list remains in active filter mode while navigating.
- The filter textbox remains visible and editable while navigating.
- `Enter` activates the currently highlighted visible item.
- An empty or whitespace-only effective filter is treated as an empty fuzzy pattern: all visible items are navigable, and `Enter` chooses the highlighted item.

### Main list and drill-down lists

Apply the picker-style behavior to root and drill-down lists.

### Picker-stage lists

Apply the same behavior to picker-stage lists:

- Filter textbox remains visible during navigation.
- `Up` / `Down` navigate visible picker results.
- `Enter` chooses the highlighted visible picker result.
- Empty or whitespace-only input navigates/selects visible picker results instead of resetting the filter.

## Non-goals

- Do not add a new user configuration option.
- Do not change `start_in_filter`; it should continue to only control whether lists initially open in filter mode.
- Do not change fuzzy matching or ranking behavior.
- Do not change the visual theme except as needed to keep the existing textbox visible.

## Implementation Sketch

1. Add a small helper for navigation while the list is in filter mode.
   - Detect `FilterState() == list.Filtering`.
   - Intercept navigation keys before Bubble's default filter-accept behavior runs.
   - Treat empty and whitespace-only effective filters the same as any other filter-mode list.

2. Handle keys while filtering:
   - `down`, `ctrl+j`: move the list cursor to the next visible item.
   - `up`, `ctrl+k`: move the list cursor to the previous visible item.
   - `enter`: refresh visible items and activate the currently highlighted item.

3. Keep filter mode active:
   - Do not let Bubble's default `AcceptWhileFiltering` path convert the state to `FilterApplied` for these keys.
   - Leave `FilterInput` focused and `FilterState() == list.Filtering`.

4. Apply the helper in both:
   - `updateList`
   - `updatePicker`

5. Reset navigation to the first visible result when the filter text changes, but preserve explicit user navigation when pending async filter results arrive.

## Test Plan

Add/update unit tests under `internal/tui/model_test.go`:

- `Down` during a non-empty filter keeps `FilterState() == list.Filtering`.
- `Down` during a non-empty filter keeps the header textbox visible.
- `Down` moves selection through visible filtered results.
- `Up` moves selection through visible filtered results.
- `Enter` during a non-empty filter with multiple visible items activates the highlighted item.
- Empty-filter `Down`/`Up` navigate visible results.
- Empty-filter `Enter` activates the highlighted item.
- Whitespace-only `Down`/`Up` navigate visible results.
- Whitespace-only `Enter` activates the highlighted item.
- Existing single-match Enter behavior still works.
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
