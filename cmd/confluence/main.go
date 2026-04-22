package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/pspoerri/confluence-reader/internal/cli"
	"github.com/pspoerri/confluence-reader/internal/ui"
	"github.com/pspoerri/confluence-reader/internal/version"
)

const usage = `confluence-reader - browse Confluence spaces like a filesystem

Usage:
  confluence-reader [-v] <command> [flags] [args...]

Commands:
  configure                 Set up Confluence credentials
  spaces                    List all accessible spaces
  ls                        List child pages and attachments (like unix ls)
  tree                      Show the full page hierarchy
  find                      Find pages by title (or list all)
  read                      Read a page as markdown
  read-file                 Download an attachment
  mirror                    Mirror a space to a local directory
  refresh                   Force-refresh the local cache for a space
  version                   Show version, commit, and build time
  help [<command>]          Show this help, or help for a specific command

Run 'confluence-reader <command> --help' for command-specific flags.
Run 'confluence-reader --version' (or -V) for a one-line version banner.

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

  Cache is stored in ~/.config/confluence-reader/cache/.
`

// command bundles a subcommand's name, help text, and handler.
type command struct {
	name    string
	summary string
	run     func(app *cli.App, args []string) error
}

// commands is the registry of subcommands that need an authenticated App.
// configure / help / spaces are handled separately in main.
var commands = []command{
	{"spaces", "List all accessible spaces", runSpaces},
	{"ls", "List child pages and attachments", runLs},
	{"tree", "Show the page hierarchy", runTree},
	{"find", "Find pages by title", runFind},
	{"read", "Read a page as markdown", runRead},
	{"read-file", "Download an attachment", runReadFile},
	{"mirror", "Mirror a space to a local directory", runMirror},
	{"refresh", "Force-refresh the local cache for a space", runRefresh},
}

func main() {
	args := os.Args[1:]

	// Extract the global -v/--verbose flag wherever it appears before the command.
	verbose := false
	for len(args) > 0 && (args[0] == "-v" || args[0] == "--verbose") {
		verbose = true
		args = args[1:]
	}

	out := ui.Stderr()

	if len(args) < 1 {
		fmt.Fprint(os.Stderr, usage)
		os.Exit(1)
	}

	name := args[0]
	rest := args[1:]

	// Commands that don't need a configured client.
	switch name {
	case "help", "-h", "--help":
		printHelp(rest)
		return
	case "version", "-V", "--version":
		fmt.Print(version.Read().Detailed())
		return
	case "configure":
		if err := cli.RunConfigure(out); err != nil {
			out.Errorf("%v", err)
			os.Exit(1)
		}
		return
	}

	cmd, ok := lookupCommand(name)
	if !ok {
		fmt.Fprintf(os.Stderr, "unknown command: %s\n\n", name)
		fmt.Fprint(os.Stderr, usage)
		os.Exit(1)
	}

	app, err := cli.NewApp(verbose, out)
	if err != nil {
		out.Errorf("%v", err)
		os.Exit(1)
	}

	if err := cmd.run(app, rest); err != nil {
		out.Errorf("%v", err)
		os.Exit(1)
	}
}

func lookupCommand(name string) (command, bool) {
	for _, c := range commands {
		if c.name == name {
			return c, true
		}
	}
	return command{}, false
}

// printHelp prints the global usage or, if a command name is given, that
// command's --help output.
func printHelp(args []string) {
	if len(args) == 0 {
		fmt.Fprint(os.Stdout, usage)
		return
	}
	cmd, ok := lookupCommand(args[0])
	if !ok {
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", args[0])
		os.Exit(1)
	}
	// Trigger the subcommand's own --help via FlagSet. Each handler creates
	// its own FlagSet, so we invoke the handler with --help and let it exit.
	_ = cmd.run(nil, []string{"--help"})
}

// newFlagSet returns a FlagSet whose usage line shows the given positional
// args after the command name.
func newFlagSet(name, positional string) *flag.FlagSet {
	fs := flag.NewFlagSet(name, flag.ExitOnError)
	fs.Usage = func() {
		header := fmt.Sprintf("Usage: confluence-reader %s [flags]", name)
		if positional != "" {
			header += " " + positional
		}
		fmt.Fprintln(fs.Output(), header)
		fmt.Fprintln(fs.Output(), "\nFlags:")
		fs.PrintDefaults()
	}
	return fs
}

