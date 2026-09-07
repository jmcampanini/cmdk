.PHONY: help build test test-unit test-e2e test-gen-icons lint lint-fix fmt fmt-check tidy tidy-check version-check vuln check clean gen-icons

BUILD_DIR   := build
BINARY      := $(BUILD_DIR)/cmdk
CMD         := .
PKG         := ./...

VERSION := $(shell git describe --tags --dirty --always 2>/dev/null || printf 'unknown')
LDFLAGS := -ldflags "-X github.com/jmcampanini/cmdk/cmd.Version=$(VERSION)"

.DEFAULT_GOAL := help

help: ## Show this help.
	@awk 'BEGIN {FS = ":.*##"; printf "Usage:\n  make <target>\n\nTargets:\n"} /^[a-zA-Z0-9_.-]+:.*##/ { printf "  %-16s %s\n", $$1, $$2 }' $(MAKEFILE_LIST)

build: ## Build cmdk into ./build/cmdk.
	@mkdir -p $(BUILD_DIR)
	go build -trimpath -buildvcs=false $(LDFLAGS) -o $(BINARY) $(CMD)

define run_unit_tests
packages="$$(go list $(PKG))"; rc=$$?; \
if [ $$rc -ne 0 ]; then exit $$rc; fi; \
packages="$$(printf '%s\n' "$$packages" | grep -v '/e2e$$')"; \
if [ -z "$$packages" ]; then echo "no unit test packages found" >&2; exit 1; fi; \
go test -count=1 -race $$packages
endef

test: ## Run all unit and required end-to-end tests uncached with the race detector.
	@$(run_unit_tests)
	@$(MAKE) --no-print-directory test-e2e

test-unit: ## Run unit tests uncached with the race detector, without end-to-end tests.
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
	go tool golangci-lint run $(PKG)

lint-fix: ## Run golangci-lint with --fix.
	go tool golangci-lint run --fix $(PKG)

fmt: ## Format Go source files.
	go tool golangci-lint fmt

fmt-check: ## Verify formatting without changing files.
	go tool golangci-lint fmt --diff

tidy: ## Apply go mod tidy.
	go mod tidy

tidy-check: ## Fail if go mod tidy would change go.mod/go.sum.
	@out=$$(go mod tidy -diff); rc=$$?; \
	if [ $$rc -eq 0 ]; then exit 0; fi; \
	if [ -n "$$out" ]; then echo "$$out"; echo "go mod tidy would change go.mod/go.sum"; exit 1; fi; \
	echo "go mod tidy failed (rc=$$rc)"; exit $$rc

version-check: build ## Verify the built binary reports the injected version.
	@case "$(VERSION)" in unknown|n/a|"") echo "degenerate version identity: '$(VERSION)'"; exit 1;; esac
	@out="$$($(BINARY) --version)" || exit $$?; \
	if [ "$$out" != "cmdk version $(VERSION)" ]; then \
		echo "version mismatch: got '$$out', want 'cmdk version $(VERSION)'"; \
		exit 1; \
	fi

vuln: ## Check dependencies and reachable code for known vulnerabilities.
	go tool govulncheck ./...

check: fmt-check tidy-check lint test build version-check vuln ## Run the complete local verification contract.

clean: ## Remove build artifacts, coverage files, and test cache.
	rm -rf $(BUILD_DIR) out dist coverage.out coverage.html *.coverprofile
	go clean -testcache

gen-icons: ## Regenerate icon entries from Nerd Fonts glyphnames.json.
	go run ./internal/icon/gen

test-gen-icons: ## Run the icon generator tests uncached with the race detector.
	go test -count=1 -race ./internal/icon/gen
