package cli

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"text/tabwriter"
	"time"

	"github.com/pspoerri/confluence-reader/internal/api"
	"github.com/pspoerri/confluence-reader/internal/cache"
	"github.com/pspoerri/confluence-reader/internal/config"
	"github.com/pspoerri/confluence-reader/internal/convert"
	"github.com/pspoerri/confluence-reader/internal/progress"
	"github.com/pspoerri/confluence-reader/internal/ui"
)

// App holds the shared state for all CLI commands.
type App struct {
	Client *api.Client
	Cache  *cache.Store
	UI     *ui.Writer
}

// NewApp creates a new App from the config file.
func NewApp(verbose bool, out *ui.Writer) (*App, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}

	client := api.NewClient(cfg.BaseURL, cfg.Email, cfg.APIToken)
	client.Verbose = verbose
	store, err := cache.NewStore(out)
	if err != nil {
		return nil, err
	}

	return &App{Client: client, Cache: store, UI: out}, nil
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
		a.UI.Infof("Refreshing cache for space %s (%s)...", space.Key, space.Name)
		return a.Cache.Refresh(a.Client, *space)
	}
	return a.Cache.EnsureSpace(a.Client, *space)
}

// ensureLoaded loads a fully populated cache, optionally forcing a refresh.
func (a *App) ensureLoaded(space *api.Space, refresh bool) (*cache.CachedSpace, error) {
	if refresh {
		a.UI.Infof("Refreshing cache for space %s (%s)...", space.Key, space.Name)
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
// When allFiles is false, only attachments referenced in the page body are shown.
func (a *App) RunLs(spaceKey, target string, longFormat, allFiles, refresh bool) error {
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
	var prefetched *api.Page
	if target != "" && target != "/" {
		pageID, prefetched, err = a.resolveTarget(cs, space, target)
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

	// When filtering, fetch the page body to determine which attachments are referenced.
	// Reuse the prefetched page from resolveTarget when available.
	var referenced map[string]bool
	if !allFiles && len(attachments) > 0 {
		page, err := a.fetchPage(prefetched, pageID)
		if err == nil && page.Body != nil && page.Body.Storage != nil {
			referenced = convert.ReferencedAttachments(page.Body.Storage.Value)
		}
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
		if referenced != nil && !referenced[att.Title] {
			continue
		}
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
		a.UI.Infof("\n%d items", len(entries))
	} else {
		for _, e := range entries {
			fmt.Println(e.Name)
		}
	}
	return nil
}

// resolveTarget resolves a target that can be a page ID, a slash-separated
// path (e.g. "/Parent/Child"), or a bare page title to a page ID. Uses the
// full tree if cached, otherwise resolves lazily.
//
// When the target is a numeric ID we fetch the page to validate it exists;
// the fetched page is returned via prefetched so callers can reuse it instead
// of issuing a second GetPageByID. prefetched is nil for path/title targets.
func (a *App) resolveTarget(cs *cache.CachedSpace, space *api.Space, target string) (pageID string, prefetched *api.Page, err error) {
	target = strings.TrimSuffix(target, "/index.md")
	target = strings.TrimSuffix(target, "/index.MD")

	// Numeric page ID: try directly.
	if !strings.Contains(target, "/") && isNumeric(target) {
		if p, err := a.Client.GetPageByID(target); err == nil {
			return target, p, nil
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
			return node.Page.ID, nil, nil
		}
		return "", nil, fmt.Errorf("page not found: %s", target)
	}

	// Lazy path resolution.
	if strings.HasPrefix(target, "/") || !strings.Contains(target, "/") {
		id, err := a.Cache.ResolvePath(a.Client, cs, space.HomepageID, target)
		return id, nil, err
	}

	return "", nil, fmt.Errorf("page not found: %s", target)
}

// fetchPage returns prefetched if non-nil, otherwise fetches the page by ID.
// Used by callers of resolveTarget to avoid a duplicate GetPageByID round trip.
func (a *App) fetchPage(prefetched *api.Page, pageID string) (*api.Page, error) {
	if prefetched != nil {
		return prefetched, nil
	}
	return a.Client.GetPageByID(pageID)
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

	pageID, prefetched, err := a.resolveTarget(cs, space, target)
	if err != nil {
		return err
	}

	page, err := a.fetchPage(prefetched, pageID)
	if err != nil {
		return fmt.Errorf("fetch page: %w", err)
	}

	if page.SpaceID != space.ID {
		return fmt.Errorf("page %s does not belong to space %s", pageID, spaceKey)
	}

	// Get attachments (lazily cached).
	attachments, err := a.Cache.EnsureAttachments(a.Client, cs, pageID)
	if err != nil {
		return fmt.Errorf("fetch attachments: %w", err)
	}

	// Convert body.
	var body string
	if page.Body != nil && page.Body.Storage != nil {
		body = page.Body.Storage.Value
	}

	resolver := &liveMacroResolver{
		client: a.Client,
		store:  a.Cache,
		cs:     cs,
		pageID: pageID,
	}
	result := convert.ToMarkdown(body, attachments, resolver)
	if len(result.UnknownTags) > 0 {
		a.UI.Warnf("unhandled tags: %s", strings.Join(result.UnknownTags, ", "))
	}

	// Print header.
	fmt.Printf("# %s\n\n", page.Title)
	fmt.Printf("> Page ID: %s | Version: %d | Updated: %s\n\n", page.ID, page.Version.Number, page.Version.CreatedAt)
	fmt.Println(result.Markdown)

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

	pageID, prefetched, err := a.resolveTarget(cs, space, target)
	if err != nil {
		return err
	}

	// Verify page scope.
	page, err := a.fetchPage(prefetched, pageID)
	if err != nil {
		return err
	}
	if page.SpaceID != space.ID {
		return fmt.Errorf("page %s does not belong to space %s", pageID, spaceKey)
	}

	attachments, err := a.Cache.EnsureAttachments(a.Client, cs, pageID)
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
	a.UI.Infof("Content-Type: %s", contentType)
	a.UI.Infof("Filename: %s", att.Title)
	a.UI.Infof("Size: %d bytes", att.FileSize)

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
	a.UI.Successf("Written to %s (%d bytes)", safeName, n)
	return nil
}

// RunRefresh forces a cache refresh for a space.
func (a *App) RunRefresh(spaceKey string) error {
	space, err := a.resolveSpace(spaceKey)
	if err != nil {
		return err
	}

	a.UI.Infof("Refreshing cache for space %s (%s)...", space.Key, space.Name)

	cs, err := a.Cache.Refresh(a.Client, *space)
	if err != nil {
		return err
	}

	a.UI.Successf("Cached %d pages (at %s)", len(cs.Pages), cs.UpdatedAt.Format("2006-01-02 15:04:05"))
	return nil
}

// mirrorWorkers caps the number of concurrent page-processing goroutines.
// Kept low because Confluence Cloud throttles aggressively above a small
// concurrency level — higher values trigger 429s and end up slower overall.
const mirrorWorkers = 2

// pageJob is one page's worth of work prepared in mirror's serial phase 1
// and consumed by parallel workers in phase 2.
type pageJob struct {
	pageID      string
	pageTitle   string
	dir         string
	attachments []api.Attachment
	renameMap   map[string]string
}

// RunMirror mirrors an entire Confluence space into a local directory.
// Each page becomes a folder with an index.md file and its attachments.
// When allFiles is false, only attachments referenced in the page body are downloaded.
//
// Producer/consumer pipeline:
//
//   - The producer (this goroutine) walks the page tree, mkdir-s each page
//     directory, fetches the attachment listing for each page, and pushes a
//     pageJob onto a channel.
//   - mirrorWorkers worker goroutines drain that channel concurrently:
//     fetch page body, convert to markdown, write index.md, download
//     attachments, render any drawio diagrams.
//
// The producer must run serially because EnsureAttachments mutates cs. The
// pipeline overlaps producer and worker time so the progress bar advances as
// soon as the first job completes (around the time the producer is fetching
// the second page's attachments). cs.RenamedFiles updates are deferred to a
// post-pipeline pass to keep workers' map reads race-free.
func (a *App) RunMirror(spaceKey, targetDir string, allFiles, refresh bool) error {
	space, err := a.resolveSpace(spaceKey)
	if err != nil {
		return err
	}

	cs, err := a.ensureLoaded(space, refresh)
	if err != nil {
		return err
	}

	if len(cs.Pages) == 0 {
		a.UI.Infof("No pages in space %s.", spaceKey)
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

	// If the target directory exists, verify it belongs to this space.
	if info, statErr := os.Stat(targetDir); statErr == nil && info.IsDir() {
		if meta, err := readSyncMetadata(targetDir); err == nil && meta != nil {
			if !strings.EqualFold(meta.SpaceKey, spaceKey) {
				return fmt.Errorf("target directory %s was previously mirrored from space %q, not %q; use a different directory", targetDir, meta.SpaceKey, spaceKey)
			}
		}
	}

	// Track all paths written during this mirror run. Mutex-guarded since
	// the producer (mkdir) and workers (file writes) both append to it.
	written := make(map[string]bool)
	var writeMu sync.Mutex
	markWritten := func(path string) {
		writeMu.Lock()
		written[path] = true
		writeMu.Unlock()
	}

	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		return fmt.Errorf("create target directory: %w", err)
	}

	// Spin up workers, then stream jobs from the producer.
	var (
		errOnce  sync.Once
		firstErr error
	)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	captureErr := func(err error) {
		if err == nil {
			return
		}
		errOnce.Do(func() {
			firstErr = err
			cancel()
		})
	}

	// Buffered so the producer can stay a few jobs ahead of the workers
	// without blocking on every send.
	jobCh := make(chan pageJob, mirrorWorkers*2)
	var wg sync.WaitGroup
	for i := 0; i < mirrorWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for job := range jobCh {
				if ctx.Err() != nil {
					continue // drain remaining jobs after cancellation
				}
				if err := a.processJob(cs, job, allFiles, bar, markWritten); err != nil {
					captureErr(err)
				}
			}
		}()
	}

	// Producer: walk the tree and feed jobs to the workers.
	var jobs []pageJob
	emit := func(job pageJob) bool {
		jobs = append(jobs, job)
		select {
		case <-ctx.Done():
			return false
		case jobCh <- job:
			return true
		}
	}

	produce := func() error {
		if homepageNode != nil {
			job, err := a.prepareJob(cs, homepageNode.Page.ID, homepageNode.Page.Title, targetDir, markWritten)
			if err != nil {
				return fmt.Errorf("prepare homepage: %w", err)
			}
			if !emit(job) {
				return nil
			}
		}
		for _, node := range displayRoots {
			if err := a.streamJobs(ctx, cs, node, targetDir, markWritten, emit); err != nil {
				return err
			}
		}
		return nil
	}

	if err := produce(); err != nil {
		captureErr(err)
	}
	close(jobCh)
	wg.Wait()
	bar.Finish()

	if firstErr != nil {
		return firstErr
	}

	// Persist rename info now that no workers are reading cs.RenamedFiles.
	updateRenameCache(cs, jobs)

	if err := writeSyncMetadata(targetDir, spaceKey, total); err != nil {
		return fmt.Errorf("write sync metadata: %w", err)
	}

	removed := cleanStaleEntries(targetDir, written)
	if removed > 0 {
		a.UI.Infof("Removed %d stale files/directories", removed)
	}

	if err := a.Cache.Save(cs); err != nil {
		return fmt.Errorf("save cache: %w", err)
	}

	a.UI.Successf("Mirrored space %s to %s (%d pages)", spaceKey, targetDir, total)
	return nil
}

// streamJobs walks node's subtree, mkdir-ing each page directory and emitting
// one pageJob per page via emit. emit returns false when the consumer side has
// cancelled and the walk should stop early.
func (a *App) streamJobs(ctx context.Context, cs *cache.CachedSpace, node *cache.PageNode, parentDir string, markWritten func(string), emit func(pageJob) bool) error {
	if ctx.Err() != nil {
		return nil
	}

	dirName := sanitizeFilename(node.Page.Title)
	dir := filepath.Join(parentDir, dirName)

	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create directory %s: %w", dir, err)
	}
	markWritten(dir)

	job, err := a.prepareJob(cs, node.Page.ID, node.Page.Title, dir, markWritten)
	if err != nil {
		return err
	}
	if !emit(job) {
		return nil
	}

	for _, child := range node.Children {
		if err := a.streamJobs(ctx, cs, child, dir, markWritten, emit); err != nil {
			return err
		}
	}
	return nil
}

