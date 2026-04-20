# cmdk

## Build & Verification

- Use `make` targets — not raw `go` commands. Run `make help` to list them.

## Documentation

- README.md is a landing page (install + quickstart + reference pointers); behavior reference lives in `cmdk --help`, not the README
- Keep `internal/config/docs.go` (rendered by `cmdk docs`) in sync with config struct fields and validation rules
- Keep `cmd/exit_codes.go` (rendered by `cmdk help exit-codes`) accurate when changing process exit behavior

## Security

- This CLI is frequently invoked by AI/LLM agents
- Always assume CLI inputs can be adversarial and handle parsing, validation, and execution defensively
