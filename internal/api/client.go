package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	defaultMaxRetries = 3
	defaultMaxBackoff = 30 * time.Second
	maxRetryAfter     = 60 * time.Second
)

// AuthError is returned for 401/403 responses. It carries the HTTP status and
// request URL so callers can give the user actionable guidance.
type AuthError struct {
	Status int
	URL    string
}

func (e *AuthError) Error() string {
	return fmt.Sprintf("authentication failed (HTTP %d) — run 'confluence-reader configure' to update credentials", e.Status)
}

// APIError is returned for non-auth HTTP errors that are either non-retryable
// (e.g. 404) or have exhausted the retry budget (5xx, 429).
type APIError struct {
	Status int
	Body   string // truncated, HTML replaced with a plain status description
	URL    string
}

func (e *APIError) Error() string {
	if e.Body == "" {
		return fmt.Sprintf("api error (status %d) from %s", e.Status, e.URL)
	}
	return fmt.Sprintf("api error (status %d): %s", e.Status, e.Body)
}

// Client communicates with the Confluence Cloud REST API v2.
type Client struct {
	BaseURL    string // e.g. "https://your-domain.atlassian.net"
	Email      string
	APIToken   string
	Verbose    bool
	HTTPClient *http.Client

	// MaxRetries is the number of retries after the first attempt. The total
	// number of HTTP calls for one logical request is MaxRetries+1.
	MaxRetries int
	// MaxBackoff caps the exponential backoff delay between retries.
	MaxBackoff time.Duration

	// sleep is the delay function, overridable in tests.
	sleep func(time.Duration)
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
		MaxRetries: defaultMaxRetries,
		MaxBackoff: defaultMaxBackoff,
		sleep:      time.Sleep,
	}
}

// logf prints a message to stderr when verbose mode is enabled.
func (c *Client) logf(format string, args ...any) {
	if c.Verbose {
		fmt.Fprintf(os.Stderr, "[debug] "+format+"\n", args...)
	}
}

func (c *Client) maxRetries() int {
	if c.MaxRetries <= 0 {
		return defaultMaxRetries
	}
	return c.MaxRetries
}

func (c *Client) maxBackoff() time.Duration {
	if c.MaxBackoff <= 0 {
		return defaultMaxBackoff
	}
	return c.MaxBackoff
}

func (c *Client) sleepFor(d time.Duration) {
	if c.sleep != nil {
		c.sleep(d)
		return
	}
	time.Sleep(d)
}

// send issues a single HTTP request. The caller must close resp.Body.
func (c *Client) send(method, path string, query url.Values) (*http.Response, *url.URL, error) {
	u, err := url.Parse(c.BaseURL + path)
	if err != nil {
		return nil, nil, fmt.Errorf("parse url: %w", err)
	}
	if query != nil {
		u.RawQuery = query.Encode()
	}

	c.logf("%s %s", method, u.String())

	req, err := http.NewRequest(method, u.String(), nil)
	if err != nil {
		return nil, u, fmt.Errorf("create request: %w", err)
	}
	req.SetBasicAuth(c.Email, c.APIToken)
	req.Header.Set("Accept", "application/json")

	resp, err := c.HTTPClient.Do(req)
	return resp, u, err
}

