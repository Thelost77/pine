//go:build integration

package abs

import (
	"bufio"
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// loadEnv reads a .env file and returns a map of key=value pairs.
func loadEnv(t *testing.T) map[string]string {
	t.Helper()

	// Walk up from this test file to find .env in project root.
	_, thisFile, _, _ := runtime.Caller(0)
	root := filepath.Join(filepath.Dir(thisFile), "..", "..")
	envPath := filepath.Join(root, ".env")

	f, err := os.Open(envPath)
	if err != nil {
		t.Fatalf("cannot open .env at %s: %v", envPath, err)
	}
	defer f.Close()

	env := make(map[string]string)
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		env[strings.TrimSpace(k)] = strings.TrimSpace(v)
	}
	return env
}

func setupIntegrationClient(t *testing.T) (*Client, context.Context) {
	t.Helper()
	env := loadEnv(t)

	url := env["ABS_URL"]
	user := env["ABS_USER"]
	pass := env["ABS_PASS"]

	if url == "" || user == "" || pass == "" {
		t.Fatal("ABS_URL, ABS_USER, and ABS_PASS must be set in .env")
	}

	client := NewClient(url, "")
	ctx := context.Background()

	token, err := client.Login(ctx, user, pass)
	if err != nil {
		t.Fatalf("Login failed: %v", err)
	}
	if token == "" {
		t.Fatal("Login returned empty token")
	}

	return client, ctx
}

func TestIntegration_Login(t *testing.T) {
	env := loadEnv(t)
	client := NewClient(env["ABS_URL"], "")
	ctx := context.Background()

	token, err := client.Login(ctx, env["ABS_USER"], env["ABS_PASS"])
	if err != nil {
		t.Fatalf("Login failed: %v", err)
	}
	if token == "" {
		t.Fatal("expected non-empty token")
	}
	t.Logf("Login succeeded, token length: %d", len(token))
}

func TestIntegration_GetLibraries(t *testing.T) {
	client, ctx := setupIntegrationClient(t)

	libs, err := client.GetLibraries(ctx)
	if err != nil {
		t.Fatalf("GetLibraries failed: %v", err)
	}
	if len(libs) == 0 {
		t.Fatal("expected at least one library")
	}
	for _, lib := range libs {
		t.Logf("Library: id=%s name=%q type=%s", lib.ID, lib.Name, lib.MediaType)
	}
}

func TestIntegration_GetPersonalized(t *testing.T) {
	client, ctx := setupIntegrationClient(t)

	libs, err := client.GetLibraries(ctx)
	if err != nil {
		t.Fatalf("GetLibraries failed: %v", err)
	}
	if len(libs) == 0 {
		t.Fatal("no libraries to test personalized endpoint")
	}

	shelves, err := client.GetPersonalized(ctx, libs[0].ID)
	if err != nil {
		t.Fatalf("GetPersonalized failed: %v", err)
	}
	// Personalized may return empty shelves on a fresh server, so just check no error.
	t.Logf("GetPersonalized returned %d shelves for library %q", len(shelves), libs[0].Name)
	for _, shelf := range shelves {
		t.Logf("  Shelf: id=%s entities=%d", shelf.ID, len(shelf.Entities))
	}
}

func TestIntegration_GetLibraryItems(t *testing.T) {
	client, ctx := setupIntegrationClient(t)

	libs, err := client.GetLibraries(ctx)
	if err != nil {
		t.Fatalf("GetLibraries failed: %v", err)
	}
	if len(libs) == 0 {
		t.Fatal("no libraries to test items endpoint")
	}

	// First page
	resp, err := client.GetLibraryItems(ctx, libs[0].ID, 0, 2)
	if err != nil {
		t.Fatalf("GetLibraryItems page 0 failed: %v", err)
	}
	t.Logf("Page 0: got %d items, total=%d", len(resp.Results), resp.Total)
	if resp.Total == 0 {
		t.Skip("library has no items, skipping pagination test")
	}
	if len(resp.Results) == 0 {
		t.Fatal("expected results on page 0")
	}

	// Second page (if enough items)
	if resp.Total > 2 {
		resp2, err := client.GetLibraryItems(ctx, libs[0].ID, 1, 2)
		if err != nil {
			t.Fatalf("GetLibraryItems page 1 failed: %v", err)
		}
		t.Logf("Page 1: got %d items", len(resp2.Results))
		if len(resp2.Results) == 0 {
			t.Fatal("expected results on page 1")
		}
		// Ensure different items
		if resp2.Results[0].ID == resp.Results[0].ID {
			t.Error("page 1 returned same first item as page 0")
		}
	}
}
