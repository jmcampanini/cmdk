.PHONY: build test lint check

check: test lint

build: check
	@mkdir -p build
	go build -o build/cmdk .

test:
	go test ./...

lint:
	golangci-lint run
