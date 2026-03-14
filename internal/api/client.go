package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

// Client communicates with the Confluence Cloud REST API v2.
type Client struct {
	BaseURL    string // e.g. "https://your-domain.atlassian.net"
	Email      string
	APIToken   string
	Verbose    bool
	HTTPClient *http.Client
}

// NewClient creates a new Confluence API client using Basic Auth credentials.
func NewClient(baseURL, email, apiToken string) *Client {
	return &Client{
		BaseURL:  baseURL,
		Email:    email,
		APIToken: apiToken,
		HTTPClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// logf prints a message to stderr when verbose mode is enabled.
func (c *Client) logf(format string, args ...any) {
	if c.Verbose {
		fmt.Fprintf(os.Stderr, "[debug] "+format+"\n", args...)
	}
}

// do executes an authenticated request and returns the response body.
func (c *Client) do(method, path string, query url.Values) ([]byte, error) {
	u, err := url.Parse(c.BaseURL + path)
	if err != nil {
		return nil, fmt.Errorf("parse url: %w", err)
	}
	if query != nil {
		u.RawQuery = query.Encode()
	}

	c.logf("%s %s", method, u.String())

	req, err := http.NewRequest(method, u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.SetBasicAuth(c.Email, c.APIToken)
	req.Header.Set("Accept", "application/json")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		c.logf("request failed: %v", err)
		return nil, fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	c.logf("response: %d %s", resp.StatusCode, resp.Status)

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		msg := strings.TrimSpace(string(body))
		if strings.HasPrefix(msg, "<") || strings.Contains(msg, "<!DOCTYPE") {
			msg = http.StatusText(resp.StatusCode)
			if msg == "" {
				msg = "unknown error"
			}
		} else if len(msg) > 200 {
			msg = msg[:200] + "..."
		}
		return nil, fmt.Errorf("api error (status %d): %s", resp.StatusCode, msg)
	}

	c.logf("body: %d bytes", len(body))

	return body, nil
}

// doRaw executes an authenticated request and returns the raw response.
// The caller is responsible for closing the response body.
func (c *Client) doRaw(method, path string, query url.Values) (*http.Response, error) {
	u, err := url.Parse(c.BaseURL + path)
	if err != nil {
		return nil, fmt.Errorf("parse url: %w", err)
	}
	if query != nil {
		u.RawQuery = query.Encode()
	}

	c.logf("%s %s", method, u.String())

	req, err := http.NewRequest(method, u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.SetBasicAuth(c.Email, c.APIToken)

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		c.logf("request failed: %v", err)
		return nil, fmt.Errorf("http request: %w", err)
	}

	c.logf("response: %d %s", resp.StatusCode, resp.Status)

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		msg := strings.TrimSpace(string(body))
		// If the response is HTML (e.g. a JIRA/Confluence 404 page), replace
		// the verbose markup with a short status description.
		if strings.HasPrefix(msg, "<") || strings.Contains(msg, "<!DOCTYPE") {
			msg = http.StatusText(resp.StatusCode)
			if msg == "" {
				msg = "unknown error"
			}
		} else if len(msg) > 200 {
			msg = msg[:200] + "..."
		}
		return nil, fmt.Errorf("api error (status %d): %s", resp.StatusCode, msg)
	}

	return resp, nil
}

// paginate fetches all pages of results for a given endpoint.
func paginate[T any](c *Client, path string, query url.Values) ([]T, error) {
	var all []T
	currentPath := path

	for {
		data, err := c.do("GET", currentPath, query)
		if err != nil {
			return nil, err
		}

		var page PaginatedResponse[T]
		if err := json.Unmarshal(data, &page); err != nil {
			return nil, fmt.Errorf("decode response: %w", err)
		}

		all = append(all, page.Results...)

		if page.Links.Next == "" {
			break
		}

		// The next link is a relative URL like /wiki/api/v2/...?cursor=...
		// Parse it to extract path and query separately.
		nextURL, err := url.Parse(page.Links.Next)
		if err != nil {
			return nil, fmt.Errorf("parse next link: %w", err)
		}
		currentPath = nextURL.Path
		query = nextURL.Query()
	}

	return all, nil
}

// GetSpaces returns all spaces the authenticated user can see.
func (c *Client) GetSpaces() ([]Space, error) {
	query := url.Values{}
	query.Set("limit", "250")
	return paginate[Space](c, "/wiki/api/v2/spaces", query)
}

// GetPagesInSpace returns all pages in a space.
func (c *Client) GetPagesInSpace(spaceID string) ([]Page, error) {
	query := url.Values{}
	query.Set("limit", "250")
	path := fmt.Sprintf("/wiki/api/v2/spaces/%s/pages", spaceID)
	return paginate[Page](c, path, query)
}

// GetPageByID returns a single page with its body in storage format.
func (c *Client) GetPageByID(pageID string) (*Page, error) {
	query := url.Values{}
	query.Set("body-format", "storage")
	data, err := c.do("GET", fmt.Sprintf("/wiki/api/v2/pages/%s", pageID), query)
	if err != nil {
		return nil, err
	}

	var page Page
	if err := json.Unmarshal(data, &page); err != nil {
		return nil, fmt.Errorf("decode page: %w", err)
	}
	return &page, nil
}

// GetChildPages returns the direct child pages of a page.
func (c *Client) GetChildPages(pageID string) ([]ChildPage, error) {
	query := url.Values{}
	query.Set("limit", "250")
	path := fmt.Sprintf("/wiki/api/v2/pages/%s/children", pageID)
	return paginate[ChildPage](c, path, query)
}

// GetAttachmentsForPage returns all attachments on a page.
func (c *Client) GetAttachmentsForPage(pageID string) ([]Attachment, error) {
	query := url.Values{}
	query.Set("limit", "250")
	path := fmt.Sprintf("/wiki/api/v2/pages/%s/attachments", pageID)
	return paginate[Attachment](c, path, query)
}

// DownloadAttachment downloads an attachment and returns the response.
// The caller must close the response body.
func (c *Client) DownloadAttachment(downloadPath string) (*http.Response, error) {
	// The download link may be an absolute URL or a relative path.
	// Strip the base URL prefix so doRaw can reconstruct it cleanly.
	downloadPath = strings.TrimPrefix(downloadPath, c.BaseURL)

	// Confluence Cloud serves content under /wiki; ensure the path
	// includes this prefix so the request is routed correctly.
	if !strings.HasPrefix(downloadPath, "/wiki/") {
		downloadPath = "/wiki" + downloadPath
	}
	return c.doRaw("GET", downloadPath, nil)
}

// GetSpaceOperations returns the permitted operations for the current user on a space.
func (c *Client) GetSpaceOperations(spaceID string) ([]Operation, error) {
	path := fmt.Sprintf("/wiki/api/v2/spaces/%s/operations", spaceID)
	data, err := c.do("GET", path, nil)
	if err != nil {
		return nil, fmt.Errorf("fetch space operations: %w", err)
	}

	var resp PermittedOperationsResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("decode operations: %w", err)
	}
	return resp.Operations, nil
}