// doWithRetry executes a request with retry + rate-limit handling and returns
// the final successful response. The caller must close resp.Body.
//
// Retryable conditions (up to MaxRetries extra attempts):
//   - network errors (timeouts, connection resets, etc.)
//   - HTTP 429, honoring the Retry-After header (capped at 60s)
//   - HTTP 5xx, with exponential backoff
//
// Non-retryable conditions:
//   - HTTP 401/403 → *AuthError
//   - other non-2xx → *APIError
func (c *Client) doWithRetry(method, path string, query url.Values) (*http.Response, error) {
	maxAttempts := c.maxRetries() + 1
	var lastErr error

	for attempt := 0; attempt < maxAttempts; attempt++ {
		resp, u, err := c.send(method, path, query)
		if err != nil {
			lastErr = fmt.Errorf("http request: %w", err)
			if attempt < maxAttempts-1 {
				wait := c.backoff(attempt)
				c.logf("connection error, retrying in %s: %v", wait, err)
				c.sleepFor(wait)
				continue
			}
			return nil, lastErr
		}

		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			c.logf("response: %d %s", resp.StatusCode, resp.Status)
			return resp, nil
		}

		// 401/403 — no retry, caller needs to update credentials.
		if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
			drainAndClose(resp)
			return nil, &AuthError{Status: resp.StatusCode, URL: u.String()}
		}

		// 429 — rate limited. Honor Retry-After (cap at 60s).
		if resp.StatusCode == http.StatusTooManyRequests && attempt < maxAttempts-1 {
			wait := parseRetryAfter(resp.Header.Get("Retry-After"), c.backoff(attempt))
			if wait > maxRetryAfter {
				wait = maxRetryAfter
			}
			drainAndClose(resp)
			c.logf("rate limited, retrying in %s", wait)
			c.sleepFor(wait)
			continue
		}

		// 5xx — retry with exponential backoff.
		if resp.StatusCode >= 500 && attempt < maxAttempts-1 {
			wait := c.backoff(attempt)
			c.logf("server error %d, retrying in %s", resp.StatusCode, wait)
			drainAndClose(resp)
			c.sleepFor(wait)
			continue
		}

		return nil, buildAPIError(resp, u.String())
	}

	if lastErr == nil {
		lastErr = fmt.Errorf("max retries exceeded")
	}
	return nil, lastErr
}

// do executes a request and returns the response body.
func (c *Client) do(method, path string, query url.Values) ([]byte, error) {
	resp, err := c.doWithRetry(method, path, query)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}

	c.logf("body: %d bytes", len(body))
	return body, nil
}

// doRaw executes a request and returns the raw response for streaming.
// The caller is responsible for closing the response body.
func (c *Client) doRaw(method, path string, query url.Values) (*http.Response, error) {
	return c.doWithRetry(method, path, query)
}

// backoff returns the wait duration for the given zero-indexed retry attempt.
// Produces 1s, 2s, 4s, 8s, ... capped at MaxBackoff.
func (c *Client) backoff(attempt int) time.Duration {
	wait := time.Duration(1<<attempt) * time.Second
	if max := c.maxBackoff(); wait > max {
		wait = max
	}
	return wait
}

// parseRetryAfter decodes a Retry-After header value (seconds or HTTP-date).
// Returns fallback if the value is missing or unparseable.
func parseRetryAfter(val string, fallback time.Duration) time.Duration {
	val = strings.TrimSpace(val)
	if val == "" {
		return fallback
	}
	if secs, err := strconv.Atoi(val); err == nil && secs >= 0 {
		return time.Duration(secs) * time.Second
	}
	if t, err := http.ParseTime(val); err == nil {
		d := time.Until(t)
		if d < 0 {
			return fallback
		}
		return d
	}
	return fallback
}

// buildAPIError builds an *APIError from a non-2xx response, truncating HTML
// bodies to a short status description so error output stays readable.
func buildAPIError(resp *http.Response, u string) error {
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	msg := strings.TrimSpace(string(body))
	if strings.HasPrefix(msg, "<") || strings.Contains(msg, "<!DOCTYPE") {
		msg = http.StatusText(resp.StatusCode)
		if msg == "" {
			msg = "unknown error"
		}
	} else if len(msg) > 200 {
		msg = msg[:200] + "..."
	}
	return &APIError{Status: resp.StatusCode, Body: msg, URL: u}
}

// drainAndClose discards the response body so the connection can be reused,
// then closes it. Used before retrying.
func drainAndClose(resp *http.Response) {
	if resp == nil {
		return
	}
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
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