// prepareJob fetches the attachment listing for a page (cached in cs) and
// builds the rename map. cs.RenamedFiles is intentionally NOT updated here:
// workers read it concurrently via downloadAttachment and a write would race.
// updateRenameCache flushes the new entries once the worker pool drains.
func (a *App) prepareJob(cs *cache.CachedSpace, pageID, pageTitle, dir string, markWritten func(string)) (pageJob, error) {
	_ = markWritten // currently unused; reserved for future per-page metadata files
	attachments, err := a.Cache.EnsureAttachments(a.Client, cs, pageID)
	if err != nil {
		return pageJob{}, fmt.Errorf("fetch attachments for page %s: %w", pageID, err)
	}

	renameMap := make(map[string]string, len(attachments))
	for _, att := range attachments {
		renameMap[att.Title] = convert.RenameAttachment(pageTitle, att.Title, att.ID)
	}

	return pageJob{
		pageID:      pageID,
		pageTitle:   pageTitle,
		dir:         dir,
		attachments: attachments,
		renameMap:   renameMap,
	}, nil
}

// updateRenameCache writes the rename info for every job into cs.RenamedFiles
// so subsequent mirror runs can skip already-downloaded attachments. Must run
// after the worker pool has drained, since workers read cs.RenamedFiles and
// concurrent map writes would race.
func updateRenameCache(cs *cache.CachedSpace, jobs []pageJob) {
	if cs.RenamedFiles == nil {
		cs.RenamedFiles = make(map[string]map[string]cache.RenameEntry, len(jobs))
	}
	for _, job := range jobs {
		if len(job.attachments) == 0 {
			continue
		}
		entries := cs.RenamedFiles[job.pageID]
		if entries == nil {
			entries = make(map[string]cache.RenameEntry, len(job.attachments))
			cs.RenamedFiles[job.pageID] = entries
		}
		for _, att := range job.attachments {
			entries[att.Title] = cache.RenameEntry{
				NewName:   job.renameMap[att.Title],
				FileID:    att.FileID,
				VersionNo: att.Version.Number,
				FileSize:  att.FileSize,
			}
		}
	}
}

