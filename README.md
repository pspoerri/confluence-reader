# confluence-reader

A CLI tool that lets you browse Confluence Cloud spaces like a filesystem. Read pages as Markdown, list page hierarchies, search by title, and download attachments -- all from your terminal.

Built with Go. Zero external dependencies.

## Features

- **Filesystem-like navigation** -- browse Confluence pages using familiar commands (`ls`, `tree`, `find`)
- **Markdown output** -- page content is converted from Confluence storage format to Markdown
- **Local caching** -- page trees are cached locally for fast repeated access
- **Attachment support** -- list and download file attachments from pages
- **Path-based addressing** -- reference pages by slash-separated paths (e.g. `/Engineering/ADR-001`) or by page ID

## Installation

Requires Go 1.26.1 or later.

```bash
# Clone the repository
git clone https://github.com/pspoerri/confluence-reader.git
cd confluence-reader

# Build and install to ~/.local/bin/
make install
```

Or build without installing:

```bash
make build
# Binary: ./confluence-reader
```

## Configuration

You need a Confluence Cloud base URL, your email, and an API token. Create an API token at: https://id.atlassian.com/manage-profile/security/api-tokens

### Interactive setup

```bash
confluence-reader configure
```

This prompts for your credentials and saves them to `~/.config/confluence-reader/config.json`.

### Manual setup

Create `~/.config/confluence-reader/config.json`:

```json
{
  "base_url": "https://your-domain.atlassian.net",
  "email": "you@example.com",
  "api_token": "your-api-token"
}
```

## Usage

```
confluence-reader [options] <command> [args...]
```

### Global options

| Option | Description |
|--------|-------------|
| `-v`, `--verbose` | Enable verbose/debug output |

### Commands

| Command | Usage | Description |
|---------|-------|-------------|
| `configure` | `confluence-reader configure` | Set up Confluence credentials interactively |
| `spaces` | `confluence-reader spaces` | List all accessible spaces |
| `ls` | `confluence-reader ls [-l] <space-key> [page-id\|/path]` | List child pages and attachments |
| `tree` | `confluence-reader tree <space-key>` | Display the full page hierarchy as a tree |
| `find` | `confluence-reader find <space-key> [query]` | Search pages by title (case-insensitive) |
| `read` | `confluence-reader read <space-key> <page-id\|/path>` | Read a page as Markdown |
| `read-file` | `confluence-reader read-file <space-key> <page-id\|/path> <filename>` | Download an attachment |
| `refresh` | `confluence-reader refresh <space-key>` | Force-refresh the local cache for a space |

### Examples

```bash
# List all spaces you have access to
confluence-reader spaces

# Browse the root pages of a space
confluence-reader ls MYSPACE

# List pages with detailed info (permissions, timestamps, authors)
confluence-reader ls -l MYSPACE

# Navigate into a specific page path
confluence-reader ls MYSPACE /Engineering/Architecture

# Show the full page tree
confluence-reader tree MYSPACE

# Find pages by title
confluence-reader find MYSPACE "onboarding"

# Read a page as Markdown (by path)
confluence-reader read MYSPACE /Engineering/Architecture/ADR-001

# Read a page as Markdown (by page ID)
confluence-reader read MYSPACE 12345678

# Download an attachment
confluence-reader read-file MYSPACE /Engineering/Architecture photo.png

# Force-refresh the cached page tree
confluence-reader refresh MYSPACE
```

### How navigation works

Pages are addressed either by **page ID** or by **slash-separated path** mirroring the Confluence page hierarchy. The `ls` command presents each page as a directory containing:

- `index.md` -- the page's own content
- Child pages (shown with a trailing `/`)
- Attachments (shown as files)

Text attachments are printed to stdout; binary files are saved to the current directory.

## Caching

Page trees are cached locally under `~/.config/confluence-reader/cache/`. The cache is populated automatically on first access to a space and can be refreshed with `confluence-reader refresh <space-key>`.

## Project structure

```
cmd/confluence/main.go       -- CLI entry point, argument parsing
internal/
  api/
    client.go                -- HTTP client, auth, pagination
    types.go                 -- API response types
  cache/
    cache.go                 -- Local JSON cache, tree builder, search
  cli/
    commands.go              -- Command implementations
  config/
    config.go                -- Config file handling
  convert/
    markdown.go              -- Confluence HTML to Markdown conversion
config.example.json          -- Template config file
Makefile                     -- Build, test, install targets
```

## Development

### Build and test

```bash
make all          # Format check + vet + test + build
make build        # Build the binary
make test         # Run all tests
make test-v       # Run tests with verbose output
make test-race    # Run tests with race detector
make vet          # Static analysis
make fmt          # Format all Go files
make fmt-check    # Check formatting (CI)
make clean        # Remove build artifacts
```

### AI tool skill

confluence-reader ships with a skill definition (`.opencode/skills/confluence-reader/SKILL.md`) that lets AI coding tools (OpenCode, Claude, etc.) use the tool to browse Confluence. To install the skill globally for OpenCode:

```bash
make install-skill
```

This copies the skill definition to `~/.config/opencode/skills/confluence-reader/`.

## License

See the repository for license information.
