package cli

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/pspoerri/confluence-reader/internal/api"
	"github.com/pspoerri/confluence-reader/internal/cache"
	"github.com/pspoerri/confluence-reader/internal/config"
	"github.com/pspoerri/confluence-reader/internal/convert"
	"github.com/pspoerri/confluence-reader/internal/progress"
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

// ensureSpace loads or creates the cache, optionally forcing a full refresh.
func (a *App) ensureSpace(space *api.Space, refresh bool) (*cache.CachedSpace, error) {
	if refresh {
		fmt.Fprintf(os.Stderr, "Refreshing cache for space %s (%s)...\n", space.Key, space.Name)
		return a.Cache.Refresh(a.Client, *space)
	}
	return a.Cache.EnsureSpace(a.Client, *space)
}

// ensureLoaded loads a fully populated cache, optionally forcing a refresh.
func (a *App) ensureLoaded(space *api.Space, refresh bool) (*cache.CachedSpace, error) {
	if refresh {
		fmt.Fprintf(os.Stderr, "Refreshing cache for space %s (%s)...\n", space.Key, space.Name)
		return a.Cache.Refresh(a.Client, *space)
	}
	return a.Cache.EnsureLoaded(a.Client, *space)
}

// lsEntry is a single item in an ls listing. It can represent a child page
// (shown as a directory), the virtual index.md, or an attachment file.
type lsEntry struct {
	Perms    string // unix-style permissions, e.g. "rwx"
	Name     string // display name, e.g. "ChildPage/", "index.md", "photo.png"
	Modified string // formatted timestamp
	Creator  string // author/editor ID
}

// RunLs lists pages like unix ls. Uses lazy caching: only fetches the
// children and attachments of the viewed page, not the entire space.
func (a *App) RunLs(spaceKey, target string, longFormat, refresh bool) error {
	space, err := a.resolveSpace(spaceKey)
	if err != nil {
		return err
	}

	cs, err := a.ensureSpace(space, refresh)
	if err != nil {
		return err
	}

	perms := operationsToPerms(cs.Operations)

	// Resolve target to a page ID. Default to the homepage.
	pageID := space.HomepageID
	if pageID == "" {
		fmt.Println("No pages found.")
		return nil
	}
	if target != "" && target != "/" {
		pageID, err = a.resolveTarget(cs, space, target)
		if err != nil {
			return err
		}
	}

	// Fetch children and attachments lazily for this page.
	children, err := a.Cache.EnsureChildren(a.Client, cs, pageID)
	if err != nil {
		return err
	}
	sort.Slice(children, func(i, j int) bool {
		return children[i].ChildPosition < children[j].ChildPosition
	})

	attachments, err := a.Cache.EnsureAttachments(a.Client, cs, pageID)
	if err != nil {
		return err
	}

	// Build entries.
	var entries []lsEntry

	// index.md for the page content.
	indexEntry := lsEntry{Perms: perms, Name: "index.md", Modified: "-", Creator: "-"}
	if p := findPageInList(cs.Pages, pageID); p != nil {
		indexEntry.Modified = formatTime(p.Version.CreatedAt)
		indexEntry.Creator = p.Version.AuthorID
	}
	entries = append(entries, indexEntry)

	// Child pages as directories.
	for _, child := range children {
		entry := lsEntry{Perms: perms, Name: child.Title + "/", Modified: "-", Creator: "-"}
		if p := findPageInList(cs.Pages, child.ID); p != nil {
			entry.Modified = formatTime(p.Version.CreatedAt)
			entry.Creator = p.Version.AuthorID
		}
		entries = append(entries, entry)
	}

	// Attachments as files.
	for _, att := range attachments {
		entries = append(entries, lsEntry{
			Perms:    perms,
			Name:     att.Title,
			Modified: formatTime(att.Version.CreatedAt),
			Creator:  att.Version.AuthorID,
		})
	}

	if len(entries) == 0 {
		fmt.Println("No pages found.")
		return nil
	}

	// Display.
	if longFormat {
		w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
		fmt.Fprintf(w, "PERMS\tNAME\tMODIFIED\tCREATOR\n")
		fmt.Fprintf(w, "-----\t----\t--------\t-------\n")
		for _, e := range entries {
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", e.Perms, e.Name, e.Modified, e.Creator)
		}
		w.Flush()
		fmt.Fprintf(os.Stderr, "\n%d items\n", len(entries))
	} else {
		for _, e := range entries {
			fmt.Println(e.Name)
		}
	}
	return nil
}

