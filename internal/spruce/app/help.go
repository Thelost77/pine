package app

import "github.com/Thelost77/pine/internal/ui/components"

// helpGroups returns the keybinding groups for the help overlay.
func helpGroups() []components.HelpGroup {
	return []components.HelpGroup{
		{
			Title: "Global",
			Bindings: []components.HelpBinding{
				{Key: "q", Desc: "quit"},
				{Key: "esc / ←", Desc: "go back / close filter"},
				{Key: "enter / →", Desc: "open / select"},
				{Key: "tab", Desc: "switch library/playlists and queue"},
				{Key: "o", Desc: "open playlists"},
				{Key: "ctrl+p", Desc: "command palette"},
				{Key: "?", Desc: "toggle help"},
			},
		},
		{
			Title: "Library",
			Bindings: []components.HelpBinding{
				{Key: "/", Desc: "filter list"},
				{Key: "enter", Desc: "open album / play track"},
				{Key: "a", Desc: "add track / album to queue"},
				{Key: "A", Desc: "queue album"},
				{Key: "S", Desc: "shuffle album to queue"},
				{Key: "t", Desc: "toggle track sort"},
				{Key: "m", Desc: "edit metadata"},
			},
		},
		{
			Title: "Playlists",
			Bindings: []components.HelpBinding{
				{Key: "/", Desc: "filter list"},
				{Key: "enter", Desc: "open playlist / play track"},
				{Key: "a", Desc: "add track / playlist to queue"},
				{Key: "A", Desc: "queue playlist"},
				{Key: "S", Desc: "shuffle playlist to queue"},
			},
		},
		{
			Title: "Queue",
			Bindings: []components.HelpBinding{
				{Key: "enter", Desc: "jump to track"},
				{Key: "space", Desc: "play / pause"},
				{Key: "< / >", Desc: "previous / next track"},
				{Key: "s", Desc: "shuffle queue"},
				{Key: "r / R", Desc: "repeat track / queue"},
				{Key: "b", Desc: "toggle subliminal mode"},
				{Key: "m", Desc: "edit metadata"},
				{Key: "d / x", Desc: "remove track"},
				{Key: "c", Desc: "clear queue"},
				{Key: "/", Desc: "filter queue"},
			},
		},
		{
			Title: "Player",
			Bindings: []components.HelpBinding{
				{Key: "space", Desc: "play / pause"},
				{Key: "l", Desc: "seek forward"},
				{Key: "h", Desc: "seek backward"},
				{Key: ">", Desc: "play next queued"},
				{Key: "<", Desc: "play previous"},
				{Key: "+", Desc: "speed up"},
				{Key: "-", Desc: "speed down"},
				{Key: "] / [", Desc: "volume up / down"},
				{Key: "s", Desc: "shuffle"},
				{Key: "S", Desc: "sleep timer (queue)"},
			},
		},
	}
}
