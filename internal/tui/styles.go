package tui

import (
	"image/color"

	"charm.land/lipgloss/v2"
)

// ColorScheme holds the palette used by the TUI styles.
type ColorScheme struct {
	Primary    color.Color // title, banner, indicator
	Secondary  color.Color // loading, help category headings
	Error      color.Color // error messages
	Warning    color.Color // warning messages
	Help       color.Color // general help text
	SelectedBg color.Color // selected row background
	TableText  color.Color // table base text foreground
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
		TableText:  lipgloss.Color("#ffffff"),
	},
	// dracula — purple/pink accent inspired by the Dracula theme
	"dracula": {
		Primary:    lipgloss.Color("#bd93f9"),
		Secondary:  lipgloss.Color("#ff79c6"),
		Error:      lipgloss.Color("#ff5555"),
		Warning:    lipgloss.Color("#f1fa8c"),
		Help:       lipgloss.Color("#6272a4"),
		SelectedBg: lipgloss.Color("#44475a"),
		TableText:  lipgloss.Color("#f8f8f2"),
	},
	// solarized — Solarized Dark blue/cyan palette
	"solarized": {
		Primary:    lipgloss.Color("#268bd2"),
		Secondary:  lipgloss.Color("#2aa198"),
		Error:      lipgloss.Color("#dc322f"),
		Warning:    lipgloss.Color("#b58900"),
		Help:       lipgloss.Color("#657b83"),
		SelectedBg: lipgloss.Color("#073642"),
		TableText:  lipgloss.Color("#839496"),
	},
	// nord — Nord blue-gray/frost/aurora palette
	"nord": {
		Primary:    lipgloss.Color("#88c0d0"),
		Secondary:  lipgloss.Color("#81a1c1"),
		Error:      lipgloss.Color("#bf616a"),
		Warning:    lipgloss.Color("#ebcb8b"),
		Help:       lipgloss.Color("#4c566a"),
		SelectedBg: lipgloss.Color("#3b4252"),
		TableText:  lipgloss.Color("#d8dee9"),
	},
}

// ThemeNames lists the available colour schemes in display order. This is
// the source of truth for the theme picker UI; colorSchemes must contain an
// entry for every name listed here.
var ThemeNames = []string{"default", "dracula", "solarized", "nord"}

// Styles holds the rendered lipgloss styles for a single Model instance.
// Each Model owns its own Styles, so multiple TUI instances can run in the
// same process with independent themes and tests can construct deterministic
// styles without mutating package state.
type Styles struct {
	TableText    color.Color
	Title        lipgloss.Style
	Banner       lipgloss.Style
	Help         lipgloss.Style
	Error        lipgloss.Style
	Warning      lipgloss.Style
	Loading      lipgloss.Style
	HelpCategory lipgloss.Style
	SelectedRow  lipgloss.Style
	Indicator    lipgloss.Style
}

// NewStyles builds a Styles for the named color scheme. Unknown names fall
// back to "default".
func NewStyles(name string) Styles {
	scheme, ok := colorSchemes[name]
	if !ok {
		scheme = colorSchemes["default"]
	}

	return Styles{
		TableText: scheme.TableText,
		Title: lipgloss.NewStyle().
			Bold(true).
			Foreground(scheme.Primary).
			MarginBottom(1),
		Banner: lipgloss.NewStyle().
			Bold(true).
			Foreground(scheme.Primary),
		Help: lipgloss.NewStyle().
			Foreground(scheme.Help),
		Error: lipgloss.NewStyle().
			Foreground(scheme.Error),
		Warning: lipgloss.NewStyle().
			Foreground(scheme.Warning),
		Loading: lipgloss.NewStyle().
			Foreground(scheme.Secondary),
		HelpCategory: lipgloss.NewStyle().
			Bold(true).
			Foreground(scheme.Secondary),
		SelectedRow: lipgloss.NewStyle().
			Foreground(scheme.TableText).
			Background(scheme.SelectedBg).
			Bold(true),
		Indicator: lipgloss.NewStyle().
			Foreground(scheme.Primary).
			Bold(true),
	}
}
