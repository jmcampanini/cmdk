## Build & Verification

- Run tasks with `mise run <task>`; do not call `go`, `golangci-lint`, `markdownlint-cli2`, `tmux`, or similar tools directly.
- Run `mise run check` before declaring work done.
- Bump a tool with `mise upgrade --bump <tool>`, run `mise lock`, and commit `mise.toml` and `mise.lock`.


## Documentation

- Command help (`cmdk --help`, each command's `--help`, and the help topics under `cmdk help <topic>`) is the canonical documentation of user-facing behavior; README.md is a landing page in the CLI README layout (purpose paragraph, Install, Representative commands, Required external programs, Configuration). Keep both consistent with observable behavior.
- Keep the key reference in `internal/config/docs.go` (rendered into `cmdk config --help`) in sync with config struct fields and validation rules
- Keep `internal/cli/exit_codes.go` (rendered by `cmdk help exit-codes`) accurate when changing process exit behavior
