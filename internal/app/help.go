package app

import "github.com/Thelost77/pine/internal/ui/components"

// helpGroups returns the keybinding groups for the help overlay.
func helpGroups() []components.HelpGroup {
	return []components.HelpGroup{
		{
			Title: "Global",
			Bindings: []components.HelpBinding{
				{Key: "q", Desc: "quit"},
				{Key: "esc / ←", Desc: "go back"},
				{Key: "enter / →", Desc: "open / select"},
				{Key: "?", Desc: "toggle help"},
				{Key: "R", Desc: "listening sessions"},
			},
		},
		{
			Title: "Home",
			Bindings: []components.HelpBinding{
				{Key: "o", Desc: "open library"},
				{Key: "tab", Desc: "switch library"},
			},
		},
		{
			Title: "Library",
			Bindings: []components.HelpBinding{
				{Key: "s", Desc: "browse series (books)"},
				{Key: "tab", Desc: "switch library"},
			},
		},
		{
			Title: "Navigation",
			Bindings: []components.HelpBinding{
				{Key: "j / ↓", Desc: "move down"},
				{Key: "k / ↑", Desc: "move up"},
				{Key: "H / L", Desc: "page up / down"},
				{Key: "tab", Desc: "toggle focus"},
			},
		},
		{
			Title: "Player",
			Bindings: []components.HelpBinding{
				{Key: "p / space", Desc: "play / pause"},
				{Key: "l", Desc: "seek forward"},
				{Key: "h", Desc: "seek backward"},
				{Key: ">", Desc: "play next queued"},
				{Key: "+", Desc: "speed up"},
				{Key: "-", Desc: "speed down"},
				{Key: "] / [", Desc: "volume up / down"},
				{Key: "C", Desc: "open chapters"},
				{Key: "c", Desc: "toggle captions"},
				{Key: "S", Desc: "sleep timer"},
			},
		},
		{
			Title: "Detail",
			Bindings: []components.HelpBinding{
				{Key: "enter / p", Desc: "play"},
				{Key: "b", Desc: "add bookmark"},
				{Key: "a", Desc: "add to queue"},
				{Key: "A", Desc: "play next"},
				{Key: "d", Desc: "delete bookmark"},
				{Key: "f", Desc: "mark finished"},
			},
		},
		{
			Title: "Series",
			Bindings: []components.HelpBinding{
				{Key: "enter", Desc: "open selected book"},
				{Key: "H / L", Desc: "page up / down"},
				{Key: "esc / ←", Desc: "back"},
			},
		},
		{
			Title: "Listening Sessions",
			Bindings: []components.HelpBinding{
				{Key: "enter", Desc: "play from here"},
				{Key: "/", Desc: "filter"},
				{Key: "H / L", Desc: "page up / down"},
			},
		},
	}
}