// processJob fetches a page body, converts to markdown, writes index.md,
// and downloads attachments. Safe to run concurrently — cs is read-only here
// and the only shared mutable state (the `written` map and the progress bar)
// is locked internally.
func (a *App) processJob(cs *cache.CachedSpace, job pageJob, allFiles bool, bar *progress.Bar, markWritten func(string)) error {
	page, err := a.Client.GetPageByID(job.pageID)
	if err != nil {
		return fmt.Errorf("fetch page %s: %w", job.pageID, err)
	}

	var body string
	if page.Body != nil && page.Body.Storage != nil {
		body = page.Body.Storage.Value
	}

	resolver := &liveMacroResolver{
		client: a.Client,
		store:  a.Cache,
		cs:     cs,
		pageID: job.pageID,
	}
	result := convert.ToMarkdown(body, job.attachments, resolver)
	if len(result.UnknownTags) > 0 {
		bar.Log("warning: unhandled tags in %s: %s", page.Title, strings.Join(result.UnknownTags, ", "))
	}
	md := result.Markdown

	// Rewrite attachment:filename references to renamed local filenames.
	for origName, newName := range job.renameMap {
		md = strings.ReplaceAll(md, "(attachment:"+origName+")", "("+newName+")")
	}
	// Catch any remaining attachment: references (shouldn't happen, but safety).
	md = strings.ReplaceAll(md, "(attachment:", "(")

	downloadAtts := job.attachments
	if !allFiles {
		referenced := convert.ReferencedAttachments(body)
		var filtered []api.Attachment
		for _, att := range job.attachments {
			if referenced[att.Title] {
				filtered = append(filtered, att)
			}
		}
		downloadAtts = filtered
	}

	fm := Frontmatter{
		Title:      page.Title,
		PageID:     page.ID,
		Version:    page.Version.Number,
		CreatedAt:  page.CreatedAt,
		AuthorID:   page.AuthorID,
		ModifiedAt: page.Version.CreatedAt,
		ModifiedBy: page.Version.AuthorID,
	}
	if page.Links.WebUI != "" {
		fm.Source = a.Client.BaseURL + "/wiki" + page.Links.WebUI
	}

	content := fmt.Sprintf("%s\n# %s\n\n%s\n", fm.Encode(), page.Title, md)
	indexPath := filepath.Join(job.dir, "index.md")
	if err := os.WriteFile(indexPath, []byte(content), 0o644); err != nil {
		return fmt.Errorf("write %s: %w", indexPath, err)
	}
	markWritten(indexPath)
	bar.Increment()

	for _, att := range downloadAtts {
		newName := job.renameMap[att.Title]
		outPath := filepath.Join(job.dir, sanitizeFilename(newName))
		markWritten(outPath)
		if err := a.downloadAttachment(cs, att, job.pageID, newName, job.dir); err != nil {
			bar.Log("warning: %v", err)
		}
	}

	return nil
}

