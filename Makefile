BINARY_NAME := cmdk
OUT_DIR := out

# Version is injected at build time via ldflags so `cmdk --version` reports
# the git describe of the working tree (or an RFC3339 timestamp as a fallback).
VERSION := $(shell git describe --tags --dirty --always 2>/dev/null || date -u '+%Y-%m-%dT%H:%M:%SZ')
LDFLAGS := -ldflags "-X github.com/jmcampanini/cmdk/cmd.Version=$(VERSION)"

.DEFAULT_GOAL := help
.PHONY: help build check fmt tidy lint test clean gen-icons

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  %-15s %s\n", $$1, $$2}'

build: ## compile binary to ./out/cmdk
	mkdir -p $(OUT_DIR)
	go build $(LDFLAGS) -o $(OUT_DIR)/$(BINARY_NAME) .

test: ## run tests with -race
	go test -race ./...

lint: ## run golangci-lint
	golangci-lint run ./...

fmt: ## apply gofmt -w in-place
	gofmt -w .

tidy: ## apply go mod tidy
	go mod tidy

check: ## fmt-check + tidy-check + lint + test (CI gate, never modifies files)
	@test -z "$$(gofmt -l .)" || { echo "gofmt needed:"; gofmt -d .; exit 1; }
	go mod tidy -diff
	$(MAKE) lint
	$(MAKE) test

clean: ## remove build artifacts and caches
	go clean
	go clean -testcache
	golangci-lint cache clean
	rm -rf $(OUT_DIR)

gen-icons: ## regenerate icon entries from Nerd Fonts glyphnames.json
	go run ./internal/icon/gen
