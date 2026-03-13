# AGENTS.md

Guidelines for AI coding agents working in this repository.

## Project Overview

`confluence-reader` is a Go CLI tool that interacts with the Confluence Cloud REST API v2
in a filesystem-like manner. It reads config from `~/.config/confluence-reader/config.json`,
caches page trees locally, and converts Confluence storage-format HTML to Markdown.

## Build / Test / Lint Commands

```bash
# Build the binary
go build -o confluence-reader ./cmd/confluence/

# Run all tests
go test ./...

# Run all tests with verbose output
go test ./... -v

# Run a single test by name
go test ./internal/convert/ -run TestToMarkdown_Headings -v

# Run tests for a single package
go test ./internal/cache/ -v

# Run tests with race detector
go test -race ./...

# Vet (static analysis)
go vet ./...

# Format all files
gofmt -w .

# Check formatting (CI - exits non-zero if unformatted)
gofmt -l . | grep -q . && echo "unformatted files" && exit 1 || true
```

If `GOCACHE` or `GOPATH` cause permission issues (sandboxed environments), set:
```bash
export GOCACHE=/tmp/go-cache GOPATH=/tmp/gopath
```

## Project Structure

```
cmd/confluence/main.go       -- CLI entry point, argument parsing, usage text
internal/
  api/
    client.go                -- HTTP client, auth, pagination helpers
    types.go                 -- API response types (Space, Page, Attachment, etc.)
  cache/
    cache.go                 -- JSON file cache (load/save/refresh), tree builder, search
    cache_test.go
  cli/
    commands.go              -- Command implementations (spaces, ls, find, read, read-file, refresh, configure)
  config/
    config.go                -- Reads ~/.config/confluence-reader/config.json
    config_test.go
  convert/
    markdown.go              -- Confluence storage HTML -> Markdown conversion
    markdown_test.go
config.example.json          -- Template config file for users
Makefile                     -- Build, test, install targets
.opencode/skills/
  confluence-reader/
    SKILL.md                 -- Skill definition for AI coding tools (keep in sync with README)
```

## Makefile Targets

| Target      | Description                                          |
|-------------|------------------------------------------------------|
| `all`       | Run checks, tests, and build (default)               |
| `build`     | Build the binary                                     |
| `test`      | Run all tests                                        |
| `test-v`    | Run all tests with verbose output                    |
| `test-race` | Run tests with race detector                         |
| `vet`       | Run static analysis                                  |
| `fmt`       | Format all Go files                                  |
| `fmt-check` | Check formatting (fails if unformatted)              |
| `clean`     | Remove build artifacts                               |
| `install-hooks` | Install git pre-commit hooks                     |
| `install`   | Build and install binary to `~/.local/bin/`          |
| `install-skill` | Install binary and skill definition globally     |
| `help`      | Show available targets                               |

## Code Style

### Formatting
- Run `gofmt` (or `goimports`) before committing. No tabs-vs-spaces debate: Go uses tabs.
- No line length limit enforced, but keep lines reasonable (~100 chars).

### Imports
- Group imports in three blocks separated by blank lines:
  1. Standard library
  2. External dependencies (none currently)
  3. Internal packages (`github.com/pspoerri/confluence-reader/internal/...`)
- Use `goimports` to manage import ordering automatically.

### Naming
- Exported types and functions use `PascalCase`.
- Unexported functions and variables use `camelCase`.
- Acronyms stay uppercase: `ID`, `URL`, `API`, `HTML`, `HTTP`.
- Receiver names are short (1-2 chars): `c` for Client, `s` for Store, `a` for App.
- Test functions follow `TestFunctionName_Scenario` pattern.

### Types
- API response types live in `internal/api/types.go` with JSON struct tags.
- Use pointer fields (`*Type`) only when nil is a meaningful value (e.g., optional body).
- Generic pagination uses `PaginatedResponse[T any]`.

### Error Handling
- Always return errors up the call stack with `fmt.Errorf("context: %w", err)`.
- Use `%w` for wrapping so callers can use `errors.Is` / `errors.As`.
- Never ignore errors silently -- at minimum log them.
- CLI commands print errors to stderr and exit with code 1.
- User-facing messages should be lowercase, no trailing period.

### Functions
- Keep functions short and single-purpose.
- The `internal/cli/commands.go` methods on `App` are the glue between API/cache/convert.
- Pagination logic is generic via the `paginate[T]` function in `client.go`.

### Testing
- Tests live in `_test.go` files alongside the code they test.
- Use `testing.T` from the standard library -- no external test frameworks.
- Table-driven tests are preferred for multiple input/output cases.
- Tests that need filesystem isolation should use `t.TempDir()` and override `$HOME`.
- Test names: `TestFunctionName_Scenario` (e.g., `TestBuildTree_OrphanBecomesRoot`).

### Configuration
- Config file: `~/.config/confluence-reader/config.json`
- Cache directory: `~/.config/confluence-reader/cache/`
- Config fields: `base_url`, `email`, `api_token` (all required).
- Never commit real credentials. `config.json` is in `.gitignore`.

### CLI Conventions
- Help/usage text works without a valid config file.
- All commands that need API access are scoped to a space key.
- Informational output (progress, metadata) goes to stderr.
- Data output (markdown, file contents, tables) goes to stdout.
- Binary file downloads write to the current directory, text to stdout.

## API Notes

- Uses Confluence Cloud REST API **v2** (base path: `/wiki/api/v2/`).
- Auth: HTTP Basic with email + API token.
- Pagination: cursor-based via `_links.next` in responses.
- Page body requested in `storage` format (Confluence XML/HTML).
- Attachments have `downloadLink` or `_links.download` for fetching content.

## Adding a New Command

1. Add the handler method on `App` in `internal/cli/commands.go`.
2. Add the command case in `cmd/confluence/main.go` switch.
3. Update the `usage` constant in `main.go`.
4. Update `README.md` and `.opencode/skills/confluence-reader/SKILL.md` with the new command.
5. Add tests if the command has testable logic.

## Skill File

The file `.opencode/skills/confluence-reader/SKILL.md` is a skill definition used by AI coding tools (OpenCode, Claude, etc.) to understand how to use the `confluence-reader` CLI. It describes available commands, navigation, configuration, and typical workflows.

**Keep the skill file in sync with the README.** When you add, remove, or change commands, options, or usage patterns, update both `README.md` and `SKILL.md`. The command table, configuration section, and workflow examples in the skill file should match the README.

The skill can be installed globally via `make install-skill`.

## Dependencies

This project has zero external dependencies -- standard library only.
Keep it that way unless there is a strong reason to add one.
