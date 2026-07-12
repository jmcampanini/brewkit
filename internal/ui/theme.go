package ui

import (
	"image/color"

	"charm.land/lipgloss/v2"
)

// Theme holds the output color palette.
type Theme struct {
	name     string
	ok       color.Color
	added    color.Color
	upgraded color.Color
	err      color.Color
	detail   color.Color
	warn     color.Color
	spinner  color.Color
}

// ThemeForBackground returns the theme matching the terminal background.
func ThemeForBackground(dark bool) Theme {
	if dark {
		return DarkTheme()
	}
	return LightTheme()
}

// LightTheme returns the palette for light terminal backgrounds.
func LightTheme() Theme {
	return Theme{
		name:     "Catppuccin Latte",
		ok:       lipgloss.Color("#40a02b"),
		added:    lipgloss.Color("#1e66f5"),
		upgraded: lipgloss.Color("#8839ef"),
		err:      lipgloss.Color("#d20f39"),
		detail:   lipgloss.Color("#5c5f77"),
		warn:     lipgloss.Color("#df8e1d"),
		spinner:  lipgloss.Color("#179299"),
	}
}

// DarkTheme returns the palette for dark terminal backgrounds.
func DarkTheme() Theme {
	return Theme{
		name:     "Catppuccin Frappe",
		ok:       lipgloss.Color("#a6d189"),
		added:    lipgloss.Color("#8caaee"),
		upgraded: lipgloss.Color("#ca9ee6"),
		err:      lipgloss.Color("#e78284"),
		detail:   lipgloss.Color("#b5bfe2"),
		warn:     lipgloss.Color("#e5c890"),
		spinner:  lipgloss.Color("#81c8be"),
	}
}
