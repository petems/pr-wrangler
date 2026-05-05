package tui

import (
	"github.com/charmbracelet/x/ansi"
)

// hyperlinksEnabled gates emission of OSC 8 sequences. lipgloss v2 + bubbletea
// v2 handle ANSI downsampling via colorprofile at the renderer layer, so
// Link() defers to that rather than probing the terminal itself. Tests flip
// this to make Link()'s output deterministic.
var hyperlinksEnabled = true

// Link wraps text with an OSC 8 hyperlink escape sequence so supporting
// terminals (iTerm2, WezTerm, Kitty, Ghostty, modern Terminal.app, GNOME
// Terminal, VS Code) render text as Cmd/Ctrl+Click-able. When disabled, or
// url/text is empty, Link returns text unchanged so piped output stays clean.
//
// Inside lipgloss styles prefer Style.Hyperlink(url) — it integrates with
// width math via charmbracelet/x/ansi. Use Link() for inline wrapping in
// places that don't have a style (e.g. error message string interpolation).
func Link(url, text string) string {
	if url == "" || text == "" {
		return text
	}
	if !hyperlinksEnabled {
		return text
	}
	return ansi.SetHyperlink(url) + text + ansi.ResetHyperlink()
}
