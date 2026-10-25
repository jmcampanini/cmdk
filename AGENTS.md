## Build & Verification

- Run tasks with `mise run <task>`; do not call `go`, `golangci-lint`, `markdownlint-cli2`, `tmux`, or similar tools directly.
- Run `mise run check` before declaring work done.
- Bump a tool with `mise upgrade --bump <tool>`, run `mise lock`, and commit `mise.toml` and `mise.lock`.


## Documentation

- README.md follows the CLI README layout: purpose paragraph, Install, Representative commands, Required external programs, and Configuration; behavior reference lives in `cmdk --help`, not the README
- Keep `internal/config/docs.go` (rendered by `cmdk docs`) in sync with config struct fields and validation rules
- Keep `internal/cli/exit_codes.go` (rendered by `cmdk help exit-codes`) accurate when changing process exit behavior
