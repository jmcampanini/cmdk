# cmdk

## Build & Verification

Use `make` targets — not raw `go` commands:

- `make check` — run all quality checks (lint + test)
- `make build` — build binary with version injection via ldflags
- `make lint` — run golangci-lint
- `make test` — run unit tests
- `make clean` — remove build artifacts
