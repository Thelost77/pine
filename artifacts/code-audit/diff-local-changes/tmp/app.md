# Cluster 2 Audit Notes: App Integration

## Target Files
- `internal/app/model.go`
- `internal/app/navigation.go`
- `internal/app/render.go`
- `internal/app/keymap.go`
- `internal/app/messages.go`

## Findings

### 1. Unused openContextPalette Function (Nit/Low)
- **File/line:** `internal/app/model.go:986:17`
- **Proof:** `golangci-lint` output: `func (*Model).openContextPalette is unused (unused)`. No call sites exist in the repository.
- **Impact:** Unused code increases codebase maintenance surface.
- **Suggested Fix:** Remove the unused function from `internal/app/model.go`.

### 2. "Show Queue" Action is a Dead UX Action (Medium)
- **File/line:** `internal/app/model.go:1034`, `internal/app/model.go:1151`
- **Proof:** Selecting "Show Queue" in the command palette triggers `ActionShowQueue` which is mapped to returning `m, nil` (does nothing).
- **Impact:** Confuses the user since there is no Queue screen in the application.
- **Suggested Fix:** Remove "Show Queue" from the command palette list since there is no UI to show the queue, or only allow "Clear Queue" and playback control.

### 3. Overlay Modal Footer Rendering Conflict (Medium)
- **File/line:** `internal/app/render.go:42-53`
- **Proof:** If `m.chapterOverlayVisible` is true, the `View()` function returns early:
  ```go
  if m.chapterOverlayVisible {
      return m.overlayChapterModal(content)
  }
  ```
  This bypasses `restoreFooter` which is only called at the end of the normal rendering path.
- **Impact:** When the chapter overlay is visible, the player footer and hint line at the bottom of the screen may be truncated or completely missing (especially on short terminals) because `restoreFooter` is not run.
- **Suggested Fix:** Call `restoreFooter` before applying the chapter overlay, or ensure that `restoreFooter` is invoked for all overlay paths at the end of `View()`.
