package cli

import (
	"strings"

	"github.com/pspoerri/confluence-reader/internal/api"
	"github.com/pspoerri/confluence-reader/internal/cache"
	"github.com/pspoerri/confluence-reader/internal/convert"
)

// liveMacroResolver implements convert.MacroResolver using the API client and cache.
type liveMacroResolver struct {
	client *api.Client
	store  *cache.Store
	cs     *cache.CachedSpace
	pageID string
}

func (r *liveMacroResolver) CurrentPageID() string {
	return r.pageID
}

func (r *liveMacroResolver) ChildPages(pageID string) ([]convert.PageInfo, error) {
	children, err := r.store.EnsureChildren(r.client, r.cs, pageID)
	if err != nil {
		return nil, err
	}
	result := make([]convert.PageInfo, len(children))
	for i, c := range children {
		result[i] = convert.PageInfo{ID: c.ID, Title: c.Title}
	}
	return result, nil
}

func (r *liveMacroResolver) PageByTitle(title string) *convert.PageInfo {
	target := strings.ToLower(title)
	for _, p := range r.cs.Pages {
		if strings.ToLower(p.Title) == target {
			return &convert.PageInfo{ID: p.ID, Title: p.Title}
		}
	}
	return nil
}

func (r *liveMacroResolver) SubTree(pageID string) ([]convert.TreeEntry, error) {
	// If the full page list is loaded, use the tree structure.
	if len(r.cs.Pages) > 0 {
		roots := cache.BuildTree(r.cs.Pages)
		node := cache.FindNode(roots, pageID)
		if node == nil {
			return nil, nil
		}
		var entries []convert.TreeEntry
		walkTree(node.Children, 0, &entries)
		return entries, nil
	}

	// Otherwise, recursively fetch children on demand.
	return r.buildSubTree(pageID, 0)
}

func walkTree(nodes []*cache.PageNode, depth int, entries *[]convert.TreeEntry) {
	for _, n := range nodes {
		*entries = append(*entries, convert.TreeEntry{
			Page:  convert.PageInfo{ID: n.Page.ID, Title: n.Page.Title},
			Depth: depth,
		})
		walkTree(n.Children, depth+1, entries)
	}
}

func (r *liveMacroResolver) buildSubTree(pageID string, depth int) ([]convert.TreeEntry, error) {
	children, err := r.store.EnsureChildren(r.client, r.cs, pageID)
	if err != nil {
		return nil, err
	}
	var entries []convert.TreeEntry
	for _, c := range children {
		entries = append(entries, convert.TreeEntry{
			Page:  convert.PageInfo{ID: c.ID, Title: c.Title},
			Depth: depth,
		})
		sub, err := r.buildSubTree(c.ID, depth+1)
		if err != nil {
			return nil, err
		}
		entries = append(entries, sub...)
	}
	return entries, nil
}
