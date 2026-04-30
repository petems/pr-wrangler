package tui

import (
	"github.com/charmbracelet/lipgloss"
)

// ColorScheme holds the palette used by the TUI styles.
type ColorScheme struct {
	Primary    lipgloss.Color // title, banner, indicator
	Secondary  lipgloss.Color // loading, help category headings
	Error      lipgloss.Color // error messages
	Warning    lipgloss.Color // warning messages
	Help       lipgloss.Color // general help text
	SelectedBg lipgloss.Color // selected row background
}

// colorSchemes maps scheme names to their palettes.
var colorSchemes = map[string]ColorScheme{
	// default — classic dark-terminal green/cyan palette
	"default": {
		Primary:    lipgloss.Color("#00ff00"),
		Secondary:  lipgloss.Color("#00ffff"),
		Error:      lipgloss.Color("#ff0000"),
		Warning:    lipgloss.Color("#ffcc00"),
		Help:       lipgloss.Color("#666666"),
		SelectedBg: lipgloss.Color("#003d00"),
	},
	// dracula — purple/pink accent inspired by the Dracula theme
	"dracula": {
		Primary:    lipgloss.Color("#bd93f9"),
		Secondary:  lipgloss.Color("#ff79c6"),
		Error:      lipgloss.Color("#ff5555"),
		Warning:    lipgloss.Color("#f1fa8c"),
		Help:       lipgloss.Color("#6272a4"),
		SelectedBg: lipgloss.Color("#44475a"),
	},
	// solarized — Solarized Dark blue/cyan palette
	"solarized": {
		Primary:    lipgloss.Color("#268bd2"),
		Secondary:  lipgloss.Color("#2aa198"),
		Error:      lipgloss.Color("#dc322f"),
		Warning:    lipgloss.Color("#b58900"),
		Help:       lipgloss.Color("#657b83"),
		SelectedBg: lipgloss.Color("#073642"),
	},
	// nord — Nord blue-gray/frost/aurora palette
	"nord": {
		Primary:    lipgloss.Color("#88c0d0"),
		Secondary:  lipgloss.Color("#81a1c1"),
		Error:      lipgloss.Color("#bf616a"),
		Warning:    lipgloss.Color("#ebcb8b"),
		Help:       lipgloss.Color("#4c566a"),
		SelectedBg: lipgloss.Color("#3b4252"),
	},
}

// tableTextColor is a fixed colour used for the table base text regardless of scheme.
var tableTextColor = lipgloss.Color("#ffffff")

var (
	// Text styles — populated by SetColorScheme.
	titleStyle        lipgloss.Style
	bannerStyle       lipgloss.Style
	helpStyle         lipgloss.Style
	errorStyle        lipgloss.Style
	warningStyle      lipgloss.Style
	loadingStyle      lipgloss.Style
	helpCategoryStyle lipgloss.Style
	selectedRowStyle  lipgloss.Style
	indicatorStyle    lipgloss.Style
)

func init() {
	SetColorScheme("default")
}

// SetColorScheme applies the named colour scheme to all TUI style variables.
// Unknown names fall back to "default".
func SetColorScheme(name string) {
	scheme, ok := colorSchemes[name]
	if !ok {
		scheme = colorSchemes["default"]
	}

	titleStyle = lipgloss.NewStyle().
		Bold(true).
		Foreground(scheme.Primary).
		MarginBottom(1)

	bannerStyle = lipgloss.NewStyle().
		Bold(true).
		Foreground(scheme.Primary)

	helpStyle = lipgloss.NewStyle().
		Foreground(scheme.Help)

	errorStyle = lipgloss.NewStyle().
		Foreground(scheme.Error)

	warningStyle = lipgloss.NewStyle().
		Foreground(scheme.Warning)

	loadingStyle = lipgloss.NewStyle().
		Foreground(scheme.Secondary)

	helpCategoryStyle = lipgloss.NewStyle().
		Bold(true).
		Foreground(scheme.Secondary)

	selectedRowStyle = lipgloss.NewStyle().
		Foreground(tableTextColor).
		Background(scheme.SelectedBg).
		Bold(true)

	indicatorStyle = lipgloss.NewStyle().
		Foreground(scheme.Primary).
		Bold(true)
}
