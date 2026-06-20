# Plan: Sort errors/loading explicitly at the top

## Goal

Stop sorting `error` and `loading` items by `Data["source_type"]`. Make unfiltered root/list ordering explicit and intentional:

1. `error`
2. `loading`
3. bell windows, when `bell_to_top = true`
4. normal known types: `action`, `dir`, `window`
5. unknown types, in first-seen bucket order

Filtering remains unchanged: filtered results continue to be fuzzy-ranked.

## Final decisions

- Errors always appear first in the unfiltered list.
- Loading placeholders appear immediately after errors.
- Bell-to-top applies after errors/loading, not before them.
- Remove `Data["source_type"]` entirely.
- Loading placeholders use a single generic loading color.
- Remove `Source.Type` / `AsyncSource.Type` if they become unused after removing `source_type`.
- Tests should assert visible behavior and ordering, not internal `source_type` metadata.

## Source-resolved decisions

- We used `internal/item/group.go` to answer: how are `error`/`loading` sorted today? → By `Data["source_type"]`.
- We used `internal/tui/delegate.go` to answer: what else uses `source_type` at runtime? → Loading icon color.
- We used `internal/generator/root.go` and `internal/tmux/windows.go` to answer: where is `source_type` written? → Error/loading constructors and tmux parse-error items.
- We used `internal/tui/filter.go` to answer: does `GroupAndOrder` control filtered ranking? → No; filtering is fuzzy-ranked separately.
- We used `internal/config/docs.go` to answer: does `bell_to_top` documentation need an update? → Yes; it currently says bell windows sort above all other items.

## Implementation steps

1. Update grouping/sorting
   - Modify `internal/item/group.go` so ordering is explicit by item type.
   - Remove `orderBucketKey` and all `source_type`-based bucket selection.
   - Preserve stable ordering within each bucket.
   - Preserve unknown-type behavior: unknown buckets come after known buckets in first-seen order.

2. Remove `source_type` writes
   - Remove `errItem.Data["source_type"] = ...` from `internal/generator/root.go`.
   - Remove `it.Data["source_type"] = ...` from loading item creation.
   - Remove tmux parse-error `source_type` metadata from `internal/tmux/windows.go`.

3. Simplify loading presentation
   - Update `internal/tui/delegate.go` so loading uses the generic loading icon/color only.
   - Remove `colorForSourceType` if no longer used.

4. Remove unused source type plumbing
   - Check whether `generator.Source.Type` and `tui.AsyncSource.Type` are still used.
   - If they are only feeding removed metadata, delete the fields and update constructors/call sites in `cmd/root.go`, tests, and async handling.

5. Update tests
   - Replace `source_type` assertions with behavior assertions.
   - Update `internal/item/group_test.go` to cover:
     - errors before loading
     - loading before bell/normal items
     - bell windows after errors/loading
     - known type order after status items
     - unknown types still last
     - stable order within error/loading buckets
   - Update async/root/integration/delegate/tmux tests to remove `source_type` expectations.
   - Rename tests that refer to source-type bucketing.

6. Update docs
   - Update `internal/config/docs.go` `bell_to_top` wording to clarify bell windows sort above normal items, but after errors/loading.
   - Check generated/help-related docs expectations if any tests cover this text.

## Verification

Use Makefile targets, per project guidance:

- `make fmt`
- `make test`
- If needed before finalizing: `make check`

## Remaining open questions

None pivotal.
