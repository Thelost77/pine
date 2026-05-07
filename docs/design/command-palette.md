# Command Palette Design

## Status

Draft — ready for implementation

## Context

jellyfin-tui has a global popup / command palette system (`Shift+P` for global, `p` for context-sensitive) that provides a searchable, overlay-based menu for all app actions. This UX pattern eliminates the need to remember context-specific keybindings and makes every action reachable from any screen through a single searchable interface.

Pine currently lacks this pattern. Navigation between screens, playback controls, and context actions are all bound to specific keys that only work on specific screens. This creates cognitive load — users must remember where they are and which keys are active there.

## Decision

Add a command palette to pine with two triggers:

- **Global palette** (`:` or `Ctrl+P`) — app-wide actions available from any screen
- **Context palette** (`;` when not playing) — actions relevant to the current screen and selected item

The palette will be a searchable, overlay-based menu with submenu support, matching the jellyfin-tui pattern.

## Design

### Triggers

| Key | Behavior | Rationale |
|-----|----------|-----------|
| `:` | Open global palette | Vim-inspired, doesn't conflict with `p` (play/pause) |
| `Ctrl+P` | Also opens global palette | Common shortcut in many apps |
| `;` | Open context palette | Semicolon is near `:` on most keyboards, easy to remember as "context" |

`p` remains play/pause during playback. The context palette only activates when playback is stopped.

### Global Palette Items

```
 pine › Command Palette
 ──────────────────────
 > go home
 > go library
 > go search
 > go series list
 ──────────────────────
 > play / pause
 > next chapter
 > prev chapter
 > seek forward
 > seek backward
 > speed up
 > speed down
 ──────────────────────
 > sleep timer: 15m
 > sleep timer: 30m
 > sleep timer: off
 ──────────────────────
 > queue: show
 > queue: clear
 ──────────────────────
 > switch library: <name>
```

### Context Palette Items (by Screen)

| Screen | Available Actions |
|--------|-------------------|
| **Home** | Open selected, Queue, Play Next, Open Library, Search, Switch Library |
| **Library** | Open selected, Queue, Play Next, Browse Series, Search, Switch Library |
| **Detail** | Play, Add Bookmark, Queue, Play Next, Mark Finished, Go to Series |
| **Search** | Open selected, Queue, Play Next |
| **Series** | Open selected, Queue, Play Next |
| **SeriesList** | Open selected |

### Submenu Examples

```
 Command Palette > Sleep Timer    Command Palette > Switch Library
 ───────────────────────────      ────────────────────────────────
 > 15 minutes                     > Audiobooks
 > 30 minutes                     > Podcasts
 > 45 minutes                     > All Libraries
 > 60 minutes
 > Off
```

`Esc` goes back to the parent menu. `Enter` selects. Typing filters items.

### Search Behavior