// resolveTarget resolves a target that can be a page ID, a slash-separated
// path (e.g. "/Parent/Child"), or a bare page title to a page ID.
// Uses the full tree if cached, otherwise resolves lazily.
func (a *App) resolveTarget(cs *cache.CachedSpace, space *api.Space, target string) (string, error) {
	target = strings.TrimSuffix(target, "/index.md")
	target = strings.TrimSuffix(target, "/index.MD")

	// Numeric page ID: try directly.
	if !strings.Contains(target, "/") && isNumeric(target) {
		if _, err := a.Client.GetPageByID(target); err == nil {
			return target, nil
		}
	}

	// If the full page list is cached, use tree-based resolution.
	if len(cs.Pages) > 0 {
		roots := cache.BuildTree(cs.Pages)
		visibleRoots := roots
		if space.HomepageID != "" {
			if hp := cache.FindNode(roots, space.HomepageID); hp != nil {
				visibleRoots = hp.Children
			}
		}

		var node *cache.PageNode
		if strings.HasPrefix(target, "/") {
			node = cache.FindNodeByPath(visibleRoots, target)
		} else {
			node = cache.FindNode(roots, target)
			if node == nil {
				node = cache.FindNodeByPath(visibleRoots, target)
			}
		}
		if node != nil {
			return node.Page.ID, nil
		}
		return "", fmt.Errorf("page not found: %s", target)
	}

	// Lazy path resolution.
	if strings.HasPrefix(target, "/") || !strings.Contains(target, "/") {
		return a.Cache.ResolvePath(a.Client, cs, space.HomepageID, target)
	}

	return "", fmt.Errorf("page not found: %s", target)
}

// findPageInList looks up a page by ID in the full page list.
// Returns nil if the page list is empty or the page is not found.
func findPageInList(pages []api.Page, pageID string) *api.Page {
	for i := range pages {
		if pages[i].ID == pageID {
			return &pages[i]
		}
	}
	return nil
}

// formatTime parses an API timestamp and returns a short human-readable form.
func formatTime(raw string) string {
	if raw == "" {
		return "-"
	}
	// Confluence v2 API uses RFC 3339 / ISO 8601.
	for _, layout := range []string{
		"2006-01-02T15:04:05.000Z",
		"2006-01-02T15:04:05Z",
		"2006-01-02T15:04:05.000-07:00",
		"2006-01-02T15:04:05-07:00",
	} {
		if t, err := time.Parse(layout, raw); err == nil {
			return t.Format("Jan 02 15:04")
		}
	}
	// Fallback: return first 16 chars.
	if len(raw) > 16 {
		return raw[:16]
	}
	return raw
}

// operationsToPerms converts a list of Confluence space operations to a
// unix-style permissions string: r=read, w=create/update, x=delete.
func operationsToPerms(ops []api.Operation) string {
	var canRead, canWrite, canDelete bool
	for _, op := range ops {
		switch op.Operation {
		case "read":
			canRead = true
		case "create", "update":
			canWrite = true
		case "delete":
			canDelete = true
		}
	}

	perm := [3]byte{'-', '-', '-'}
	if canRead {
		perm[0] = 'r'
	}
	if canWrite {
		perm[1] = 'w'
	}
	if canDelete {
		perm[2] = 'x'
	}
	return string(perm[:])
}

