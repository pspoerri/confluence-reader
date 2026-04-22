BINARY  := confluence-reader
PKG     := ./cmd/confluence/
GOFLAGS :=

# Detect sandboxed environments where default cache dirs are inaccessible.
export GOCACHE  ?= $(shell go env GOCACHE 2>/dev/null || echo /tmp/go-cache)
export GOPATH   ?= $(shell go env GOPATH 2>/dev/null || echo /tmp/gopath)

# Version info baked into the binary via -ldflags. Fall back gracefully when
# git is unavailable (e.g. a source tarball) so the build still succeeds.
GIT_VERSION := $(shell git describe --tags --always 2>/dev/null)
GIT_COMMIT  := $(shell git rev-parse --short=12 HEAD 2>/dev/null)
GIT_DIRTY   := $(shell test -n "`git status --porcelain 2>/dev/null`" && echo -dirty)
BUILD_TIME  := $(shell date -u +%Y-%m-%dT%H:%M:%SZ)

VERSION_PKG := github.com/pspoerri/confluence-reader/internal/version
LDFLAGS := \
	-X '$(VERSION_PKG).ldVersion=$(GIT_VERSION)' \
	-X '$(VERSION_PKG).ldCommit=$(GIT_COMMIT)$(GIT_DIRTY)' \
	-X '$(VERSION_PKG).ldTime=$(BUILD_TIME)'

.PHONY: all build test test-v test-race vet fmt fmt-check clean install install-hooks install-skill help

all: fmt-check vet test build ## Run checks, tests, and build (default)

build: ## Build the binary
	go build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o $(BINARY) $(PKG)

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

install-hooks: ## Install git pre-commit hooks
	cp githooks/pre-commit .git/hooks/pre-commit
	chmod +x .git/hooks/pre-commit

INSTALL_DIR := $(HOME)/.local/bin

install: build ## Install binary to ~/.local/bin
	mkdir -p $(INSTALL_DIR)
	cp $(BINARY) $(INSTALL_DIR)/$(BINARY)

SKILL_DIR := $(HOME)/.config/opencode/skills/confluence-reader

install-skill: install ## Install binary and OpenCode skill globally
	mkdir -p $(SKILL_DIR)
	cp .opencode/skills/confluence-reader/SKILL.md $(SKILL_DIR)/SKILL.md

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "  %-14s %s\n", $$1, $$2}'
