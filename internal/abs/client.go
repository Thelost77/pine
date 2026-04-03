package abs

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/Thelost77/pine/internal/logger"
)

// HTTPStatusError wraps a non-2xx HTTP response status.
type HTTPStatusError struct {
	StatusCode int
	Body       string
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

// Client is an HTTP client for the Audiobookshelf API.
type Client struct {
	baseURL    string
	token      string
	httpClient *http.Client
}

// NewClient creates a new ABS API client.
func NewClient(baseURL, token string) *Client {
	return &Client{
		baseURL: baseURL,
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

	resp, err := c.httpClient.Do(req)
	if err != nil {
		logger.Error("http request failed", "method", method, "path", path, "err", err, "duration", time.Since(start))
		return nil, fmt.Errorf("execute request: %w", err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		logger.Error("http response read failed", "method", method, "path", path, "status", resp.StatusCode, "err", err, "duration", time.Since(start))
		return nil, fmt.Errorf("read response body: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		logger.Error("http request returned unexpected status", "method", method, "path", path, "status", resp.StatusCode, "body", truncateLogBody(data), "duration", time.Since(start))
		return nil, &HTTPStatusError{StatusCode: resp.StatusCode, Body: string(data)}
	}

	logger.Debug("http request completed", "method", method, "path", path, "status", resp.StatusCode, "duration", time.Since(start))
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

func truncateLogBody(data []byte) string {
	const max = 500
	if len(data) <= max {
		return string(data)
	}
	return string(data[:max]) + "..."
}