// boolFlag registers both -short and --long for the same target variable so
// users can write either form.
func boolFlag(fs *flag.FlagSet, target *bool, short, long, usage string) {
	fs.BoolVar(target, short, false, usage)
	fs.BoolVar(target, long, false, usage+" (alias)")
}

// requireArgs prints the FlagSet usage and returns an error if argc is below min.
func requireArgs(fs *flag.FlagSet, args []string, min int) error {
	if len(args) < min {
		fs.Usage()
		return fmt.Errorf("missing required arguments")
	}
	return nil
}

// --- subcommand handlers ---------------------------------------------------

func runSpaces(app *cli.App, args []string) error {
	fs := newFlagSet("spaces", "")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if app == nil {
		return nil
	}
	return app.RunSpaces()
}

func runLs(app *cli.App, args []string) error {
	fs := newFlagSet("ls", "<space-key> [page-id|/path]")
	var longFormat, allFiles, refresh bool
	boolFlag(fs, &longFormat, "l", "long", "Detailed listing (permissions, timestamps, authors)")
	boolFlag(fs, &allFiles, "a", "all", "Include all attachments, not just those referenced in the page")
	boolFlag(fs, &refresh, "r", "refresh", "Force a cache refresh before running")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if app == nil {
		return nil
	}
	rest := fs.Args()
	if err := requireArgs(fs, rest, 1); err != nil {
		return err
	}
	target := ""
	if len(rest) >= 2 {
		target = rest[1]
	}
	return app.RunLs(rest[0], target, longFormat, allFiles, refresh)
}

func runTree(app *cli.App, args []string) error {
	fs := newFlagSet("tree", "<space-key>")
	var refresh bool
	boolFlag(fs, &refresh, "r", "refresh", "Force a cache refresh before running")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if app == nil {
		return nil
	}
	rest := fs.Args()
	if err := requireArgs(fs, rest, 1); err != nil {
		return err
	}
	return app.RunTree(rest[0], refresh)
}

func runFind(app *cli.App, args []string) error {
	fs := newFlagSet("find", "<space-key> [query]")
	var refresh bool
	boolFlag(fs, &refresh, "r", "refresh", "Force a cache refresh before running")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if app == nil {
		return nil
	}
	rest := fs.Args()
	if err := requireArgs(fs, rest, 1); err != nil {
		return err
	}
	query := ""
	if len(rest) >= 2 {
		query = rest[1]
	}
	return app.RunFind(rest[0], query, refresh)
}

func runRead(app *cli.App, args []string) error {
	fs := newFlagSet("read", "<space-key> <page-id|/path>")
	var refresh bool
	boolFlag(fs, &refresh, "r", "refresh", "Force a cache refresh before running")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if app == nil {
		return nil
	}
	rest := fs.Args()
	if err := requireArgs(fs, rest, 2); err != nil {
		return err
	}
	return app.RunRead(rest[0], rest[1], refresh)
}

func runReadFile(app *cli.App, args []string) error {
	fs := newFlagSet("read-file", "<space-key> <page-id|/path> <filename>")
	var refresh bool
	boolFlag(fs, &refresh, "r", "refresh", "Force a cache refresh before running")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if app == nil {
		return nil
	}
	rest := fs.Args()
	if err := requireArgs(fs, rest, 3); err != nil {
		return err
	}
	return app.RunReadFile(rest[0], rest[1], rest[2], refresh)
}

func runMirror(app *cli.App, args []string) error {
	fs := newFlagSet("mirror", "<space-key> <target-dir>")
	var allFiles, refresh bool
	boolFlag(fs, &allFiles, "a", "all", "Include all attachments, not just those referenced in pages")
	boolFlag(fs, &refresh, "r", "refresh", "Force a cache refresh before running")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if app == nil {
		return nil
	}
	rest := fs.Args()
	if err := requireArgs(fs, rest, 2); err != nil {
		return err
	}
	return app.RunMirror(rest[0], rest[1], allFiles, refresh)
}

func runRefresh(app *cli.App, args []string) error {
	fs := newFlagSet("refresh", "<space-key>")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if app == nil {
		return nil
	}
	rest := fs.Args()
	if err := requireArgs(fs, rest, 1); err != nil {
		return err
	}
	return app.RunRefresh(rest[0])
}
