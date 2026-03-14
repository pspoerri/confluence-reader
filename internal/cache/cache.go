package cache

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/pspoerri/confluence-reader/internal/api"
	"github.com/pspoerri/confluence-reader/internal/config"
	"github.com/pspoerri/confluence-reader/internal/progress"
)

// RenameEntry records an attachment rename for the mirror command.
type RenameEntry struct {
	NewName   string `json:"new_name"`
	FileID    string `json:"file_id"`
	VersionNo int    `json:"version"`
	FileSize  int64  `json:"file_size"`
}

// CachedSpace stores space metadata and its page tree.
// Pages and Children are populated lazily: Pages holds the full page list
// (set by EnsureFullTree or Refresh), while Children holds per-page child
// lists fetched on demand. Attachments are always fetched lazily per page.
type CachedSpace struct {
	Space        api.Space                         `json:"space"`
	Pages        []api.Page                        `json:"pages"`                   // full page list (populated by EnsureFullTree/Refresh)
	Children     map[string][]api.ChildPage        `json:"children,omitempty"`      // lazy: pageID -> direct children
	Attachments  map[string][]api.Attachment       `json:"attachments,omitempty"`   // lazy: pageID -> attachments
	RenamedFiles map[string]map[string]RenameEntry `json:"renamed_files,omitempty"` // pageID -> originalName -> rename info
	Operations   []api.Operation                   `json:"operations"`              // permitted space operations
	UpdatedAt    time.Time                         `json:"updated_at"`
}

// PageNode is a tree node used for display and traversal.
type PageNode struct {
	Page     api.Page
	Children []*PageNode
}

// Store manages the local page cache.
type Store struct {
	dir string
}

// NewStore creates a cache store. Cache files live under ~/.config/confluence-reader/cache/.
func NewStore() (*Store, error) {
	stateDir, err := config.StateDir()
	if err != nil {
		return nil, err
	}
	dir := filepath.Join(stateDir, "cache")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("create cache dir: %w", err)
	}
	return &Store{dir: dir}, nil
}

func (s *Store) spaceFile(spaceKey string) string {
	return filepath.Join(s.dir, spaceKey+".json")
}

// Save persists a cached space to disk.
func (s *Store) Save(cs *CachedSpace) error {
	data, err := json.MarshalIndent(cs, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal cache: %w", err)
	}
	return os.WriteFile(s.spaceFile(cs.Space.Key), data, 0o644)
}

// Load reads a cached space from disk. Returns nil, nil if not cached.
func (s *Store) Load(spaceKey string) (*CachedSpace, error) {
	data, err := os.ReadFile(s.spaceFile(spaceKey))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read cache: %w", err)
	}

	var cs CachedSpace
	if err := json.Unmarshal(data, &cs); err != nil {
		return nil, fmt.Errorf("decode cache: %w", err)
	}
	return &cs, nil
}

