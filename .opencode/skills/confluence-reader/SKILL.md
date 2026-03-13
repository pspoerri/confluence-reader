---
name: confluence-reader
description: Browse and read Confluence Cloud spaces, pages, and attachments from the terminal using the confluence-reader CLI tool
---

## What I do

I help you interact with Confluence Cloud using the `confluence-reader` CLI tool. This tool exposes Confluence spaces as a filesystem-like hierarchy, letting you browse pages, read content as Markdown, and download attachments -- all from the command line.

## Available commands

| Command | Usage | Description |
|---------|-------|-------------|
| `configure` | `confluence-reader configure` | Interactively set up Confluence credentials (base URL, email, API token) |
| `spaces` | `confluence-reader spaces` | List all accessible Confluence spaces |
| `ls` | `confluence-reader ls [-l] <space-key> [page-id\|/path]` | List child pages and attachments (like unix `ls`). Use `-l` for long format with permissions |
| `tree` | `confluence-reader tree <space-key>` | Display the full page hierarchy as a tree |
| `find` | `confluence-reader find <space-key> [query]` | Search pages by title substring (case-insensitive) |
| `read` | `confluence-reader read <space-key> <page-id\|/path>` | Read a page converted to Markdown |
| `read-file` | `confluence-reader read-file <space-key> <page-id\|/path> <filename>` | Download an attachment (text to stdout, binary to file) |
| `refresh` | `confluence-reader refresh <space-key>` | Force-refresh the local cache for a space |

## When to use me

Use this skill when you need to:

- Look up documentation stored in Confluence
- Read the content of a specific Confluence page
- Browse the page hierarchy of a Confluence space
- Find pages by title
- Download attachments from Confluence pages
- Set up or troubleshoot Confluence credentials

## How navigation works

Pages are addressed either by **page ID** or by **path**. Paths use `/` separators mirroring the page hierarchy:

```
confluence-reader ls MYSPACE /Engineering/Architecture
confluence-reader read MYSPACE /Engineering/Architecture/ADR-001
```

The `ls` command shows `index.md` (the page's own content) alongside child pages (shown as directories) and attachments (shown as files).

## Configuration

The tool reads credentials from `~/.config/confluence-reader/config.json`:

```json
{
  "base_url": "https://your-domain.atlassian.net",
  "email": "you@example.com",
  "api_token": "your-api-token"
}
```

Run `confluence-reader configure` for interactive setup. API tokens are created at https://id.atlassian.com/manage-profile/security/api-tokens

## Caching

Page trees are cached locally under `~/.config/confluence-reader/cache/`. Use `confluence-reader refresh <space-key>` to rebuild the cache for a space.

## Typical workflow

1. List available spaces: `confluence-reader spaces`
2. Browse a space: `confluence-reader ls SPACEKEY` or `confluence-reader tree SPACEKEY`
3. Find a page: `confluence-reader find SPACEKEY "search term"`
4. Read a page: `confluence-reader read SPACEKEY /path/to/page`
5. Download an attachment: `confluence-reader read-file SPACEKEY /path/to/page filename.pdf`

## Options

- `-v` / `--verbose` -- Enable verbose/debug output (global flag, placed before the command)

## Notes

- The tool uses Confluence Cloud REST API v2 with HTTP Basic authentication
- Page content is automatically converted from Confluence storage format (HTML) to Markdown
- Attachment references in pages are rendered as `attachment:filename` links
- Binary file downloads are saved to the current working directory
- Informational output goes to stderr; data output goes to stdout
