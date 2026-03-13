BINARY  := confluence-reader
PKG     := ./cmd/confluence/
GOFLAGS :=

# Detect sandboxed environments where default cache dirs are inaccessible.
export GOCACHE  ?= $(shell go env GOCACHE 2>/dev/null || echo /tmp/go-cache)
export GOPATH   ?= $(shell go env GOPATH 2>/dev/null || echo /tmp/gopath)

.PHONY: all build test test-v test-race vet fmt fmt-check clean install help

all: fmt-check vet test build ## Run checks, tests, and build (default)

build: ## Build the binary
	go build $(GOFLAGS) -o $(BINARY) $(PKG)

test: ## Run all tests
	go test ./...

test-v: ## Run all tests with verbose output
	go test ./... -v

test-race: ## Run tests with race detector
	go test -race ./...

vet: ## Run static analysis
	go vet ./...

fmt: ## Format all Go files
	gofmt -w .

fmt-check: ## Check formatting (fails if unformatted)
	@test -z "$$(gofmt -l .)" || (echo "unformatted files:"; gofmt -l .; exit 1)

clean: ## Remove build artifacts
	rm -f $(BINARY)

INSTALL_DIR := $(HOME)/.local/bin

install: build ## Install binary to ~/.local/bin
	mkdir -p $(INSTALL_DIR)
	cp $(BINARY) $(INSTALL_DIR)/$(BINARY)

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "  %-14s %s\n", $$1, $$2}'
