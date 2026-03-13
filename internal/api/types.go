package api

// Space represents a Confluence space.
type Space struct {
	ID         string     `json:"id"`
	Key        string     `json:"key"`
	Name       string     `json:"name"`
	Type       string     `json:"type"`
	Status     string     `json:"status"`
	HomepageID string     `json:"homepageId"`
	Links      SpaceLinks `json:"_links"`
}

// SpaceLinks holds link data for a space.
type SpaceLinks struct {
	WebUI string `json:"webui"`
	Base  string `json:"base"`
}

// Page represents a Confluence page.
type Page struct {
	ID         string    `json:"id"`
	Status     string    `json:"status"`
	Title      string    `json:"title"`
	SpaceID    string    `json:"spaceId"`
	ParentID   string    `json:"parentId"`
	ParentType string    `json:"parentType"`
	Position   int       `json:"position"`
	AuthorID   string    `json:"authorId"`
	CreatedAt  string    `json:"createdAt"`
	Version    Version   `json:"version"`
	Body       *PageBody `json:"body,omitempty"`
	Links      PageLinks `json:"_links"`
}

// PageBody holds page body content in different representations.
type PageBody struct {
	Storage        *BodyContent `json:"storage,omitempty"`
	AtlasDocFormat *BodyContent `json:"atlas_doc_format,omitempty"`
	View           *BodyContent `json:"view,omitempty"`
}

// BodyContent holds a single body representation.
type BodyContent struct {
	Representation string `json:"representation"`
	Value          string `json:"value"`
}

// Version holds version metadata.
type Version struct {
	CreatedAt string `json:"createdAt"`
	Message   string `json:"message"`
	Number    int    `json:"number"`
	MinorEdit bool   `json:"minorEdit"`
	AuthorID  string `json:"authorId"`
}

// PageLinks holds link data for a page.
type PageLinks struct {
	WebUI  string `json:"webui"`
	EditUI string `json:"editui"`
	TinyUI string `json:"tinyui"`
}

// Attachment represents a Confluence attachment.
type Attachment struct {
	ID                   string      `json:"id"`
	Status               string      `json:"status"`
	Title                string      `json:"title"`
	CreatedAt            string      `json:"createdAt"`
	PageID               string      `json:"pageId"`
	BlogPostID           string      `json:"blogPostId"`
	MediaType            string      `json:"mediaType"`
	MediaTypeDescription string      `json:"mediaTypeDescription"`
	Comment              string      `json:"comment"`
	FileID               string      `json:"fileId"`
	FileSize             int64       `json:"fileSize"`
	WebuiLink            string      `json:"webuiLink"`
	DownloadLink         string      `json:"downloadLink"`
	Version              Version     `json:"version"`
	Links                AttachLinks `json:"_links"`
}

// AttachLinks holds link data for an attachment.
type AttachLinks struct {
	WebUI    string `json:"webui"`
	Download string `json:"download"`
}

// ChildPage represents a child page in the tree.
type ChildPage struct {
	ID            string `json:"id"`
	Status        string `json:"status"`
	Title         string `json:"title"`
	SpaceID       string `json:"spaceId"`
	ChildPosition int    `json:"childPosition"`
	Type          string `json:"type,omitempty"`
}

// PaginatedResponse is the generic pagination wrapper from the API.
type PaginatedResponse[T any] struct {
	Results []T `json:"results"`
	Links   struct {
		Next string `json:"next"`
		Base string `json:"base"`
	} `json:"_links"`
}
