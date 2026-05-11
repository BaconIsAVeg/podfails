package tui

import (
	"github.com/BaconIsAVeg/github-tuis/ui/statusbar"
)

func tableKeybindings() []statusbar.KeyBinding {
	return []statusbar.KeyBinding{
		{Key: "↑/↓", Desc: "navigate"},
		{Key: "enter", Desc: "select"},
		{Key: "r", Desc: "refresh"},
		{Key: "q", Desc: "quit"},
	}
}

func detailKeybindings() []statusbar.KeyBinding {
	return []statusbar.KeyBinding{
		{Key: "↑/↓", Desc: "scroll"},
		{Key: "esc", Desc: "back"},
		{Key: "q", Desc: "quit"},
	}
}