// downloadAttachment downloads a single attachment into dir using the
// given filename. Skips the download if the file already exists on disk
// with matching size from a previous mirror run.
func (a *App) downloadAttachment(cs *cache.CachedSpace, att api.Attachment, pageID, filename, dir string) error {
	outPath := filepath.Join(dir, sanitizeFilename(filename))

	// Skip download if file exists and matches cached metadata.
	if info, err := os.Stat(outPath); err == nil {
		if cached, ok := cs.RenamedFiles[pageID][att.Title]; ok {
			if cached.VersionNo == att.Version.Number && info.Size() == cached.FileSize {
				return nil // already up to date
			}
		}
	}

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

// syncMetadataFile is the name of the file written to the mirror root to
// identify it as a confluence-reader mirror directory.
const syncMetadataFile = ".confluence-sync.json"

// syncMetadata is the JSON structure stored in the sync metadata file.
type syncMetadata struct {
	SpaceKey  string `json:"space_key"`
	SyncedAt  string `json:"synced_at"`
	PageCount int    `json:"page_count"`
}

// writeSyncMetadata writes a metadata file to the mirror root.
func writeSyncMetadata(targetDir, spaceKey string, pageCount int) error {
	meta := syncMetadata{
		SpaceKey:  spaceKey,
		SyncedAt:  time.Now().UTC().Format(time.RFC3339),
		PageCount: pageCount,
	}
	data, err := json.Marshal(meta)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(targetDir, syncMetadataFile), data, 0o644)
}

