package tui

import (
	"github.com/charmbracelet/lipgloss"
)

var (
	// Colors
	white = lipgloss.Color("#ffffff")
	gray  = lipgloss.Color("#666666")
	green = lipgloss.Color("#00ff00")
	red   = lipgloss.Color("#ff0000")
	cyan  = lipgloss.Color("#00ffff")

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

	loadingStyle = lipgloss.NewStyle().
			Foreground(cyan)

	helpCategoryStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(cyan)
)
