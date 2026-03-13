package cli

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/pascal/confluence-reader/internal/api"
	"github.com/pascal/confluence-reader/internal/cache"
	"github.com/pascal/confluence-reader/internal/config"
	"github.com/pascal/confluence-reader/internal/convert"
)

// App holds the shared state for all CLI commands.
type App struct {
	Client *api.Client
	Cache  *cache.Store
}

// NewApp creates a new App from the config file.
func NewApp(verbose bool) (*App, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}

	client := api.NewClient(cfg.BaseURL, cfg.Email, cfg.APIToken)
	client.Verbose = verbose
	store, err := cache.NewStore()
	if err != nil {
		return nil, err
	}

	return &App{Client: client, Cache: store}, nil
}

// RunSpaces lists all accessible spaces.
func (a *App) RunSpaces() error {
	spaces, err := a.Client.GetSpaces()
	if err != nil {
		return fmt.Errorf("fetch spaces: %w", err)
	}

	if len(spaces) == 0 {
		fmt.Println("No spaces found.")
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	fmt.Fprintf(w, "KEY\tNAME\tTYPE\tSTATUS\n")
	fmt.Fprintf(w, "---\t----\t----\t------\n")
	for _, s := range spaces {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", s.Key, s.Name, s.Type, s.Status)
	}
	w.Flush()
	return nil
}

// resolveSpace finds a space by key from the API.
func (a *App) resolveSpace(spaceKey string) (*api.Space, error) {
	spaces, err := a.Client.GetSpaces()
	if err != nil {
		return nil, err
	}
	key := strings.ToUpper(spaceKey)
	for _, s := range spaces {
		if strings.ToUpper(s.Key) == key {
			return &s, nil
		}
	}
	return nil, fmt.Errorf("space %q not found", spaceKey)
}

// RunLS lists pages in a space as a tree.
func (a *App) RunLS(spaceKey string) error {
	space, err := a.resolveSpace(spaceKey)
	if err != nil {
		return err
	}

	cs, err := a.Cache.EnsureLoaded(a.Client, *space)
	if err != nil {
		return err
	}

	if len(cs.Pages) == 0 {
		fmt.Printf("No pages in space %s.\n", spaceKey)
		return nil
	}

	roots := cache.BuildTree(cs.Pages)
	sortNodes(roots)
	for _, root := range roots {
		printTree(os.Stdout, root, "", true)
	}

	fmt.Printf("\n(%d pages, cached %s)\n", len(cs.Pages), cs.UpdatedAt.Format("2006-01-02 15:04:05"))
	return nil
}

func sortNodes(nodes []*cache.PageNode) {
	sort.Slice(nodes, func(i, j int) bool {
		return nodes[i].Page.Position < nodes[j].Page.Position
	})
	for _, n := range nodes {
		sortNodes(n.Children)
	}
}

func printTree(w io.Writer, node *cache.PageNode, prefix string, last bool) {
	connector := "├── "
	if last {
		connector = "└── "
	}
	fmt.Fprintf(w, "%s%s%s [%s]\n", prefix, connector, node.Page.Title, node.Page.ID)

	childPrefix := prefix + "│   "
	if last {
		childPrefix = prefix + "    "
	}

	for i, child := range node.Children {
		printTree(w, child, childPrefix, i == len(node.Children)-1)
	}
}

// RunFind searches for pages matching a query within a space.
func (a *App) RunFind(spaceKey, query string) error {
	space, err := a.resolveSpace(spaceKey)
	if err != nil {
		return err
	}

	cs, err := a.Cache.EnsureLoaded(a.Client, *space)
	if err != nil {
		return err
	}

	results := cache.FindPages(cs.Pages, query)
	if len(results) == 0 {
		fmt.Println("No pages found.")
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	fmt.Fprintf(w, "ID\tTITLE\tPATH\n")
	fmt.Fprintf(w, "--\t-----\t----\n")
	for _, p := range results {
		path := cache.PagePath(cs.Pages, p.ID)
		fmt.Fprintf(w, "%s\t%s\t%s\n", p.ID, p.Title, path)
	}
	w.Flush()

	fmt.Printf("\n(%d results)\n", len(results))
	return nil
}

// RunRead reads a page by ID, converts to markdown, includes attachment refs.
func (a *App) RunRead(spaceKey, pageID string) error {
	// Verify page belongs to the space scope.
	space, err := a.resolveSpace(spaceKey)
	if err != nil {
		return err
	}

	page, err := a.Client.GetPageByID(pageID)
	if err != nil {
		return fmt.Errorf("fetch page: %w", err)
	}

	if page.SpaceID != space.ID {
		return fmt.Errorf("page %s does not belong to space %s", pageID, spaceKey)
	}

	// Get attachments.
	attachments, err := a.Client.GetAttachmentsForPage(pageID)
	if err != nil {
		return fmt.Errorf("fetch attachments: %w", err)
	}

	// Convert body.
	var body string
	if page.Body != nil && page.Body.Storage != nil {
		body = page.Body.Storage.Value
	}

	md := convert.ToMarkdown(body, attachments)

	// Print header.
	fmt.Printf("# %s\n\n", page.Title)
	fmt.Printf("> Page ID: %s | Version: %d | Updated: %s\n\n", page.ID, page.Version.Number, page.Version.CreatedAt)
	fmt.Println(md)

	return nil
}

// RunReadFile downloads an attachment by page ID and filename.
func (a *App) RunReadFile(spaceKey, pageID, filename string) error {
	space, err := a.resolveSpace(spaceKey)
	if err != nil {
		return err
	}

	// Verify page scope.
	page, err := a.Client.GetPageByID(pageID)
	if err != nil {
		return err
	}
	if page.SpaceID != space.ID {
		return fmt.Errorf("page %s does not belong to space %s", pageID, spaceKey)
	}

	attachments, err := a.Client.GetAttachmentsForPage(pageID)
	if err != nil {
		return err
	}

	var target *api.Attachment
	for _, a := range attachments {
		if strings.EqualFold(a.Title, filename) {
			target = &a
			break
		}
	}
	if target == nil {
		return fmt.Errorf("attachment %q not found on page %s", filename, pageID)
	}

	downloadPath := target.Links.Download
	if downloadPath == "" {
		downloadPath = target.DownloadLink
	}
	if downloadPath == "" {
		return fmt.Errorf("no download link for attachment %q", filename)
	}

	resp, err := a.Client.DownloadAttachment(downloadPath)
	if err != nil {
		return fmt.Errorf("download attachment: %w", err)
	}
	defer resp.Body.Close()

	contentType := resp.Header.Get("Content-Type")
	fmt.Fprintf(os.Stderr, "Content-Type: %s\n", contentType)
	fmt.Fprintf(os.Stderr, "Filename: %s\n", target.Title)
	fmt.Fprintf(os.Stderr, "Size: %d bytes\n", target.FileSize)

	if strings.HasPrefix(contentType, "text/") {
		_, err = io.Copy(os.Stdout, resp.Body)
	} else {
		// Write binary to file in current directory.
		outFile, ferr := os.Create(target.Title)
		if ferr != nil {
			return fmt.Errorf("create output file: %w", ferr)
		}
		defer outFile.Close()
		n, cerr := io.Copy(outFile, resp.Body)
		if cerr != nil {
			return fmt.Errorf("write output file: %w", cerr)
		}
		fmt.Fprintf(os.Stderr, "Written to %s (%d bytes)\n", target.Title, n)
	}

	return err
}

// RunRefresh forces a cache refresh for a space.
func (a *App) RunRefresh(spaceKey string) error {
	space, err := a.resolveSpace(spaceKey)
	if err != nil {
		return err
	}

	fmt.Fprintf(os.Stderr, "Refreshing cache for space %s (%s)...\n", space.Key, space.Name)

	cs, err := a.Cache.Refresh(a.Client, *space)
	if err != nil {
		return err
	}

	fmt.Fprintf(os.Stderr, "Cached %d pages (at %s)\n", len(cs.Pages), cs.UpdatedAt.Format("2006-01-02 15:04:05"))
	return nil
}

// RunConfigure interactively prompts for Basic Auth credentials and writes the config file.
func RunConfigure() error {
	cfgPath, err := config.Path()
	if err != nil {
		return err
	}

	// Load existing config as defaults if present.
	existing, _ := config.Load()

	reader := bufio.NewReader(os.Stdin)

	fmt.Fprintf(os.Stderr, "Configuring confluence-reader (%s)\n\n", cfgPath)

	baseURL := prompt(reader, "Base URL (e.g. https://your-domain.atlassian.net)", defaultVal(existing, func(c *config.Config) string { return c.BaseURL }))
	email := prompt(reader, "Email", defaultVal(existing, func(c *config.Config) string { return c.Email }))
	apiToken := prompt(reader, "API Token", defaultVal(existing, func(c *config.Config) string { return c.APIToken }))

	// Normalize: strip trailing slash from base URL.
	baseURL = strings.TrimRight(baseURL, "/")

	cfg := &config.Config{
		BaseURL:  baseURL,
		Email:    email,
		APIToken: apiToken,
	}

	if err := config.Save(cfg); err != nil {
		return err
	}

	fmt.Fprintf(os.Stderr, "\nConfig saved to %s\n", cfgPath)
	return nil
}

// prompt asks the user for a value with an optional default.
func prompt(reader *bufio.Reader, label, defaultValue string) string {
	if defaultValue != "" {
		// Mask tokens for display.
		display := defaultValue
		if strings.Contains(strings.ToLower(label), "token") && len(defaultValue) > 8 {
			display = defaultValue[:4] + "..." + defaultValue[len(defaultValue)-4:]
		}
		fmt.Fprintf(os.Stderr, "%s [%s]: ", label, display)
	} else {
		fmt.Fprintf(os.Stderr, "%s: ", label)
	}

	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(input)

	if input == "" {
		return defaultValue
	}
	return input
}

// defaultVal safely extracts a field from an existing config that may be nil.
func defaultVal(cfg *config.Config, fn func(*config.Config) string) string {
	if cfg == nil {
		return ""
	}
	return fn(cfg)
}
