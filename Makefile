.PHONY: help build test test-unit test-e2e test-gen-icons lint lint-fix fmt fmt-check tidy tidy-check check clean gen-icons

BUILD_DIR   := build
BINARY      := $(BUILD_DIR)/cmdk
CMD         := .
PKG         := ./...
GOFMT_FILES := $(shell git ls-files '*.go')

# Version is injected at build time via ldflags so `cmdk --version` reports
# the git describe of the working tree (or an RFC3339 timestamp as a fallback).
VERSION := $(shell git describe --tags --dirty --always 2>/dev/null || date -u '+%Y-%m-%dT%H:%M:%SZ')
LDFLAGS := -ldflags "-X github.com/jmcampanini/cmdk/cmd.Version=$(VERSION)"

.DEFAULT_GOAL := help

help: ## Show this help.
	@awk 'BEGIN {FS = ":.*##"; printf "Usage:\n  make <target>\n\nTargets:\n"} /^[a-zA-Z0-9_.-]+:.*##/ { printf "  %-16s %s\n", $$1, $$2 }' $(MAKEFILE_LIST)

build: ## Build cmdk into ./build/cmdk.
	@mkdir -p $(BUILD_DIR)
	go build $(LDFLAGS) -o $(BINARY) $(CMD)

define run_unit_tests
packages="$$(go list $(PKG))"; rc=$$?; \
if [ $$rc -ne 0 ]; then exit $$rc; fi; \
packages="$$(printf '%s\n' "$$packages" | grep -v '/e2e$$')"; \
if [ -z "$$packages" ]; then echo "no unit test packages found" >&2; exit 1; fi; \
go test -race $$packages
endef

test: ## Run unit and required end-to-end tests with the race detector.
	@$(run_unit_tests)
	@$(MAKE) --no-print-directory test-e2e

test-unit: ## Run unit tests without end-to-end tests.
	@echo "CMDK_E2E_STATUS=OPTED_OUT"
	@if [ -n "$${CI:-}" ]; then echo "required CI rejects the end-to-end opt-out" >&2; exit 1; fi
	@$(run_unit_tests)

test-e2e: ## Run required tmux end-to-end tests and validate their completion markers.
	@mkdir -p .sandbox
	@output=".sandbox/test-e2e.$$$$.log"; \
	trap 'rm -f "$$output"' EXIT HUP INT TERM; \
	go test -count=1 -race -v ./e2e >"$$output" 2>&1; test_rc=$$?; \
	cat "$$output"; cat_rc=$$?; \
	validation_rc=0; \
	if [ $$cat_rc -ne 0 ]; then \
		echo "failed to read e2e test output" >&2; \
		validation_rc=1; \
	fi; \
	if ! grep -Fq 'CMDK_E2E_TMUX_SENTINEL=PASS' "$$output"; then \
		echo "missing CMDK_E2E_TMUX_SENTINEL=PASS in e2e test output" >&2; \
		validation_rc=1; \
	fi; \
	if ! grep -Fq -- '--- PASS: TestE2E_TmuxSentinel ' "$$output"; then \
		echo "tmux e2e sentinel test did not pass" >&2; \
		validation_rc=1; \
	fi; \
	pass_count="$$(grep -c '^--- PASS:' "$$output" || true)"; \
	case "$$pass_count" in ''|*[!0-9]*) pass_count=0 ;; esac; \
	if [ "$$pass_count" -le 0 ]; then \
		echo "e2e test output reported no passing tests" >&2; \
		validation_rc=1; \
	else \
		echo "CMDK_E2E_TEST_COUNT=$$pass_count"; \
	fi; \
	if [ $$test_rc -ne 0 ]; then exit $$test_rc; fi; \
	exit $$validation_rc

lint: ## Run golangci-lint.
	golangci-lint run $(PKG)

lint-fix: ## Run golangci-lint with --fix.
	golangci-lint run --fix $(PKG)

fmt: ## Format tracked Go files.
	@if [ -n "$(GOFMT_FILES)" ]; then gofmt -w $(GOFMT_FILES); fi

fmt-check: ## Fail if tracked Go files need gofmt.
	@files="$$(gofmt -l $(GOFMT_FILES))"; \
	if [ -n "$$files" ]; then \
		echo "gofmt needed:"; \
		echo "$$files"; \
		echo "Run: make fmt"; \
		exit 1; \
	fi

tidy: ## Apply go mod tidy.
	go mod tidy

tidy-check: ## Fail if go mod tidy would change go.mod/go.sum.
	@out=$$(go mod tidy -diff); rc=$$?; \
	if [ $$rc -eq 0 ]; then exit 0; fi; \
	if [ -n "$$out" ]; then echo "$$out"; echo "go mod tidy would change go.mod/go.sum"; exit 1; fi; \
	echo "go mod tidy failed (rc=$$rc)"; exit $$rc

check: fmt-check tidy-check lint test ## Run all non-mutating checks.

clean: ## Remove build artifacts, coverage files, and test cache.
	rm -rf $(BUILD_DIR) out dist coverage.out coverage.html *.coverprofile
	go clean -testcache

gen-icons: ## Regenerate icon entries from Nerd Fonts glyphnames.json.
	go run ./internal/icon/gen

test-gen-icons: ## Run the icon generator tests.
	go test -race ./internal/icon/gen
