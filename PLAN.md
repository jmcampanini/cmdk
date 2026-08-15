# Plan: Issue #128 — Report created tmux state when client switching fails

## Decision

Issue #128 describes a real defect but its suggested fix is rejected. The decided
contract for `cmdk action run`:

- **Default mode's contract is "create AND switch." Any unmet part is failure:
  exit 1, empty stdout, no success JSON.** There is no partial-success state.
  Callers that do not need the switch must use `--no-switch`, which removes the
  switch phase entirely.
- The defect that remains is that today's diagnostic **hides that creation
  happened**. When `switch-client` fails after the window was created, stderr
  shows only the raw tmux error. The caller cannot tell "nothing happened" from
  "a worktree exists and the command is already running," so retrying duplicates
  irreversible side effects and giving up leaks them.
- The fix: keep exit 1 and empty stdout, but make the failure diagnostic name
  the created session/window/pane IDs, state that the action already launched,
  and warn against rerunning.

This supersedes the issue's suggested acceptance criteria (exit 0 + success
JSON + stderr warning). When closing the issue, note the contract decision so
follow-up #130 drops its "coordinate switch-only success-with-warning" point:
under this contract, switch failure becomes an ordinary classified error
(future `switch_failed` code) whose structured details can carry the created
IDs.

## Behavior specification

Scenario: creation phases succeed (`launch_path_cmd`, session/window/pane
creation, command started), then the requested `switch-client` fails.

- Exit status: 1 (unchanged).
- stdout: empty (unchanged).
- stderr (new, draft wording; values escaped via `safetext`):

  ```
  Error: action "pi worktree" launched, but switching the client failed.

  The action's side effects already happened: the launch path exists and the
  window is running its command. Do not rerun this action — rerunning launches
  it a second time.

  Created tmux state:
    session:     $5  (/Users/you/Code/github.com/acme/api)
    window:      @18  wt-fix-login-bug
    pane:        %51
    launch path: /Users/you/Code/github.com/acme/api-wt/fix-login-bug

  Switch failure: tmux switch-client: exit status 1: can't find client /dev/ttys004

  To go there manually:  tmux switch-client -t '$5:@18'
  Automation that does not need the switch should pass --no-switch.
  ```

Unchanged behavior, verified by existing tests:

- Validation, input, launch-resolution, session-resolution, window-creation,
  and session-metadata failures: same messages, exit 1, no created-state block.
  Metadata is set before the switch step, so it can never be classified as a
  switch failure (issue point 6 holds structurally).
- `--no-switch` never invokes `switch-client`; nothing changes there.
- Successful runs: same JSON, exit 0. No schema changes.
- Interactive TUI and `cmdk session window` keep current behavior; they have no
  JSON-handle contract. The typed error is available to them later if wanted.

## Implementation steps

1. **`internal/tmux/session_window.go` — typed error.**
   Add `SwitchClientError` (`Err error` cause, `Error()`, `Unwrap()`), following
   the `cmdrun.CommandError` typed-error pattern. In `createResolvedWindow`,
   wrap the `switchClient` failure — and only that failure — in
   `*SwitchClientError`. The already-populated `SessionWindowResult` continues
   to be returned alongside the error. No signature changes.

2. **`internal/execute/execute.go` — no change.**
   It already returns the populated `LaunchResult` together with the raw error,
   so `errors.As` reaches the typed error. Add a test locking this in (step 5).

3. **`cmd/action.go` — enriched diagnostic branch.**
   In `runPreparedAction`, when `executeConfiguredActionLaunch` errors:
   - If `errors.As(err, &switchErr)` **and** every `LaunchResult` field needed
     for the diagnostic is non-empty (belt-and-braces completeness guard),
     return the multi-line diagnostic error built from the result and the
     underlying cause. It flows through `terminalSafeActionRunError` for
     terminal-control escaping; `SilenceUsage`/`SilenceErrors` are already set
     on the root command, so the multi-line message prints cleanly.
   - Otherwise fall through to today's plain error path (covers any future
     regression where a switch error appears without a complete result).

4. **Docs.**
   - `cmd/action.go` long help: document that a post-creation switch failure
     exits 1 with the created identifiers in the diagnostic; do not rerun the
     action; automation should prefer `--no-switch`.
   - `cmd/exit_codes.go`: extend the exit-1 paragraph for `action run` with the
     created-but-not-switched case and the same retry guidance.
   - `internal/config/docs.go`: untouched (no config fields change).

