package player

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
)

// DefaultMediaCacheLimit bounds temporary media storage to 2 GiB.
const DefaultMediaCacheLimit int64 = 2 << 30

// ErrMediaCacheFull reports that a download would exceed the session limit.
var ErrMediaCacheFull = errors.New("media cache size limit reached")

// MediaDownloadFunc writes one media item and returns its byte count.
type MediaDownloadFunc func(context.Context, io.Writer) (int64, error)

// SessionCache stores media files for the lifetime of one app process.
// Downloads are serialized so the configured size limit cannot be exceeded by
// concurrent cache requests.
type SessionCache struct {
	dir      string
	maxBytes int64

	downloadMu sync.Mutex
	mu         sync.RWMutex
	paths      map[string]string
	usedBytes  int64
	closed     bool
}

// NewSessionCache creates a private cache directory that mpv can access.
func NewSessionCache(maxBytes int64) (*SessionCache, error) {
	root, err := MpvSocketDir()
	if err != nil {
		return nil, fmt.Errorf("resolve media cache directory: %w", err)
	}
	return newSessionCacheAt(root, maxBytes)
}

func newSessionCacheAt(root string, maxBytes int64) (*SessionCache, error) {
	if maxBytes <= 0 {
		return nil, errors.New("media cache limit must be positive")
	}
	dir, err := os.MkdirTemp(root, fmt.Sprintf("%s-media-%d-", appName, os.Getpid()))
	if err != nil {
		return nil, fmt.Errorf("create media cache directory: %w", err)
	}
	if err := os.Chmod(dir, 0700); err != nil {
		_ = os.RemoveAll(dir)
		return nil, fmt.Errorf("secure media cache directory: %w", err)
	}
	return &SessionCache{
		dir:      dir,
		maxBytes: maxBytes,
		paths:    make(map[string]string),
	}, nil
}

// Path returns a complete cached file for itemID.
func (c *SessionCache) Path(itemID string) (string, bool) {
	if c == nil {
		return "", false
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	path, ok := c.paths[itemID]
	return path, ok
}

// Store downloads itemID once and returns its local path.
func (c *SessionCache) Store(ctx context.Context, itemID string, download MediaDownloadFunc) (string, error) {
	if c == nil {
		return "", errors.New("media cache is unavailable")
	}
	if ctx == nil {
		return "", errors.New("media cache context is nil")
	}
	if itemID == "" {
		return "", errors.New("media cache item ID is empty")
	}
	if download == nil {
		return "", errors.New("media download function is nil")
	}
	if path, ok := c.Path(itemID); ok {
		return path, nil
	}

	c.downloadMu.Lock()
	defer c.downloadMu.Unlock()

	if path, ok := c.Path(itemID); ok {
		return path, nil
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}

	c.mu.RLock()
	closed := c.closed
	remaining := c.maxBytes - c.usedBytes
	dir := c.dir
	c.mu.RUnlock()
	if closed {
		return "", errors.New("media cache is closed")
	}
	if remaining <= 0 {
		return "", ErrMediaCacheFull
	}

	digest := sha256.Sum256([]byte(itemID))
	name := fmt.Sprintf("%x.media", digest[:16])
	part, err := os.CreateTemp(dir, name+".part-")
	if err != nil {
		return "", fmt.Errorf("create partial media file: %w", err)
	}
	partPath := part.Name()
	complete := false
	defer func() {
		_ = part.Close()
		if !complete {
			_ = os.Remove(partPath)
		}
	}()
	if err := os.Chmod(partPath, 0600); err != nil {
		return "", fmt.Errorf("secure partial media file: %w", err)
	}

	limited := &cacheLimitWriter{writer: part, remaining: remaining}
	written, err := download(ctx, limited)
	if err != nil {
		return "", fmt.Errorf("download media: %w", err)
	}
	if written != limited.written {
		return "", fmt.Errorf("download byte count mismatch: reported %d, wrote %d", written, limited.written)
	}
	if written == 0 {
		return "", errors.New("downloaded media file is empty")
	}
	if err := part.Sync(); err != nil {
		return "", fmt.Errorf("sync media cache file: %w", err)
	}
	if err := part.Close(); err != nil {
		return "", fmt.Errorf("close media cache file: %w", err)
	}

	path := filepath.Join(dir, name)
	if err := os.Rename(partPath, path); err != nil {
		return "", fmt.Errorf("commit media cache file: %w", err)
	}
	complete = true

	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		_ = os.Remove(path)
		return "", errors.New("media cache is closed")
	}
	c.paths[itemID] = path
	c.usedBytes += limited.written
	c.mu.Unlock()
	return path, nil
}

// Close removes every cached file.
func (c *SessionCache) Close() error {
	if c == nil {
		return nil
	}
	c.downloadMu.Lock()
	defer c.downloadMu.Unlock()
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return nil
	}
	c.closed = true
	dir := c.dir
	c.paths = nil
	c.usedBytes = 0
	c.mu.Unlock()
	if err := os.RemoveAll(dir); err != nil {
		return fmt.Errorf("remove media cache: %w", err)
	}
	return nil
}

type cacheLimitWriter struct {
	writer    io.Writer
	remaining int64
	written   int64
}

func (w *cacheLimitWriter) Write(p []byte) (int, error) {
	if int64(len(p)) > w.remaining {
		if w.remaining <= 0 {
			return 0, ErrMediaCacheFull
		}
		n, err := w.writer.Write(p[:w.remaining])
		w.remaining -= int64(n)
		w.written += int64(n)
		if err != nil {
			return n, err
		}
		return n, ErrMediaCacheFull
	}
	n, err := w.writer.Write(p)
	w.remaining -= int64(n)
	w.written += int64(n)
	return n, err
}
