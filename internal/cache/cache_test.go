package cache

import (
	"testing"

	"github.com/pspoerri/confluence-reader/internal/api"
)

func TestBuildTree(t *testing.T) {
	// Confluence v2 API: all pages have parentType="page". The homepage's
	// parentId references an ID outside the fetched set, making it the root.
	pages := []api.Page{
		{ID: "1", Title: "Home", ParentID: "ext-space", ParentType: "page"},
		{ID: "2", Title: "Child A", ParentID: "1", ParentType: "page"},
		{ID: "3", Title: "Child B", ParentID: "1", ParentType: "page"},
		{ID: "4", Title: "Grandchild", ParentID: "2", ParentType: "page"},
	}

	roots := BuildTree(pages)

	if len(roots) != 1 {
		t.Fatalf("expected 1 root, got %d", len(roots))
	}
	if roots[0].Page.Title != "Home" {
		t.Errorf("expected root title 'Home', got %q", roots[0].Page.Title)
	}
	if len(roots[0].Children) != 2 {
		t.Fatalf("expected 2 children, got %d", len(roots[0].Children))
	}
}

func TestBuildTree_MultipleRoots(t *testing.T) {
	// Pages whose parentId is not in the set all become roots.
	pages := []api.Page{
		{ID: "1", Title: "Root A", ParentID: "ext-1", ParentType: "page"},
		{ID: "2", Title: "Root B", ParentID: "ext-2", ParentType: "page"},
	}

	roots := BuildTree(pages)

	if len(roots) != 2 {
		t.Fatalf("expected 2 roots, got %d", len(roots))
	}
}

func TestBuildTree_OrphanBecomesRoot(t *testing.T) {
	pages := []api.Page{
		{ID: "2", Title: "Orphan", ParentID: "999", ParentType: "page"},
	}

	roots := BuildTree(pages)

	if len(roots) != 1 {
		t.Fatalf("expected 1 root (orphan), got %d", len(roots))
	}
}

func TestFindPages_EmptyQuery(t *testing.T) {
	pages := []api.Page{
		{ID: "1", Title: "Alpha"},
		{ID: "2", Title: "Beta"},
	}

	results := FindPages(pages, "")
	if len(results) != 2 {
		t.Errorf("expected 2 results for empty query, got %d", len(results))
	}
}

func TestFindPages_CaseInsensitive(t *testing.T) {
	pages := []api.Page{
		{ID: "1", Title: "Getting Started"},
		{ID: "2", Title: "API Reference"},
		{ID: "3", Title: "started guide"},
	}

	results := FindPages(pages, "started")
	if len(results) != 2 {
		t.Errorf("expected 2 results for 'started', got %d", len(results))
	}
}

func TestFindPages_NoMatch(t *testing.T) {
	pages := []api.Page{
		{ID: "1", Title: "Alpha"},
	}

	results := FindPages(pages, "xyz")
	if len(results) != 0 {
		t.Errorf("expected 0 results, got %d", len(results))
	}
}

func TestPagePath(t *testing.T) {
	pages := []api.Page{
		{ID: "1", Title: "Home", ParentID: "ext-space", ParentType: "page"},
		{ID: "2", Title: "Section", ParentID: "1", ParentType: "page"},
		{ID: "3", Title: "Page", ParentID: "2", ParentType: "page"},
	}

	path := PagePath(pages, "3")
	expected := "/Home/Section/Page"
	if path != expected {
		t.Errorf("expected %q, got %q", expected, path)
	}
}

func TestPagePath_RootPage(t *testing.T) {
	pages := []api.Page{
		{ID: "1", Title: "Home", ParentID: "ext-space", ParentType: "page"},
	}

	path := PagePath(pages, "1")
	if path != "/Home" {
		t.Errorf("expected /Home, got %q", path)
	}
}

func TestCountNodes(t *testing.T) {
	pages := []api.Page{
		{ID: "1", Title: "Home", ParentID: "ext-space", ParentType: "page"},
		{ID: "2", Title: "Child A", ParentID: "1", ParentType: "page"},
		{ID: "3", Title: "Child B", ParentID: "1", ParentType: "page"},
		{ID: "4", Title: "Grandchild", ParentID: "2", ParentType: "page"},
	}

	roots := BuildTree(pages)
	count := CountNodes(roots)
	if count != 4 {
		t.Errorf("expected 4 nodes, got %d", count)
	}
}

func TestCountNodes_Empty(t *testing.T) {
	count := CountNodes(nil)
	if count != 0 {
		t.Errorf("expected 0 for nil roots, got %d", count)
	}
}

func TestBuildChildrenMap(t *testing.T) {
	pages := []api.Page{
		{ID: "1", Title: "Home", ParentID: "ext-space", ParentType: "page"},
		{ID: "2", Title: "Child A", ParentID: "1", ParentType: "page", Position: 0},
		{ID: "3", Title: "Child B", ParentID: "1", ParentType: "page", Position: 1},
		{ID: "4", Title: "Grandchild", ParentID: "2", ParentType: "page", Position: 0},
	}

	children := buildChildrenMap(pages)

	// Page 1 should have 2 children.
	if len(children["1"]) != 2 {
		t.Errorf("expected 2 children for page 1, got %d", len(children["1"]))
	}

	// Page 2 should have 1 child.
	if len(children["2"]) != 1 {
		t.Errorf("expected 1 child for page 2, got %d", len(children["2"]))
	}

	// Page 3 (leaf) should have key present but nil value.
	if _, ok := children["3"]; !ok {
		t.Error("expected key '3' to exist in children map")
	}
	if children["3"] != nil {
		t.Errorf("expected nil children for page 3, got %v", children["3"])
	}
}

func TestDeriveChildren(t *testing.T) {
	pages := []api.Page{
		{ID: "1", Title: "Home", ParentID: "ext-space", ParentType: "page"},
		{ID: "2", Title: "Child A", ParentID: "1", ParentType: "page"},
		{ID: "3", Title: "Child B", ParentID: "1", ParentType: "page"},
	}

	children := deriveChildren(pages, "1")
	if len(children) != 2 {
		t.Errorf("expected 2 children for page 1, got %d", len(children))
	}

	noChildren := deriveChildren(pages, "2")
	if len(noChildren) != 0 {
		t.Errorf("expected 0 children for page 2, got %d", len(noChildren))
	}
}
