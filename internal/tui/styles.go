package tui

import (
	"github.com/charmbracelet/lipgloss"
)

var (
	// Colors
	white  = lipgloss.Color("#ffffff")
	gray   = lipgloss.Color("#666666")
	green  = lipgloss.Color("#00ff00")
	red    = lipgloss.Color("#ff0000")
	yellow = lipgloss.Color("#ffff00")
	cyan   = lipgloss.Color("#00ffff")
	blue   = lipgloss.Color("#0000ff")
	purple = lipgloss.Color("#ff00ff")

	// Table styles
	selectedBg = lipgloss.Color("#222222")
	rowEvenBg  = lipgloss.Color("#000000")
	rowOddBg   = lipgloss.Color("#0a0a0a")

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

	searchBarStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(helpColor).
			Padding(0, 1)

	searchBarActiveStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(cyan).
				Padding(0, 1)

	helpCategoryStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(cyan)

	spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
)