// Remove deletes the cache file for a space.
func (s *Store) Remove(spaceKey string) error {
	err := os.Remove(s.spaceFile(spaceKey))
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// Refresh fetches all pages, attachments, and permissions for a space.
// This is the most complete (and slowest) cache population method.
func (s *Store) Refresh(client *api.Client, space api.Space) (*CachedSpace, error) {
	pages, err := client.GetPagesInSpace(space.ID)
	if err != nil {
		return nil, fmt.Errorf("fetch pages for space %s: %w", space.Key, err)
	}

	// Fetch attachment metadata for every page.
	attachments := make(map[string][]api.Attachment, len(pages))
	bar := progress.New("Fetching attachments", len(pages))
	for _, p := range pages {
		atts, err := client.GetAttachmentsForPage(p.ID)
		if err != nil {
			return nil, fmt.Errorf("fetch attachments for page %s: %w", p.ID, err)
		}
		if len(atts) > 0 {
			attachments[p.ID] = atts
		}
		bar.Increment()
	}
	bar.Finish()

	// Fetch space-level permissions (one call per space).
	fmt.Fprintf(os.Stderr, "Fetching space permissions...\n")
	operations, err := client.GetSpaceOperations(space.ID)
	if err != nil {
		return nil, fmt.Errorf("fetch space operations for %s: %w", space.Key, err)
	}

	cs := &CachedSpace{
		Space:       space,
		Pages:       pages,
		Children:    buildChildrenMap(pages),
		Attachments: attachments,
		Operations:  operations,
		UpdatedAt:   time.Now(),
	}

	if err := s.Save(cs); err != nil {
		return nil, err
	}
	return cs, nil
}

// EnsureSpace loads a cached space from disk, or creates a minimal cache
// with just space metadata and permissions. Unlike EnsureLoaded, this does
// NOT fetch the full page list — pages are loaded lazily as needed.
func (s *Store) EnsureSpace(client *api.Client, space api.Space) (*CachedSpace, error) {
	cs, err := s.Load(space.Key)
	if err != nil {
		return nil, err
	}
	if cs != nil {
		return cs, nil
	}

	// Create minimal cache: space metadata + permissions only.
	fmt.Fprintf(os.Stderr, "Fetching space permissions...\n")
	operations, err := client.GetSpaceOperations(space.ID)
	if err != nil {
		return nil, fmt.Errorf("fetch space operations for %s: %w", space.Key, err)
	}

	cs = &CachedSpace{
		Space:      space,
		Operations: operations,
		UpdatedAt:  time.Now(),
	}

	if err := s.Save(cs); err != nil {
		return nil, err
	}
	return cs, nil
}

// EnsureLoaded returns a fully populated cache (all pages loaded).
// Used by commands that need the complete page tree (tree, find, mirror).
func (s *Store) EnsureLoaded(client *api.Client, space api.Space) (*CachedSpace, error) {
	cs, err := s.EnsureSpace(client, space)
	if err != nil {
		return nil, err
	}
	if err := s.EnsureFullTree(client, cs); err != nil {
		return nil, err
	}
	return cs, nil
}

// EnsureFullTree fetches all pages in the space if not already cached.
func (s *Store) EnsureFullTree(client *api.Client, cs *CachedSpace) error {
	if len(cs.Pages) > 0 {
		return nil
	}

	fmt.Fprintf(os.Stderr, "Fetching page tree...\n")
	pages, err := client.GetPagesInSpace(cs.Space.ID)
	if err != nil {
		return fmt.Errorf("fetch pages for space %s: %w", cs.Space.Key, err)
	}

	cs.Pages = pages
	cs.Children = buildChildrenMap(pages)
	cs.UpdatedAt = time.Now()

	return s.Save(cs)
}

// EnsureChildren returns the direct children of a page, fetching and
// caching them lazily if not already present.
func (s *Store) EnsureChildren(client *api.Client, cs *CachedSpace, pageID string) ([]api.ChildPage, error) {
	// Check the children cache first.
	if cs.Children != nil {
		if children, ok := cs.Children[pageID]; ok {
			return children, nil
		}
	}

	// If the full page list is available, derive children from it.
	if len(cs.Pages) > 0 {
		children := deriveChildren(cs.Pages, pageID)
		if cs.Children == nil {
			cs.Children = make(map[string][]api.ChildPage)
		}
		cs.Children[pageID] = children
		return children, nil
	}

	// Fetch from API.
	children, err := client.GetChildPages(pageID)
	if err != nil {
		return nil, fmt.Errorf("fetch children for page %s: %w", pageID, err)
	}
	if cs.Children == nil {
		cs.Children = make(map[string][]api.ChildPage)
	}
	cs.Children[pageID] = children
	if err := s.Save(cs); err != nil {
		return nil, err
	}
	return children, nil
}

// EnsureAttachments returns the attachments for a page, fetching and
// caching them lazily if not already present.
func (s *Store) EnsureAttachments(client *api.Client, cs *CachedSpace, pageID string) ([]api.Attachment, error) {
	if cs.Attachments != nil {
		if atts, ok := cs.Attachments[pageID]; ok {
			return atts, nil
		}
	}

	atts, err := client.GetAttachmentsForPage(pageID)
	if err != nil {
		return nil, fmt.Errorf("fetch attachments for page %s: %w", pageID, err)
	}
	if cs.Attachments == nil {
		cs.Attachments = make(map[string][]api.Attachment)
	}
	cs.Attachments[pageID] = atts
	if err := s.Save(cs); err != nil {
		return nil, err
	}
	return atts, nil
}

// ResolvePath resolves a slash-separated path (e.g. "/Parent/Child") to a
// page ID by walking the tree lazily, fetching children on demand.
func (s *Store) ResolvePath(client *api.Client, cs *CachedSpace, homepageID, path string) (string, error) {
	path = strings.Trim(path, "/")
	if path == "" {
		return homepageID, nil
	}

	parts := strings.Split(path, "/")
	currentID := homepageID

	for _, part := range parts {
		children, err := s.EnsureChildren(client, cs, currentID)
		if err != nil {
			return "", err
		}
		target := strings.ToLower(part)
		found := false
		for _, child := range children {
			if strings.ToLower(child.Title) == target {
				currentID = child.ID
				found = true
				break
			}
		}
		if !found {
			return "", fmt.Errorf("page not found: %s", part)
		}
	}

	return currentID, nil
}

// buildChildrenMap groups pages by their parent ID, creating a lookup
// from parent page ID to child page list.
func buildChildrenMap(pages []api.Page) map[string][]api.ChildPage {
	children := make(map[string][]api.ChildPage, len(pages))
	// Mark every page as having been checked (even if it has no children).
	for _, p := range pages {
		if _, ok := children[p.ID]; !ok {
			children[p.ID] = nil
		}
		if p.ParentID != "" && p.ParentType == "page" {
			children[p.ParentID] = append(children[p.ParentID], toChildPage(p))
		}
	}
	return children
}

func toChildPage(p api.Page) api.ChildPage {
	return api.ChildPage{
		ID:            p.ID,
		Status:        p.Status,
		Title:         p.Title,
		SpaceID:       p.SpaceID,
		ChildPosition: p.Position,
	}
}

func deriveChildren(pages []api.Page, parentID string) []api.ChildPage {
	var children []api.ChildPage
	for _, p := range pages {
		if p.ParentID == parentID && p.ParentType == "page" {
			children = append(children, toChildPage(p))
		}
	}
	return children
}

// BuildTree builds a tree of PageNodes from a flat list of pages.
func BuildTree(pages []api.Page) []*PageNode {
	nodeMap := make(map[string]*PageNode, len(pages))
	for i := range pages {
		nodeMap[pages[i].ID] = &PageNode{Page: pages[i]}
	}

	var roots []*PageNode
	for _, node := range nodeMap {
		if node.Page.ParentID == "" || node.Page.ParentType != "page" {
			roots = append(roots, node)
			continue
		}
		parent, ok := nodeMap[node.Page.ParentID]
		if ok {
			parent.Children = append(parent.Children, node)
		} else {
			roots = append(roots, node)
		}
	}
	return roots
}

// FindPages searches cached pages by title substring (case-insensitive).
// If query is empty, returns all pages.
func FindPages(pages []api.Page, query string) []api.Page {
	if query == "" {
		return pages
	}
	q := strings.ToLower(query)
	var results []api.Page
	for _, p := range pages {
		if strings.Contains(strings.ToLower(p.Title), q) {
			results = append(results, p)
		}
	}
	return results
}

// FindNode searches a tree for a node by page ID.
func FindNode(roots []*PageNode, pageID string) *PageNode {
	for _, root := range roots {
		if root.Page.ID == pageID {
			return root
		}
		if found := FindNode(root.Children, pageID); found != nil {
			return found
		}
	}
	return nil
}

// FindNodeByPath resolves a slash-separated path (e.g. "/Parent/Child") to a node.
// Path matching is case-insensitive. Leading and trailing slashes are ignored.
func FindNodeByPath(roots []*PageNode, path string) *PageNode {
	path = strings.Trim(path, "/")
	if path == "" {
		return nil
	}
	parts := strings.Split(path, "/")
	return findNodeByParts(roots, parts)
}

func findNodeByParts(nodes []*PageNode, parts []string) *PageNode {
	if len(parts) == 0 || len(nodes) == 0 {
		return nil
	}
	target := strings.ToLower(parts[0])
	for _, n := range nodes {
		if strings.ToLower(n.Page.Title) == target {
			if len(parts) == 1 {
				return n
			}
			return findNodeByParts(n.Children, parts[1:])
		}
	}
	return nil
}

// ChildPages returns the direct children of a page, or the roots if pageID is empty.
func ChildPages(roots []*PageNode, pageID string) []*PageNode {
	if pageID == "" {
		return roots
	}
	node := FindNode(roots, pageID)
	if node == nil {
		return nil
	}
	return node.Children
}

// PagePath returns the slash-separated path from root to the given page.
func PagePath(pages []api.Page, pageID string) string {
	index := make(map[string]*api.Page, len(pages))
	for i := range pages {
		index[pages[i].ID] = &pages[i]
	}

	var parts []string
	visited := make(map[string]bool)
	current := pageID
	for current != "" {
		if visited[current] {
			break
		}
		visited[current] = true
		p, ok := index[current]
		if !ok {
			break
		}
		parts = append([]string{p.Title}, parts...)
		if p.ParentType != "page" {
			break
		}
		current = p.ParentID
	}
	return "/" + strings.Join(parts, "/")
}

// CountNodes returns the total number of nodes in the given trees.
func CountNodes(roots []*PageNode) int {
	n := 0
	for _, r := range roots {
		n += countSubtree(r)
	}
	return n
}

func countSubtree(node *PageNode) int {
	n := 1
	for _, c := range node.Children {
		n += countSubtree(c)
	}
	return n
}
