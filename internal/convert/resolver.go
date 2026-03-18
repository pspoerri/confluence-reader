package convert

// PageInfo holds basic page metadata for macro resolution.
type PageInfo struct {
	ID    string
	Title string
}

// TreeEntry represents a page at a given depth within a page tree.
type TreeEntry struct {
	Page  PageInfo
	Depth int
}

// MacroResolver provides page data needed to expand children and pagetree macros.
// When nil, macros render as placeholder text instead.
type MacroResolver interface {
	CurrentPageID() string
	ChildPages(pageID string) ([]PageInfo, error)
	PageByTitle(title string) *PageInfo
	SubTree(pageID string) ([]TreeEntry, error)
}
