## Build & Verification

- This repository uses mise for tool versions and task execution. Run tasks with `mise run <task>`; do not call `go`, `golangci-lint`, `tmux`, or similar tools directly.
- Common tasks (`mise tasks` lists all of them):
  - `mise run check` runs the complete verification contract. **Run this before declaring work done.**
  - `mise run build` compiles `build/cmdk` with version ldflags.
  - `mise run test` runs unit tests then the required tmux end-to-end suite; `mise run test-unit` skips e2e locally only.
  - `mise run fmt` and `mise run lint-fix` apply formatting and lint fixes.
- `mise.toml` pins Go, golangci-lint, govulncheck, and the tmux used by tests; `mise.lock` records their checksums. Bump a tool with `mise upgrade --bump <tool>`, run `mise lock`, and commit both files.

## Documentation

- README.md is a landing page (install + quickstart + reference pointers); behavior reference lives in `cmdk --help`, not the README
- Keep `internal/config/docs.go` (rendered by `cmdk docs`) in sync with config struct fields and validation rules
- Keep `cmd/exit_codes.go` (rendered by `cmdk help exit-codes`) accurate when changing process exit behavior

## Security

- This CLI is frequently invoked by AI/LLM agents
