package tui

import (
	"strings"
	"sync"
	"testing"

	"github.com/charmbracelet/lipgloss"
)
var hyperlinkStateMu sync.Mutex

// withHyperlinks toggles the package-level enable flag for the duration of a
// test, returning a cleanup function. Necessary because detectHyperlinkSupport
// is a one-shot probe of the real terminal at init time.
func withHyperlinks(t *testing.T, enabled bool) func() {
	t.Helper()
	hyperlinkStateMu.Lock()
	prev := hyperlinksEnabled
	hyperlinksEnabled = enabled
	return func() {
		hyperlinksEnabled = prev
		hyperlinkStateMu.Unlock()
	}
}

func TestLink_TTYEnabled_EmitsOSC8(t *testing.T) {
	defer withHyperlinks(t, true)()

	got := Link("https://example.com/pr/1", "#1")
	want := "\x1b]8;;https://example.com/pr/1\x1b\\#1\x1b]8;;\x1b\\"
	if got != want {
		t.Errorf("Link(): got %q, want %q", got, want)
	}
}

func TestLink_NonTTY_ReturnsPlainText(t *testing.T) {
	defer withHyperlinks(t, false)()

	got := Link("https://example.com/pr/1", "#1")
	if got != "#1" {
		t.Errorf("Link(): got %q, want plain text %q", got, "#1")
	}
	if strings.Contains(got, "\x1b") {
		t.Errorf("Link() leaked escape sequence in non-TTY output: %q", got)
	}
}

func TestLink_EmptyURL_ReturnsText(t *testing.T) {
	defer withHyperlinks(t, true)()

	if got := Link("", "title"); got != "title" {
		t.Errorf("Link(\"\", \"title\"): got %q, want %q", got, "title")
	}
}

func TestLink_EmptyText_ReturnsEmpty(t *testing.T) {
	defer withHyperlinks(t, true)()

	if got := Link("https://example.com", ""); got != "" {
		t.Errorf("Link(url, \"\"): got %q, want empty string", got)
	}
}

func TestLink_PreservesVisibleWidth(t *testing.T) {
	defer withHyperlinks(t, true)()

	const text = "octocat/hello-world"
	plain := lipgloss.Width(text)
	linked := lipgloss.Width(Link("https://github.com/octocat/hello-world", text))

	if plain != linked {
		t.Errorf("lipgloss.Width should ignore OSC 8 wrapping: plain=%d linked=%d", plain, linked)
	}
}