Use **fuzzy subsequence matching** via `sahilm/fuzzy` (already a dependency in pine's search screen). This gives ranked results and character highlighting for free.

Example: typing "sl" matches "**S**leep timer: 15m" and "**S**witch **l**ibrary".

### Overlay Rendering

The palette renders as a **centered modal overlay** on top of the existing screen content, using the same `normalizeOverlayCanvas` + line-by-line insertion technique already used by the chapter overlay.

Size: ~50 chars wide, height dynamic up to ~60% of screen.

## Architecture

### New Files

```
internal/ui/components/palette.go      # Reusable Palette component
internal/app/palette.go                # Menu builders + action handlers
internal/app/palette_test.go           # Unit tests for palette integration
```

### Modified Files

```
internal/app/model.go       # Add palette state + key interception
internal/app/render.go      # Add palette overlay rendering
internal/app/keymap.go      # Add palette trigger bindings
internal/app/messages.go    # Add palette action messages
```

### Component: `Palette`

```go
type PaletteItem struct {
    Label   string
    Action  PaletteAction
    Submenu *PaletteMenu
}

type PaletteMenu struct {
    Title  string
    Items  []PaletteItem
    Parent *PaletteMenu
}

type Palette struct {
    visible    bool
    global     bool
    menuStack  []*PaletteMenu
    list       list.Model    // bubbles/list
    width      int
    height     int
    styles     ui.Styles
    searchMode bool
    searchTerm string
}
```

Key methods:
- `Open(global bool, root *PaletteMenu)` — show palette, reset selection
- `Close()` — hide and clear stack
- `Update(msg tea.Msg) (Palette, tea.Cmd)` — handle navigation, search, selection
- `View() string` — render centered overlay with border
- `CurrentAction() (PaletteAction, bool)` — return selected action when Enter pressed

### Root Model Integration

Add to `Model`:
```go
palette     components.Palette
paletteOpen bool
```

Key interception order in `Update()` (before screen dispatch):

1. `Ctrl+C` — quit
2. `?` — help overlay
3. **Palette open** — all keys go to palette
4. `:` / `Ctrl+P` — open global palette
5. `;` — open context palette (when not playing)
6. Chapter overlay
7. Error banner dismiss
8. Existing screen + playback keys

When palette is open and `CurrentAction()` returns an action:
```go
func (m Model) handlePaletteAction(action components.PaletteAction) (Model, tea.Cmd) {
    m.palette.Close()
    m.paletteOpen = false
    switch action {
    case ActionGoHome:      return m.navigate(ScreenHome)
    case ActionGoLibrary:   return m.navigate(ScreenLibrary)
    case ActionTogglePlay:  return m.togglePlayViaPalette()
    case ActionNextChapter: return m.seekToChapter(m.nextChapter())
    // ... etc
    }
    return m, nil
}
```

### Visual Design

Use existing pine theme colors:
- Border: `styles.Border`
- Title: `styles.Title`
- Selected row: `styles.Selected` (bg `#475258`, fg `#d3c6aa`, bold)
- Normal row: fg `#d3c6aa`
- Muted text: `styles.Muted`
- Search prompt: `styles.Muted` with underscore cursor

Footer hint: `↑↓ navigate • enter select • esc close/back`

### Testing

Following pine's existing patterns:

**Component tests** (`internal/ui/components/palette_test.go`):
- `TestPaletteOpenClose`
- `TestPaletteNavigationUpDown`
- `TestPaletteSearchFilter`
- `TestPaletteSubmenuPushPop`
- `TestPaletteEmptySearch`

**App tests** (`internal/app/palette_test.go`):
- `TestGlobalPaletteOpens`
- `TestContextPaletteShowsScreenActions`
- `TestPaletteActionNavigatesScreen`
- `TestPaletteClosesOnEsc`
- `TestPaletteSubmenuBackOnEsc`

**E2E tests** (`internal/app/e2e_test.go`):
- Open palette from Home, select "Go to Library", assert screen = Library
- Open palette from Detail, select "Add Bookmark", assert bookmark created

## Open Questions

1. **Should `p` open the context palette when not playing?** Currently `p` is play/pause. On non-playing screens, it could be repurposed for the palette, but this creates mode-dependent behavior. The `;` key avoids this ambiguity.

2. **Should the global palette include screen-specific actions?** jellyfin-tui keeps them strictly separate. For pine, the global palette could include a "Current screen actions" section that dynamically shows context actions, making the global palette a superset.

3. **Queue management in palette:** Should "Show queue" open a submenu listing queued items with "Play now / Remove" actions? Or should it navigate to a dedicated queue screen?

4. **Library switching:** Should the library switcher submenu fetch library names dynamically, or should it use cached libraries from the home screen?

## References

- jellyfin-tui popup implementation: `src/popup.rs`, `src/keyboard.rs`
- Pine chapter overlay: `internal/app/render.go:overlayChapterModal`
- Pine help overlay: `internal/ui/components/help.go`
- Fuzzy search library: `github.com/sahilm/fuzzy` (already used in `internal/screens/search`)
- Bubble Tea list component: `github.com/charmbracelet/bubbles/list`
