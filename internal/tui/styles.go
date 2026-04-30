package tui

import (
	"github.com/charmbracelet/lipgloss"
)

var (
	// Colors
	white     = lipgloss.Color("#ffffff")
	gray      = lipgloss.Color("#666666")
	green     = lipgloss.Color("#00ff00")
	darkGreen = lipgloss.Color("#003d00")
	red       = lipgloss.Color("#ff0000")
	cyan      = lipgloss.Color("#00ffff")
	yellow    = lipgloss.Color("#ffcc00")

	// Text styles
	helpColor = gray

	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(green).
			MarginBottom(1)

	bannerStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(green)

	helpStyle = lipgloss.NewStyle().
			Foreground(helpColor)

	errorStyle = lipgloss.NewStyle().
			Foreground(red)

	warningStyle = lipgloss.NewStyle().
			Foreground(yellow)

	loadingStyle = lipgloss.NewStyle().
			Foreground(cyan)

	helpCategoryStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(cyan)

	selectedRowStyle = lipgloss.NewStyle().
				Foreground(white).
				Background(darkGreen).
				Bold(true)

	indicatorStyle = lipgloss.NewStyle().
			Foreground(green).
			Bold(true)
)
