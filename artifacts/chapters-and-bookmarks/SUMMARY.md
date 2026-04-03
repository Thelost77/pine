# Summary: chapters-and-bookmarks

## Progress

### Task 1: Add root chapter overlay state
- **Status:** done
- **Commit:** b853322 feat(app): add chapter overlay state
- **Deviations:** none
- **Decisions:** Root owns overlay state; TDD started with root-model tests first.

### Task 2: Render chapter overlay
- **Status:** done
- **Commit:** 06505b7 feat(app): render chapter overlay modal
- **Deviations:** none
- **Decisions:** none

### Task 3: Wire overlay lifecycle
- **Status:** done
- **Commit:** b0de7f2 fix(app): reset chapter overlay lifecycle
- **Deviations:** none
- **Decisions:** none

### Task 4: Seek from overlay
- **Status:** done
- **Commit:** 77efd08 feat(app): seek chapters from overlay
- **Deviations:** none
- **Decisions:** none

### Task 5: Remove inline chapter UI
- **Status:** done
- **Commit:** df87daa refactor(detail): remove inline chapter UI
- **Deviations:** none
- **Decisions:** none

### Task 6: Add end-to-end coverage
- **Status:** done
- **Commit:** 18bb5f5 test(app): cover chapter overlay e2e flow
- **Deviations:** none
- **Decisions:** none

### Task 7: Run final verification
- **Status:** done
- **Commit:** n/a
- **Deviations:** used `go build -o /tmp/pine-test-build ./cmd/` because `go build ./cmd/` attempts to write a binary named `cmd`, which conflicts with the existing directory name.
- **Decisions:** Full automated verification passed with the adjusted build command and `go test ./... -count=1`.

### Debug follow-up: Resolve chapter count/title confusion
- **Status:** done
- **Commit:** pending
- **Deviations:** none
- **Decisions:** The apparent `86` vs `98` mismatch was not a bad seek index. Live ABS payload inspection showed the book had `98` audio tracks but only `86` chapters, and ABS chapter titles reflected raw track numbering. The overlay was updated to open on the current chapter, support `H`/`L` top-bottom jumps, and render pine chapter ordinals alongside the raw ABS title to remove the ambiguity.

### Debug follow-up: Add higher-signal runtime logging
- **Status:** done
- **Commit:** pending
- **Deviations:** none
- **Decisions:** Added structured logs around ABS requests, mpv lifecycle/IPC failures, playback track selection and sync/cleanup, seek transitions, DB open/migrations, screen transitions, and login attempts. Also removed logging of the full tokenized stream URL.

## Current behavior notes
- Chapters are now playback-scoped in the root overlay.
- Bookmarks remain separate from the chapter overlay and were not expanded in this task.
- Bookmark UX is still a separate unfinished topic.

## Metrics
- Tasks completed: 9/9
- Deviations: 1
