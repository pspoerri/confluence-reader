package cache

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/pascal/confluence-reader/internal/api"
	"github.com/pascal/confluence-reader/internal/config"
)

// CachedSpace stores space metadata and its page tree.
type CachedSpace struct {
	Space     api.Space  `json:"space"`
	Pages     []api.Page `json:"pages"`
	UpdatedAt time.Time  `json:"updated_at"`
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

// Refresh fetches all pages in a space from the API and caches them.
func (s *Store) Refresh(client *api.Client, space api.Space) (*CachedSpace, error) {
	pages, err := client.GetPagesInSpace(space.ID)
	if err != nil {
		return nil, fmt.Errorf("fetch pages for space %s: %w", space.Key, err)
	}

	cs := &CachedSpace{
		Space:     space,
		Pages:     pages,
		UpdatedAt: time.Now(),
	}

	if err := s.Save(cs); err != nil {
		return nil, err
	}
	return cs, nil
}

// EnsureLoaded loads from cache, or fetches from the API if not cached.
func (s *Store) EnsureLoaded(client *api.Client, space api.Space) (*CachedSpace, error) {
	cs, err := s.Load(space.Key)
	if err != nil {
		return nil, err
	}
	if cs != nil {
		return cs, nil
	}
	return s.Refresh(client, space)
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

// PagePath returns the slash-separated path from root to the given page.
func PagePath(pages []api.Page, pageID string) string {
	index := make(map[string]*api.Page, len(pages))
	for i := range pages {
		index[pages[i].ID] = &pages[i]
	}

	var parts []string
	current := pageID
	for current != "" {
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
