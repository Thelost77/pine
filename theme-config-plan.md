# Theme Config Findings And Plan

## Current Findings

- Pine already has `[theme]` fields in `internal/config/config.go`.
- `main.go` loads `~/.config/pine/config.toml`.
- `internal/app/model.go` already uses `styles := ui.NewStyles(cfg.Theme)`.
- So Pine already applies configured theme at startup.
- Everforest should remain the default. That default is the target experience.
- `warning` exists in config/tests but is not used by style construction.
- Palette selected row currently hardcodes Everforest foreground in `internal/ui/components/palette.go`, so configured themes can still leak default colors.
- README only points users to `internal/config/config.go`; that is not a good user-facing config doc.

## What Went Wrong In The Abandoned Attempt

- Theme presets were not valuable enough and made most themes look similar or bad.
- Runtime theme switching created too much code and too much visual risk.
- `surface` and UI size config changed default rendering in bad ways.
- Terminal font size cannot be controlled by Pine. Mapping size to row spacing breaks layout and expectations.
- Command palette appearance actions created the illusion of useful configurability.

## Proper Minimal Plan

1. Keep config shape stable.
   - Keep existing `[theme]` color fields.
   - Do not add named presets.
   - Do not add `[appearance]`.
   - Do not add command palette actions for theme or size.

2. Keep Pine startup theme behavior.
   - No change needed for `ui.NewStyles(cfg.Theme)`.
   - Avoid live style propagation unless there is a separate, explicit runtime theme feature later.

3. Remove hardcoded palette foreground.
   - In `components.Palette.SetStyles`, do not force `#d3c6aa`.
   - Let `styles.Selected` decide both foreground and background.

4. Decide `warning`.
   - Preferred: add `Warning lipgloss.Style` only if an existing warning UI should use it.
   - Otherwise keep field for compatibility and do not expand behavior.

5. Improve docs only.
   - README should show a small Everforest `[theme]` example.
   - Mention that config is optional.
   - Do not advertise presets or runtime switching.

6. Add focused tests.
   - `app.New` with custom `Theme.Accent` produces model styles using that accent.
   - Palette selected style does not hardcode default foreground.

## Non-Goals

- No runtime theme switching.
- No theme preset registry.
- No UI size or font size setting.
- No command palette appearance section.
- No broad visual redesign.

## Success Criteria

- Default Pine looks the same as before.
- Existing startup theme config keeps working.
- Palette no longer leaks hardcoded Everforest foreground.
- README gives enough config guidance without pretending Pine has a full theme system.
- Diff stays small and easy to review.