5. **Tests.**
   - `internal/tmux/session_window_test.go` (fake runner): switch-client
     failure after new-session and after new-window returns the populated
     result and an error matching `*SwitchClientError`; metadata and creation
     failures do not match it.
   - `internal/execute/execute_test.go`: stubbed
     `createResolvedSessionWindow` returning result + `*SwitchClientError`
     yields a populated `LaunchResult` and an `errors.As`-matchable error from
     `ExecuteWithResult`.
   - `cmd/action_test.go` (stubbed executor): switch error + complete result →
     exit-1 error whose message contains the IDs, launch path, "do not rerun"
     guidance, and no stdout JSON; switch error + incomplete result → plain
     error, no created-state block; non-switch errors unchanged; control bytes
     in result fields are escaped.
   - `e2e/e2e_test.go`: new live test (see verification below).

## End-to-end verification workflow (agent-verified)

New e2e test, e.g. `TestE2E_ActionRunSwitchFailureReportsCreatedState`:

1. Build a PATH-shim `tmux` script in a temp dir: for `switch-client` it writes
   a distinctive error to stderr and exits 1; every other invocation execs the
   real tmux binary with the original arguments. cmdk resolves `tmux` by bare
   name through PATH (`internal/tmux/command.go`), and the harness already
   controls PATH/env for the binary under test, so this is deterministic — no
   races, and all creation-phase tmux calls hit the real server.
2. Reuse the attached-control-client harness from
   `TestE2E_ActionRunLaunchesWithLiteralInputsAndReportsLiveTmuxState`: run the
   built cmdk binary inside a pane with the shim first on PATH, capturing
   stdout, stderr, and exit status to files.
3. Assert: exit status 1; stdout empty; stderr contains the shim's switch
   failure text, the "do not rerun" guidance, and session/window/pane IDs.
4. Prove the IDs identify live state: query the real tmux server for the
   session key option, window, and pane; assert they match the IDs printed in
   the diagnostic and that the window is running the configured command.

Full verification: `make check` (unit + e2e + lint + fmt + tidy + vuln). The
e2e test above is the agent-verified proof that the requested behavior works
end-to-end against a real tmux server.

## Decision ledger

### Interview-settled

- Q: contract when creation succeeds but switch fails → strict failure: exit 1,
  empty stdout, no success JSON; `--no-switch` is the automation escape hatch.
- Q: what to build for #128 under that contract → Option A: typed
  `SwitchClientError` at the tmux boundary + enriched exit-1 diagnostic naming
  the created IDs + docs steering automation to `--no-switch`.

### Source-resolved

- issue #128 + `cmd/action.go`: defect → switch-only failure discards the
  populated result; caller can't distinguish "nothing happened" from "side
  effects exist," so it retries (duplicates) or gives up (leaks).
- issue #128 scope + siblings #127/#129/#130: surfaces → `cmdk action run`
  only; TUI and `cmdk session window` unchanged.
- code structure (`createManagedSession` sets metadata before `switchClient`):
  metadata/partial-setup failures are structurally excluded from the switch
  classification.
- issue #127: `--no-switch` never reaches the switch phase; orthogonal.
- issue #130: machine-readable error codes deferred there; this diagnostic
  stays human-readable, terminal-safe prose. `SwitchClientError` is the
  classification #130 later maps to a stable code.
- codebase (`safetext`, `actionRunError`): all diagnostic values are
  terminal-control escaped.
- `cmd/root.go` (`SilenceUsage`/`SilenceErrors`): multi-line error messages
  print without cobra usage noise.
- AGENTS.md: docs surfaces → command help + `cmd/exit_codes.go`; `docs.go`
  untouched; build/verify via make targets.

### Approved judgment calls

1. Typed error (`tmux.SwitchClientError`, `errors.As`) instead of changing
   `CreateResolvedSessionWindow`'s signature to return switch outcome as data.
2. Command-layer completeness guard: enriched diagnostic only when the typed
   error co-occurs with a fully populated result; otherwise plain failure.
3. Live e2e via PATH-shim tmux failing only `switch-client`, plus fake-runner
   unit tests; not unit-only.
4. Multi-line diagnostic rendered in `cmd/action.go`, including the manual
   `tmux switch-client -t` recovery command and `--no-switch` guidance.

## Open questions

None blocking. Diagnostic wording above is a draft; final phrasing may be
tightened during implementation without changing the contract (exit 1, empty
stdout, IDs + do-not-rerun guidance on stderr).
