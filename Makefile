BINARY    := pr-wrangler
MODULE    := github.com/petems/pr-wrangler
GO        := go
GOFLAGS   ?=
LDFLAGS   ?=

VERSION   ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT    ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo none)
BUILD_DATE := $(shell date -u +%Y-%m-%dT%H:%M:%SZ)

.DEFAULT_GOAL := build

## help: Show this help message
.PHONY: help
help:
	@grep -E '^## ' $(MAKEFILE_LIST) | sed 's/## //' | column -t -s ':'

## build: Build the binary
.PHONY: build
build:
	$(GO) build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o $(BINARY) .

## install: Install the binary to $GOPATH/bin
.PHONY: install
install:
	$(GO) install $(GOFLAGS) -ldflags "$(LDFLAGS)" .

## run: Build and run
.PHONY: run
run: build
	./$(BINARY)

## test: Run all tests
.PHONY: test
test:
	$(GO) test $(GOFLAGS) ./...

## test-acceptance: Run TUI acceptance tests with mock GitHub and tmux dependencies
.PHONY: test-acceptance
test-acceptance:
	$(GO) test $(GOFLAGS) -tags acceptance ./internal/tui

## test-all: Run unit and acceptance tests
.PHONY: test-all
test-all: test test-acceptance

## test-verbose: Run all tests with verbose output
.PHONY: test-verbose
test-verbose:
	$(GO) test $(GOFLAGS) -v ./...

## test-race: Run tests with race detector
.PHONY: test-race
test-race:
	$(GO) test $(GOFLAGS) -race ./...

## test-cover: Run tests with coverage report
.PHONY: test-cover
test-cover:
	$(GO) test $(GOFLAGS) -coverprofile=coverage.out ./...
	$(GO) tool cover -func=coverage.out
	@rm -f coverage.out

## test-cover-html: Generate HTML coverage report
.PHONY: test-cover-html
test-cover-html:
	$(GO) test $(GOFLAGS) -coverprofile=coverage.out ./...
	$(GO) tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

## bench: Run benchmarks
.PHONY: bench
bench:
	$(GO) test $(GOFLAGS) -bench=. -benchmem ./...

## lint: Run golangci-lint
.PHONY: lint
lint:
	golangci-lint run ./...

## vet: Run go vet
.PHONY: vet
vet:
	$(GO) vet ./...

## fmt: Format code
.PHONY: fmt
fmt:
	$(GO) fmt ./...
	gofumpt -w . 2>/dev/null || true

## fmt-check: Check formatting (CI-friendly)
.PHONY: fmt-check
fmt-check:
	@test -z "$$(gofmt -l .)" || (echo "Files need formatting:"; gofmt -l .; exit 1)

## tidy: Tidy and verify module dependencies
.PHONY: tidy
tidy:
	$(GO) mod tidy
	$(GO) mod verify

## clean: Remove build artifacts
.PHONY: clean
clean:
	rm -f $(BINARY) coverage.out coverage.html

## check: Run fmt-check, lint, vet, test-race (CI entrypoint)
.PHONY: check
check: fmt-check lint vet test-race