// RunTree lists pages in a space as a tree.
func (a *App) RunTree(spaceKey string, refresh bool) error {
	space, err := a.resolveSpace(spaceKey)
	if err != nil {
		return err
	}

	cs, err := a.ensureLoaded(space, refresh)
	if err != nil {
		return err
	}

	if len(cs.Pages) == 0 {
		fmt.Printf("No pages in space %s.\n", spaceKey)
		return nil
	}

	roots := cache.BuildTree(cs.Pages)
	sortNodes(roots)

	// Skip the homepage wrapper — show its children as the top-level tree.
	displayRoots := roots
	if space.HomepageID != "" {
		if hp := cache.FindNode(roots, space.HomepageID); hp != nil {
			displayRoots = hp.Children
		}
	}

	for i, root := range displayRoots {
		printTree(os.Stdout, root, "", i == len(displayRoots)-1)
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
func (a *App) RunFind(spaceKey, query string, refresh bool) error {
	space, err := a.resolveSpace(spaceKey)
	if err != nil {
		return err
	}

	cs, err := a.ensureLoaded(space, refresh)
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

// RunRead reads a page by ID or path, converts to markdown, includes attachment refs.
func (a *App) RunRead(spaceKey, target string, refresh bool) error {
	space, err := a.resolveSpace(spaceKey)
	if err != nil {
		return err
	}

	cs, err := a.ensureSpace(space, refresh)
	if err != nil {
		return err
	}

	pageID, err := a.resolveTarget(cs, space, target)
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

// RunReadFile downloads an attachment by page ID (or path) and filename.
func (a *App) RunReadFile(spaceKey, target, filename string, refresh bool) error {
	space, err := a.resolveSpace(spaceKey)
	if err != nil {
		return err
	}

	cs, err := a.ensureSpace(space, refresh)
	if err != nil {
		return err
	}

	pageID, err := a.resolveTarget(cs, space, target)
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

	var att *api.Attachment
	for _, at := range attachments {
		if strings.EqualFold(at.Title, filename) {
			att = &at
			break
		}
	}
	if att == nil {
		return fmt.Errorf("attachment %q not found on page %s", filename, pageID)
	}

	downloadPath := att.Links.Download
	if downloadPath == "" {
		downloadPath = att.DownloadLink
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
	fmt.Fprintf(os.Stderr, "Filename: %s\n", att.Title)
	fmt.Fprintf(os.Stderr, "Size: %d bytes\n", att.FileSize)

	if strings.HasPrefix(contentType, "text/") {
		_, err = io.Copy(os.Stdout, resp.Body)
		return err
	}

	// Write binary to file in current directory.
	safeName := sanitizeFilename(att.Title)
	outFile, err := os.Create(safeName)
	if err != nil {
		return fmt.Errorf("create output file: %w", err)
	}
	defer outFile.Close()
	n, err := io.Copy(outFile, resp.Body)
	if err != nil {
		return fmt.Errorf("write output file: %w", err)
	}
	fmt.Fprintf(os.Stderr, "Written to %s (%d bytes)\n", safeName, n)
	return nil
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

// RunMirror mirrors an entire Confluence space into a local directory.
// Each page becomes a folder with an index.md file and its attachments.
func (a *App) RunMirror(spaceKey, targetDir string, refresh bool) error {
	space, err := a.resolveSpace(spaceKey)
	if err != nil {
		return err
	}

	cs, err := a.ensureLoaded(space, refresh)
	if err != nil {
		return err
	}

	if len(cs.Pages) == 0 {
		fmt.Fprintf(os.Stderr, "No pages in space %s.\n", spaceKey)
		return nil
	}

	roots := cache.BuildTree(cs.Pages)
	sortNodes(roots)

	// Skip homepage wrapper — show its children as top-level entries.
	displayRoots := roots
	var homepageNode *cache.PageNode
	if space.HomepageID != "" {
		homepageNode = cache.FindNode(roots, space.HomepageID)
		if homepageNode != nil {
			displayRoots = homepageNode.Children
		}
	}

	// Count total pages for the progress bar.
	total := cache.CountNodes(displayRoots)
	if homepageNode != nil {
		total++
	}
	bar := progress.New("Mirroring pages", total)

	// Create target directory.
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		return fmt.Errorf("create target directory: %w", err)
	}

	// Download homepage content into the root directory.
	if homepageNode != nil {
		if err := a.downloadPage(homepageNode.Page.ID, targetDir, bar); err != nil {
			return fmt.Errorf("download homepage: %w", err)
		}
	}

	// Download all child pages recursively.
	for _, node := range displayRoots {
		if err := a.downloadTree(node, targetDir, bar); err != nil {
			return err
		}
	}
	bar.Finish()

	fmt.Fprintf(os.Stderr, "Mirrored space %s to %s (%d pages)\n", spaceKey, targetDir, total)
	return nil
}

// downloadTree recursively downloads a page node and its children.
func (a *App) downloadTree(node *cache.PageNode, parentDir string, bar *progress.Bar) error {
	dirName := sanitizeName(node.Page.Title)
	dir := filepath.Join(parentDir, dirName)

	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create directory %s: %w", dir, err)
	}

	if err := a.downloadPage(node.Page.ID, dir, bar); err != nil {
		return err
	}

	for _, child := range node.Children {
		if err := a.downloadTree(child, dir, bar); err != nil {
			return err
		}
	}
	return nil
}

// downloadPage fetches a page, converts it to markdown, saves index.md,
// and downloads all attachments into the same directory.
func (a *App) downloadPage(pageID, dir string, bar *progress.Bar) error {
	page, err := a.Client.GetPageByID(pageID)
	if err != nil {
		return fmt.Errorf("fetch page %s: %w", pageID, err)
	}

	attachments, err := a.Client.GetAttachmentsForPage(pageID)
	if err != nil {
		return fmt.Errorf("fetch attachments for page %s: %w", pageID, err)
	}

	// Convert to markdown.
	var body string
	if page.Body != nil && page.Body.Storage != nil {
		body = page.Body.Storage.Value
	}
	md := convert.ToMarkdown(body, attachments)

	// Rewrite attachment:filename references to local filenames.
	md = strings.ReplaceAll(md, "(attachment:", "(")

	var meta strings.Builder
	meta.WriteString("---\n")
	fmt.Fprintf(&meta, "title: %q\n", page.Title)
	fmt.Fprintf(&meta, "page_id: %s\n", page.ID)
	fmt.Fprintf(&meta, "version: %d\n", page.Version.Number)
	if page.CreatedAt != "" {
		fmt.Fprintf(&meta, "created_at: %s\n", page.CreatedAt)
	}
	fmt.Fprintf(&meta, "author_id: %s\n", page.AuthorID)
	if page.Version.CreatedAt != "" {
		fmt.Fprintf(&meta, "modified_at: %s\n", page.Version.CreatedAt)
	}
	fmt.Fprintf(&meta, "modified_by: %s\n", page.Version.AuthorID)
	if page.Links.WebUI != "" {
		fmt.Fprintf(&meta, "source: %s%s\n", a.Client.BaseURL, page.Links.WebUI)
	}
	meta.WriteString("---\n\n")

	content := fmt.Sprintf("%s# %s\n\n%s\n", meta.String(), page.Title, md)
	indexPath := filepath.Join(dir, "index.md")
	if err := os.WriteFile(indexPath, []byte(content), 0o644); err != nil {
		return fmt.Errorf("write %s: %w", indexPath, err)
	}
	bar.Increment()

	// Download attachments into the same directory.
	for _, att := range attachments {
		if err := a.downloadAttachment(att, dir); err != nil {
			bar.Log("warning: %v", err)
		}
	}

	return nil
}

// downloadAttachment downloads a single attachment into dir.
func (a *App) downloadAttachment(att api.Attachment, dir string) error {
	downloadPath := att.Links.Download
	if downloadPath == "" {
		downloadPath = att.DownloadLink
	}
	if downloadPath == "" {
		return fmt.Errorf("no download link for %q", att.Title)
	}

	resp, err := a.Client.DownloadAttachment(downloadPath)
	if err != nil {
		return fmt.Errorf("download %q: %w", att.Title, err)
	}
	defer resp.Body.Close()

	outPath := filepath.Join(dir, sanitizeFilename(att.Title))
	f, err := os.Create(outPath)
	if err != nil {
		return fmt.Errorf("create %s: %w", outPath, err)
	}
	defer f.Close()

	if _, err := io.Copy(f, resp.Body); err != nil {
		return fmt.Errorf("write %s: %w", outPath, err)
	}

	return nil
}

// isNumeric returns true if s consists entirely of ASCII digits.
func isNumeric(s string) bool {
	if s == "" {
		return false
	}
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

// sanitizeFilename strips path separators and traversal sequences from a
// filename so it cannot escape the target directory.
func sanitizeFilename(name string) string {
	// Use only the base name to prevent directory traversal.
	name = filepath.Base(name)
	if name == "." || name == ".." || name == "" {
		name = "_"
	}
	return name
}

// sanitizeName replaces characters that are invalid in file/directory names.
func sanitizeName(name string) string {
	replacer := strings.NewReplacer(
		"/", "_",
		"\\", "_",
		":", "_",
		"*", "_",
		"?", "_",
		"\"", "_",
		"<", "_",
		">", "_",
		"|", "_",
	)
	s := replacer.Replace(name)
	s = strings.TrimSpace(s)
	if s == "" {
		s = "_"
	}
	return s
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
