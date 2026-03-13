package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/pascal/confluence-reader/internal/cli"
)

const usage = `confluence-reader - browse Confluence spaces like a filesystem

Usage:
  confluence-reader [options] <command> [args...]

Options:
  -v, --verbose                          Enable verbose/debug output

Commands:
  configure                              Set up Confluence credentials
  spaces                                 List all accessible spaces
  ls [-l] <space-key> [page-id|/path]    List child pages (like unix ls)
  tree <space-key>                       List page tree in a space
  find <space-key> [query]               Find pages by title (or list all)
  read <space-key> <page-id|/path>      Read a page as markdown
  read-file <space-key> <page-id|/path> <filename>
                                         Download an attachment
  refresh <space-key>                    Refresh the local cache for a space

Configuration:
  Run 'confluence-reader configure' to set up your credentials.
  You need your Confluence base URL, email, and an API token.
  Create an API token at: https://id.atlassian.com/manage-profile/security/api-tokens

  Config is stored in ~/.config/confluence-reader/config.json:

  {
    "base_url": "https://your-domain.atlassian.net",
    "email": "you@example.com",
    "api_token": "your-api-token"
  }

Environment:
  Cache is stored in ~/.config/confluence-reader/cache/
`

func main() {
	// Parse global flags before the command.
	args := os.Args[1:]
	verbose := false
	for len(args) > 0 && (args[0] == "-v" || args[0] == "--verbose") {
		verbose = true
		args = args[1:]
	}

	if len(args) < 1 {
		fmt.Fprint(os.Stderr, usage)
		os.Exit(1)
	}

	cmd := args[0]
	args = args[1:]

	// Handle commands that work without a config file.
	switch cmd {
	case "help", "-h", "--help":
		fmt.Fprint(os.Stdout, usage)
		return
	case "configure":
		if err := cli.RunConfigure(); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		return
	}

	app, err := cli.NewApp(verbose)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	switch cmd {
	case "spaces":
		err = app.RunSpaces()

	case "ls":
		// Parse ls-specific flags: -l for long format.
		longFormat := false
		lsArgs := args
		for len(lsArgs) > 0 && strings.HasPrefix(lsArgs[0], "-") {
			switch lsArgs[0] {
			case "-l", "--long":
				longFormat = true
			default:
				die("ls: unknown flag: " + lsArgs[0])
			}
			lsArgs = lsArgs[1:]
		}
		if len(lsArgs) < 1 {
			die("usage: confluence-reader ls [-l] <space-key> [page-id|/path]")
		}
		target := ""
		if len(lsArgs) >= 2 {
			target = lsArgs[1]
		}
		err = app.RunLs(lsArgs[0], target, longFormat)

	case "tree":
		if len(args) < 1 {
			die("usage: confluence-reader tree <space-key>")
		}
		err = app.RunTree(args[0])

	case "find":
		if len(args) < 1 {
			die("usage: confluence-reader find <space-key> [query]")
		}
		query := ""
		if len(args) >= 2 {
			query = args[1]
		}
		err = app.RunFind(args[0], query)

	case "read":
		if len(args) < 2 {
			die("usage: confluence-reader read <space-key> <page-id|/path>")
		}
		err = app.RunRead(args[0], args[1])

	case "read-file":
		if len(args) < 3 {
			die("usage: confluence-reader read-file <space-key> <page-id|/path> <filename>")
		}
		err = app.RunReadFile(args[0], args[1], args[2])

	case "refresh":
		if len(args) < 1 {
			die("usage: confluence-reader refresh <space-key>")
		}
		err = app.RunRefresh(args[0])

	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n\n", cmd)
		fmt.Fprint(os.Stderr, usage)
		os.Exit(1)
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func die(msg string) {
	fmt.Fprintln(os.Stderr, msg)
	os.Exit(1)
}
