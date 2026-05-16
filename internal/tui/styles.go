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
	Success    color.Color // approved/mergeable states and merge actions
	Info       color.Color // waiting/open/investigate states
	Review     color.Color // feedback and review states/actions
	Conflict   color.Color // merge-conflict states/actions
	Draft      color.Color // draft states
	Help       color.Color // general help text
	Border     color.Color // table border and structural chrome
	Header     color.Color // table header text
	Repo       color.Color // repository column
	Number     color.Color // PR number column
	TitleText  color.Color // PR title column
	SelectedBg color.Color // selected row background
	TableText  color.Color // table base text foreground
}

// colorSchemes maps scheme names to their palettes.
var colorSchemes = map[string]ColorScheme{
	// default — classic dark-terminal green/cyan palette
	"default": {
		Primary:    lipgloss.Color("#00ff00"),
		Secondary:  lipgloss.Color("#00ffff"),
		Error:      lipgloss.Color("#ff5f7a"),
		Warning:    lipgloss.Color("#ffd166"),
		Success:    lipgloss.Color("#5cffb1"),
		Info:       lipgloss.Color("#7dd3fc"),
		Review:     lipgloss.Color("#f0abfc"),
		Conflict:   lipgloss.Color("#ff9f6e"),
		Draft:      lipgloss.Color("#b6a4ff"),
		Help:       lipgloss.Color("#8a948f"),
		Border:     lipgloss.Color("#28514a"),
		Header:     lipgloss.Color("#b8fff2"),
		Repo:       lipgloss.Color("#80ed99"),
		Number:     lipgloss.Color("#64dfdf"),
		TitleText:  lipgloss.Color("#e8f6ef"),
		SelectedBg: lipgloss.Color("#123a31"),
		TableText:  lipgloss.Color("#dff7ec"),
	},
	// dracula — purple/pink accent inspired by the Dracula theme
	"dracula": {
		Primary:    lipgloss.Color("#bd93f9"),
		Secondary:  lipgloss.Color("#ff79c6"),
		Error:      lipgloss.Color("#ff5555"),
		Warning:    lipgloss.Color("#f1fa8c"),
		Success:    lipgloss.Color("#50fa7b"),
		Info:       lipgloss.Color("#8be9fd"),
		Review:     lipgloss.Color("#ff92df"),
		Conflict:   lipgloss.Color("#ffb86c"),
		Draft:      lipgloss.Color("#caa9fa"),
		Help:       lipgloss.Color("#8f9bd6"),
		Border:     lipgloss.Color("#5b6178"),
		Header:     lipgloss.Color("#f1fa8c"),
		Repo:       lipgloss.Color("#8be9fd"),
		Number:     lipgloss.Color("#ffb86c"),
		TitleText:  lipgloss.Color("#f8f8f2"),
		SelectedBg: lipgloss.Color("#3b4258"),
		TableText:  lipgloss.Color("#f8f8f2"),
	},
	// solarized — Solarized Dark blue/cyan palette
	"solarized": {
		Primary:    lipgloss.Color("#268bd2"),
		Secondary:  lipgloss.Color("#2aa198"),
		Error:      lipgloss.Color("#ff6b61"),
		Warning:    lipgloss.Color("#d6a73f"),
		Success:    lipgloss.Color("#7fcf7f"),
		Info:       lipgloss.Color("#66c2d1"),
		Review:     lipgloss.Color("#d28bd2"),
		Conflict:   lipgloss.Color("#e89f5c"),
		Draft:      lipgloss.Color("#a5a8ff"),
		Help:       lipgloss.Color("#8fa1a1"),
		Border:     lipgloss.Color("#31545f"),
		Header:     lipgloss.Color("#93d7d0"),
		Repo:       lipgloss.Color("#66c2d1"),
		Number:     lipgloss.Color("#d6a73f"),
		TitleText:  lipgloss.Color("#d6e4df"),
		SelectedBg: lipgloss.Color("#123f49"),
		TableText:  lipgloss.Color("#c8d8d2"),
	},
	// nord — Nord blue-gray/frost/aurora palette
	"nord": {
		Primary:    lipgloss.Color("#88c0d0"),
		Secondary:  lipgloss.Color("#81a1c1"),
		Error:      lipgloss.Color("#d77b86"),
		Warning:    lipgloss.Color("#ebcb8b"),
		Success:    lipgloss.Color("#a3be8c"),
		Info:       lipgloss.Color("#8fbcbb"),
		Review:     lipgloss.Color("#b48ead"),
		Conflict:   lipgloss.Color("#d08770"),
		Draft:      lipgloss.Color("#a8b2e6"),
		Help:       lipgloss.Color("#7f8da7"),
		Border:     lipgloss.Color("#4c566a"),
		Header:     lipgloss.Color("#b8d7e2"),
		Repo:       lipgloss.Color("#8fbcbb"),
		Number:     lipgloss.Color("#ebcb8b"),
		TitleText:  lipgloss.Color("#e5e9f0"),
		SelectedBg: lipgloss.Color("#40495d"),
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
	Header       lipgloss.Style
	Border       lipgloss.Style
	Repo         lipgloss.Style
	Number       lipgloss.Style
	TitleText    lipgloss.Style
	Success      lipgloss.Style
	Info         lipgloss.Style
	Review       lipgloss.Style
	Conflict     lipgloss.Style
	Draft        lipgloss.Style
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
		Header: lipgloss.NewStyle().
			Bold(true).
			Foreground(scheme.Header),
		Border: lipgloss.NewStyle().
			Foreground(scheme.Border),
		Repo: lipgloss.NewStyle().
			Foreground(scheme.Repo),
		Number: lipgloss.NewStyle().
			Foreground(scheme.Number),
		TitleText: lipgloss.NewStyle().
			Foreground(scheme.TitleText),
		Success: lipgloss.NewStyle().
			Foreground(scheme.Success),
		Info: lipgloss.NewStyle().
			Foreground(scheme.Info),
		Review: lipgloss.NewStyle().
			Foreground(scheme.Review),
		Conflict: lipgloss.NewStyle().
			Foreground(scheme.Conflict),
		Draft: lipgloss.NewStyle().
			Foreground(scheme.Draft),
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
			Background(scheme.SelectedBg),
		Indicator: lipgloss.NewStyle().
			Foreground(scheme.Primary).
			Bold(true),
	}
}
