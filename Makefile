.PHONY: build test lint check

check: test lint

build: check
	go build -o cmdk .

test:
	go test ./...

lint:
	golangci-lint run
