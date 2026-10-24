package abs

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync/atomic"
	"time"

	"github.com/Thelost77/pine/internal/logger"
)

// HTTPStatusError wraps a non-2xx HTTP response status.
type HTTPStatusError struct {
	StatusCode int
	// Body is the start of the response body, at most 500 bytes.
	Body string
}

func (e *HTTPStatusError) Error() string {
	return fmt.Sprintf("unexpected status %d: %s", e.StatusCode, e.Body)
}

// IsHTTPStatus reports whether err or one of its wrapped errors is an HTTPStatusError
// for the given status code.
func IsHTTPStatus(err error, statusCode int) bool {
	var statusErr *HTTPStatusError
	return errors.As(err, &statusErr) && statusErr.StatusCode == statusCode
}

const maxResponseSize = 50 * 1024 * 1024 // 50 MB
// Client is an HTTP client for the Audiobookshelf API.
type Client struct {
	baseURL    string
	token      string
	httpClient *http.Client
	// noBookmarkRoute is true after the server answered 404 for the bookmarks
	// of one item. See GetBookmarks.
	noBookmarkRoute atomic.Bool
}

// NewClient creates a new ABS API client. It removes the spaces around
// baseURL and the slashes at its end, so that a request path does not start
// with "//".
func NewClient(baseURL, token string) *Client {
	return &Client{
		baseURL: strings.TrimRight(strings.TrimSpace(baseURL), "/"),
		token:   token,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// do executes an authenticated HTTP request and returns the response body.
func (c *Client) do(ctx context.Context, method, path string, body any) ([]byte, error) {
	start := time.Now()
	var bodyReader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("marshal request body: %w", err)
		}
		bodyReader = bytes.NewReader(b)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}

	shownPath := logPath(path)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		var urlErr *url.Error
		if errors.As(err, &urlErr) {
			urlErr.URL = strings.Replace(urlErr.URL, path, shownPath, 1)
		}
		logger.Error("http request failed", "method", method, "path", shownPath, "err", err, "duration", time.Since(start))
		return nil, fmt.Errorf("execute request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	limitedReader := io.LimitReader(resp.Body, maxResponseSize+1)
	data, err := io.ReadAll(limitedReader)
	if err != nil {
		logger.Error("http response read failed", "method", method, "path", shownPath, "status", resp.StatusCode, "err", err, "duration", time.Since(start))
		return nil, fmt.Errorf("read response body: %w", err)
	}
	if len(data) > maxResponseSize {
		logger.Error("http response body too large", "method", method, "path", shownPath, "status", resp.StatusCode, "limit", maxResponseSize, "duration", time.Since(start))
		return nil, fmt.Errorf("response body exceeds %d byte limit", maxResponseSize)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body := truncateLogBody(data)
		logger.Error("http request returned unexpected status", "method", method, "path", shownPath, "status", resp.StatusCode, "body", body, "duration", time.Since(start))
		return nil, &HTTPStatusError{StatusCode: resp.StatusCode, Body: body}
	}

	logger.Debug("http request completed", "method", method, "path", shownPath, "status", resp.StatusCode, "duration", time.Since(start))
	return data, nil
}

// Login authenticates with the ABS server and stores the token.
func (c *Client) Login(ctx context.Context, username, password string) (string, error) {
	creds := struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}{
		Username: username,
		Password: password,
	}

	data, err := c.do(ctx, http.MethodPost, "/login", creds)
	if err != nil {
		return "", fmt.Errorf("login: %w", err)
	}

	var resp LoginResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return "", fmt.Errorf("decode login response: %w", err)
	}

	c.token = resp.User.Token
	return c.token, nil
}

// BaseURL returns the server base URL.
func (c *Client) BaseURL() string {
	return c.baseURL
}

// Token returns the auth token.
func (c *Client) Token() string {
	return c.token
}

// SessionTag returns a short name for the ID of a play session, for a log
// line. The ID gives the audio of the session with no login, so a log line
// must not contain it. The same ID gives the same name.
func SessionTag(id string) string {
	sum := sha256.Sum256([]byte(id))
	return "#" + hex.EncodeToString(sum[:3])
}

// sessionPathPrefix starts the path of each request about one play session.
const sessionPathPrefix = "/api/session/"

// logPath returns path with the ID of a play session replaced by its tag.
func logPath(path string) string {
	rest, ok := strings.CutPrefix(path, sessionPathPrefix)
	if !ok {
		return path
	}
	end := strings.IndexByte(rest, '/')
	if end < 0 {
		end = len(rest)
	}
	return sessionPathPrefix + SessionTag(rest[:end]) + rest[end:]
}

func truncateLogBody(data []byte) string {
	const max = 500
	if len(data) <= max {
		return string(data)
	}
	return string(data[:max]) + "..."
}
