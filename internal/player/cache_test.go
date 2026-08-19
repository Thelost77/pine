package player

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
)

func TestSessionCacheStoresOnceAndRemovesFilesOnClose(t *testing.T) {
	cache, err := newSessionCacheAt(t.TempDir(), 1024)
	if err != nil {
		t.Fatalf("newSessionCacheAt() error: %v", err)
	}
	dir := cache.dir
	var calls atomic.Int32
	download := func(_ context.Context, dst io.Writer) (int64, error) {
		calls.Add(1)
		n, err := io.Copy(dst, bytes.NewBufferString("audio data"))
		return n, err
	}

	first, err := cache.Store(context.Background(), "track-1", download)
	if err != nil {
		t.Fatalf("first Store() error: %v", err)
	}
	second, err := cache.Store(context.Background(), "track-1", download)
	if err != nil {
		t.Fatalf("second Store() error: %v", err)
	}
	if first != second || calls.Load() != 1 {
		t.Fatalf("paths = %q, %q; download calls = %d", first, second, calls.Load())
	}
	info, err := os.Stat(first)
	if err != nil {
		t.Fatalf("stat cached file: %v", err)
	}
	if info.Mode().Perm() != 0600 {
		t.Fatalf("cached file mode = %o, want 600", info.Mode().Perm())
	}
	data, err := os.ReadFile(first)
	if err != nil || string(data) != "audio data" {
		t.Fatalf("cached data = %q, err=%v", data, err)
	}

	if err := cache.Close(); err != nil {
		t.Fatalf("Close() error: %v", err)
	}
	if _, err := os.Stat(dir); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("cache directory remains after Close(): %v", err)
	}
}

func TestSessionCacheDeduplicatesConcurrentStores(t *testing.T) {
	cache, err := newSessionCacheAt(t.TempDir(), 1024)
	if err != nil {
		t.Fatalf("newSessionCacheAt() error: %v", err)
	}
	t.Cleanup(func() { _ = cache.Close() })

	var calls atomic.Int32
	download := func(_ context.Context, dst io.Writer) (int64, error) {
		calls.Add(1)
		return io.Copy(dst, bytes.NewBufferString("audio"))
	}
	var wg sync.WaitGroup
	errs := make(chan error, 8)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := cache.Store(context.Background(), "track-1", download)
			errs <- err
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatalf("Store() error: %v", err)
		}
	}
	if calls.Load() != 1 {
		t.Fatalf("download calls = %d, want 1", calls.Load())
	}
}

func TestSessionCacheRejectsOversizedDownloadAndRemovesPartialFile(t *testing.T) {
	root := t.TempDir()
	cache, err := newSessionCacheAt(root, 4)
	if err != nil {
		t.Fatalf("newSessionCacheAt() error: %v", err)
	}
	t.Cleanup(func() { _ = cache.Close() })

	_, err = cache.Store(context.Background(), "track-1", func(_ context.Context, dst io.Writer) (int64, error) {
		return io.Copy(dst, bytes.NewBufferString("too large"))
	})
	if !errors.Is(err, ErrMediaCacheFull) {
		t.Fatalf("Store() error = %v, want ErrMediaCacheFull", err)
	}
	entries, err := os.ReadDir(cache.dir)
	if err != nil {
		t.Fatalf("ReadDir() error: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("partial cache files remain: %v", entries)
	}
	if _, ok := cache.Path("track-1"); ok {
		t.Fatal("oversized item was marked cached")
	}
}

func TestSessionCacheUsesHashedFileName(t *testing.T) {
	cache, err := newSessionCacheAt(t.TempDir(), 1024)
	if err != nil {
		t.Fatalf("newSessionCacheAt() error: %v", err)
	}
	t.Cleanup(func() { _ = cache.Close() })

	path, err := cache.Store(context.Background(), "../../secret", func(_ context.Context, dst io.Writer) (int64, error) {
		return io.Copy(dst, bytes.NewBufferString("x"))
	})
	if err != nil {
		t.Fatalf("Store() error: %v", err)
	}
	if filepath.Dir(path) != cache.dir || filepath.Base(path) == "secret" {
		t.Fatalf("unsafe cache path: %q", path)
	}
}