// readSyncMetadata reads the sync metadata from a mirror directory.
// Returns nil if the file does not exist.
func readSyncMetadata(targetDir string) (*syncMetadata, error) {
	data, err := os.ReadFile(filepath.Join(targetDir, syncMetadataFile))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var meta syncMetadata
	if err := json.Unmarshal(data, &meta); err != nil {
		return nil, err
	}
	return &meta, nil
}

// cleanStaleEntries removes files and empty directories under root that
// are not in the written set. Returns the number of entries removed.
func cleanStaleEntries(root string, written map[string]bool) int {
	removed := 0
	// Collect all entries first, then remove in reverse order so that
	// child entries are removed before their parent directories.
	var entries []string
	filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		// Never remove the root directory itself, the sync metadata file,
		// or any git-related files/directories (.git, .gitignore, etc.).
		base := filepath.Base(path)
		if path == root || base == syncMetadataFile || strings.HasPrefix(base, ".git") {
			if info.IsDir() && strings.HasPrefix(base, ".git") {
				return filepath.SkipDir
			}
			return nil
		}
		entries = append(entries, path)
		return nil
	})

	// Process in reverse order (deepest paths first).
	for i := len(entries) - 1; i >= 0; i-- {
		path := entries[i]
		if written[path] {
			continue
		}
		info, err := os.Stat(path)
		if err != nil {
			continue
		}
		if info.IsDir() {
			// Only remove if empty (all contents already cleaned).
			dirEntries, err := os.ReadDir(path)
			if err != nil || len(dirEntries) > 0 {
				continue
			}
			os.Remove(path)
		} else {
			os.Remove(path)
		}
		removed++
	}
	return removed
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

// unsafeFilenameChars replaces filesystem-unsafe characters with underscores.
var unsafeFilenameChars = strings.NewReplacer(
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

// sanitizeFilename produces a safe single-segment file or directory name from
// arbitrary input. Path separators and shell-unsafe characters are replaced
// with underscores, then filepath.Base provides defense in depth so the
// result cannot escape the target directory.
func sanitizeFilename(name string) string {
	name = unsafeFilenameChars.Replace(strings.TrimSpace(name))
	name = filepath.Base(name)
	if name == "." || name == ".." || name == "" {
		return "_"
	}
	return name
}

// RunConfigure interactively prompts for Basic Auth credentials and writes the config file.
func RunConfigure(out *ui.Writer) error {
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

	out.Successf("Config saved to %s", cfgPath)
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
