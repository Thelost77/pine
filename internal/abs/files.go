package abs

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/Thelost77/pine/internal/logger"
)

// GetLibraryFiles returns libraryFiles from an expanded item without storing the item.
func (c *Client) GetLibraryFiles(ctx context.Context, itemID string) ([]LibraryFile, error) {
	path := fmt.Sprintf("/api/items/%s?expanded=1", itemID)
	logger.Debug("API request", "method", "GET", "path", path, "itemID", itemID)
	data, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		logger.Error("get library files failed", "itemID", itemID, "err", err)
		return nil, fmt.Errorf("get library files: %w", err)
	}
	var resp struct {
		LibraryFiles []LibraryFile `json:"libraryFiles"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("decode library files: %w", err)
	}
	return resp.LibraryFiles, nil
}

// GetLibraryFile downloads a library file by inode.
func (c *Client) GetLibraryFile(ctx context.Context, itemID, ino string) ([]byte, error) {
	path := fmt.Sprintf("/api/items/%s/file/%s", itemID, ino)
	logger.Debug("API request", "method", "GET", "path", path, "itemID", itemID, "ino", ino)
	data, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		logger.Error("get library file failed", "itemID", itemID, "ino", ino, "err", err)
		return nil, fmt.Errorf("get library file: %w", err)
	}
	return data, nil
}

// TranscriptIno returns the inode of the VTT sidecar for audioFilename.
// If audioFilename is empty and the item has exactly one VTT, that file is used.
func TranscriptIno(files []LibraryFile, audioFilename string) string {
	var vtts []LibraryFile
	for _, file := range files {
		if isVTT(file) {
			vtts = append(vtts, file)
		}
	}
	if len(vtts) == 0 {
		return ""
	}
	stem := fileStem(audioFilename)
	if stem == "" {
		if len(vtts) == 1 {
			return vtts[0].Ino
		}
		return ""
	}
	for _, file := range vtts {
		if strings.EqualFold(fileStem(file.Metadata.Filename), stem) {
			return file.Ino
		}
	}
	return ""
}

func isVTT(file LibraryFile) bool {
	ext := strings.TrimPrefix(strings.ToLower(file.Metadata.Ext), ".")
	if ext == "vtt" {
		return true
	}
	return strings.EqualFold(filepath.Ext(file.Metadata.Filename), ".vtt")
}

func fileStem(name string) string {
	base := filepath.Base(strings.TrimSpace(name))
	if base == "" || base == "." || base == string(filepath.Separator) {
		return ""
	}
	return strings.TrimSuffix(base, filepath.Ext(base))
}
