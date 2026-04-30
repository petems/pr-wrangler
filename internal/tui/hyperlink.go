package tui

import (
	"github.com/muesli/termenv"
)

// hyperlinksEnabled is resolved once on package init to avoid querying the
// terminal profile for every cell render. Tests can override it.
var hyperlinksEnabled = detectHyperlinkSupport()

// detectHyperlinkSupport returns true when the active output supports
// styling (i.e. is a TTY with a non-Ascii profile and NO_COLOR is unset).
// We piggyback on termenv's profile detection so behavior stays consistent
// with how lipgloss decides whether to emit color escapes.
func detectHyperlinkSupport() bool {
	out := termenv.DefaultOutput()
	if out == nil {
		return false
	}
	return out.Profile != termenv.Ascii
}

// Link wraps text with an OSC 8 hyperlink escape sequence so supporting
// terminals (iTerm2, WezTerm, Kitty, Ghostty, modern Terminal.app, GNOME
// Terminal, VS Code) render text as Cmd/Ctrl+Click-able. When the output
// is not a TTY, NO_COLOR is set, or url/text is empty, Link returns text
// unchanged so piped output and CI logs stay clean.
//
// The escape sequence used is: ESC ] 8 ; ; URL ESC \ TEXT ESC ] 8 ; ; ESC \
func Link(url, text string) string {
	if url == "" || text == "" {
		return text
	}
	if !hyperlinksEnabled {
		return text
	}
	return "\x1b]8;;" + url + "\x1b\\" + text + "\x1b]8;;\x1b\\"
}
