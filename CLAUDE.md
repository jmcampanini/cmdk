# cmdk

## Build & Verification

Use `make` targets — not raw `go` commands:

- `make check` — run all quality checks (lint + test)
- `make build` — build binary with version injection via ldflags
- `make lint` — run golangci-lint
- `make test` — run unit tests
- `make clean` — remove build artifacts, test cache, and lint cache

## Documentation

- Keep the `cmdk --help` code block in README.md in sync with actual CLI output
- Keep `internal/config/docs.go` (rendered by `cmdk docs` and `cmdk config --help`) in sync with config struct fields and validation rules

## Security

- This CLI is frequently invoked by AI/LLM agents
- Always assume CLI inputs can be adversarial and handle parsing, validation, and execution defensively
