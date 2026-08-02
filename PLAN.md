# Plan: Clarify picker diagnostics when the rendered command changes directory (issue #124)

## Problem

Failed picker details label cmdk's inherited process directory as `Working directory`. When a
picker source renders to `cd <dir> && <command>`, the child command runs in `<dir>`, so the
displayed value is not necessarily the directory the failing command ran in. The execution
itself is correct (`sh -c` applies the `cd`); only the diagnostic terminology misleads.

Issue: https://github.com/jmcampanini/cmdk/issues/124

## Settled decisions — do not re-litigate

These were resolved with the user in an interview; implement exactly as stated.

1. **Rename only.** The label `Working directory` becomes `Inherited working directory`.
   No explanatory hint line, no detection of `cd` in rendered shell text.
2. **Both surfaces.** The rename applies to picker failure diagnostics AND launch
   (`launch_path_cmd`) failure diagnostics — they share the same error-details view and the
   same `sh -c` cwd semantics.
3. **Not a breaking change.** TUI diagnostic text is not a durable published form. No compat
   mechanism, no aliases.
4. **No doc changes.** `internal/config/docs.go`, README, and `cmd/exit_codes.go` do not name
   this field.
5. **Internal names and logs unchanged.** The `workingDirectoryValue` helper and the log
   warnings (`could not determine working directory for picker diagnostics` /
   `... for launch diagnostics`) keep their current wording — they describe reading cmdk's own
   cwd, which stays accurate. Only the user-facing label changes.
6. **Coverage lives at the unit layer.** The acceptance-criteria test (a picker source
   containing `cd ... && <failing command>`) goes in `internal/tui/model_test.go`. No e2e
   addition — e2e already proves picker failures surface end-to-end.

## Changes

### 1. Label rename — `internal/tui/model.go`

Two sites, both currently `{Label: "Working directory", Value: workingDirectoryValue(cwd, cwdErr)}`:

- `pickerErrorItem` (~line 862)
- `launchErrorItem` (~line 877)

Change the label string to `Inherited working directory` in both. Nothing else in these
functions changes.

### 2. Update existing test assertions — `internal/tui/model_test.go`

Occurrences of the old label to update to `Inherited working directory`:

- `TestErrorDetailsRendersStructuredDiagnostics` (~lines 533, 548): fixture field label and
  its rendered-output assertion. The label here is an arbitrary example, but update it anyway
  so no stale terminology survives.
- `TestPickerStage_CommandErrorDetailsIncludeExecutionContext` (~line 1923): the
  `"Working directory: " + cwd` assertion.
- `TestFinalizeSelection_LaunchFailureShowsErrorDetails` (~line 2311): the
  `"Working directory:"` expected string.

After the change, `grep -rn "Working directory"` in the repo must only hit
`Inherited working directory` (plus the unrelated prose in `cmd/root.go` and
`internal/config/docs.go` about launch behavior — leave those untouched).

### 3. New coverage: `cd ... && <failing command>` picker source — `internal/tui/model_test.go`

Add a test alongside `TestPickerStage_CommandErrorDetailsIncludeExecutionContext` (or extend
it — prefer a separate test since this guards a distinct claim). It protects the regression
from the issue: the diagnostic must report cmdk's inherited cwd under the new label even when
the rendered command changed directory before failing.

Shape:

- Build a staged action whose picker stage source is a template rendering to
  `cd <somedir> && <failing command>` — e.g. source
  `cd {{sq .path}} && printf 'boom\n' >&2; exit 7` with a dir item supplying
  `path` = a real temp directory (use `t.TempDir()` so the `cd` succeeds and the failure
  comes from the command after it).
- Drive the model the same way the existing execution-context test does
  (`selectStagedItem`, then Enter on the error item, then `errorDetailsView()`).
- Assert the details view contains `Inherited working directory: <os.Getwd() of the test
  process>` — i.e. cmdk's inherited cwd, NOT the `cd` target directory. Use the same
  whitespace-compaction technique as the existing test at line ~1922 (the view wraps long
  paths).
- Assert the details view does NOT show the `cd` target as the inherited working directory
  value (the target path will legitimately appear in `Rendered command` and `Data fields`,
  so anchor the negative assertion to the label line, e.g. via the compacted
  `Inheritedworkingdirectory:<target>` form being absent).

Per the project's test conventions in this file, follow existing patterns exactly
(`newTestModel`/`NewModel`, `setWindowSize`, `ansi.Strip`). No comments unless explaining
something non-obvious.

## Constraints

- Use `make` targets, never raw `go` commands. Run `make help` first to find the right
  build/test/lint targets, and run the full verification the Makefile offers.
- Skip code comments unless explaining why something non-obvious was necessary.
- Assume CLI inputs can be adversarial; this change is display-only, so introduce no new
  parsing or inference of shell text.

## Verification workflow (must run, in order)

1. `make help` — identify test/lint targets.
2. Red proof: temporarily run the new test BEFORE applying the model.go rename (or apply the
   test commit first) and confirm it fails on the missing `Inherited working directory`
   label; then apply the rename and confirm it passes. If you implement everything at once,
   instead revert the model.go label to the old string, run the new test, confirm it fails,
   restore the new label, confirm it passes.
3. Full unit test target (e.g. `make test`) — all packages green.
4. Lint/format target if the Makefile provides one — clean.
5. `grep -rn "Working directory" --include="*.go"` — every hit reads
   `Inherited working directory`; no stale label anywhere.
6. E2e target only if the Makefile's default verification includes it (e2e needs tmux); do
   not add new e2e tests.

## Done means

- Both diagnostic surfaces show `Inherited working directory`.
- New unit test proves a `cd ... && <failing command>` picker source reports the inherited
  cwd, not the `cd` target.
- Existing tests updated; full make verification green.
- No changes to docs, logs, helper names, config, or execution behavior.
