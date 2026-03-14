package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/pspoerri/confluence-reader/internal/cli"
)

const usage = `confluence-reader - browse Confluence spaces like a filesystem

Usage:
  confluence-reader [options] <command> [args...]

Options:
  -v, --verbose                          Enable verbose/debug output

Commands:
  configure                              Set up Confluence credentials
  spaces                                 List all accessible spaces
  ls [-l] [-r] <space-key> [page-id|/path]
                                         List child pages (like unix ls)
  tree [-r] <space-key>                  List page tree in a space
  find [-r] <space-key> [query]          Find pages by title (or list all)
  read [-r] <space-key> <page-id|/path>  Read a page as markdown
  read-file [-r] <space-key> <page-id|/path> <filename>
                                         Download an attachment
  mirror [-r] <space-key> <target-dir>   Mirror entire space to local directory
  refresh <space-key>                    Refresh the local cache for a space

Flags:
  -l, --long       Show detailed listing (ls only)
  -r, --refresh    Force a cache refresh before running the command

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
		longFormat := false
		refresh := false
		lsArgs := args
		for len(lsArgs) > 0 && strings.HasPrefix(lsArgs[0], "-") {
			switch lsArgs[0] {
			case "-l", "--long":
				longFormat = true
			case "-r", "--refresh":
				refresh = true
			default:
				die("ls: unknown flag: " + lsArgs[0])
			}
			lsArgs = lsArgs[1:]
		}
		if len(lsArgs) < 1 {
			die("usage: confluence-reader ls [-l] [-r] <space-key> [page-id|/path]")
		}
		target := ""
		if len(lsArgs) >= 2 {
			target = lsArgs[1]
		}
		err = app.RunLs(lsArgs[0], target, longFormat, refresh)

	case "tree":
		refresh, cmdArgs := parseRefreshFlag(args)
		if len(cmdArgs) < 1 {
			die("usage: confluence-reader tree [-r] <space-key>")
		}
		err = app.RunTree(cmdArgs[0], refresh)

	case "find":
		refresh, cmdArgs := parseRefreshFlag(args)
		if len(cmdArgs) < 1 {
			die("usage: confluence-reader find [-r] <space-key> [query]")
		}
		query := ""
		if len(cmdArgs) >= 2 {
			query = cmdArgs[1]
		}
		err = app.RunFind(cmdArgs[0], query, refresh)

	case "read":
		refresh, cmdArgs := parseRefreshFlag(args)
		if len(cmdArgs) < 2 {
			die("usage: confluence-reader read [-r] <space-key> <page-id|/path>")
		}
		err = app.RunRead(cmdArgs[0], cmdArgs[1], refresh)

	case "read-file":
		refresh, cmdArgs := parseRefreshFlag(args)
		if len(cmdArgs) < 3 {
			die("usage: confluence-reader read-file [-r] <space-key> <page-id|/path> <filename>")
		}
		err = app.RunReadFile(cmdArgs[0], cmdArgs[1], cmdArgs[2], refresh)

	case "mirror":
		refresh, cmdArgs := parseRefreshFlag(args)
		if len(cmdArgs) < 2 {
			die("usage: confluence-reader mirror [-r] <space-key> <target-dir>")
		}
		err = app.RunMirror(cmdArgs[0], cmdArgs[1], refresh)

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

// parseRefreshFlag extracts -r/--refresh from arguments, returning the
// flag value and the remaining arguments.
func parseRefreshFlag(args []string) (bool, []string) {
	refresh := false
	remaining := args
	for len(remaining) > 0 && strings.HasPrefix(remaining[0], "-") {
		switch remaining[0] {
		case "-r", "--refresh":
			refresh = true
		default:
			die("unknown flag: " + remaining[0])
		}
		remaining = remaining[1:]
	}
	return refresh, remaining
}

func die(msg string) {
	fmt.Fprintln(os.Stderr, msg)
	os.Exit(1)
}
