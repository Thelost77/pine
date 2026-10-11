# Theme configuration

Spruce keeps theme support as editable color values in `config.toml`.

The default theme is a curated Everforest palette. Users can change colors by
editing the existing `[theme]` fields.

Do not add runtime theme switching, named theme registries, command palette theme
actions, appearance presets, UI size settings, or terminal font size settings.

We tested broader theme support. It added complexity across app model wiring,
screen rebuilds, command palette actions, config shape, and tests for little user
value. The better experience is a stable default plus direct config values for
users who want to tune colors.

Terminal font size and terminal window size are outside Spruce control. Mapping a
font-size setting to spacing or layout behavior is misleading and creates fragile
UI behavior.

Predefined themes can be reconsidered later as commented config examples only.
For now, keep one curated Everforest default and no preset system.
