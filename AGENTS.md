## Build & Verification

- Run tasks with `mise run <task>`; do not call `go`, `golangci-lint`, `tmux`, or similar tools directly.
- Run `mise run check` before declaring work done.
- Bump a tool with `mise upgrade --bump <tool>`, run `mise lock`, and commit `mise.toml` and `mise.lock`.

## Documentation

- README.md is a landing page (install + quickstart + reference pointers); behavior reference lives in `cmdk --help`, not the README
- Keep `internal/config/docs.go` (rendered by `cmdk docs`) in sync with config struct fields and validation rules
- Keep `cmd/exit_codes.go` (rendered by `cmdk help exit-codes`) accurate when changing process exit behavior
