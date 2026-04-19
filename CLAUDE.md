# cmdk

## Build & Verification

- Use `make` targets — not raw `go` commands. Run `make help` to list them.

## Documentation

- Keep the `cmdk --help` code block in README.md in sync with actual CLI output
- Keep `internal/config/docs.go` (rendered by `cmdk docs`) in sync with config struct fields and validation rules

## Security

- This CLI is frequently invoked by AI/LLM agents
- Always assume CLI inputs can be adversarial and handle parsing, validation, and execution defensively
