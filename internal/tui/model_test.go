package tui

import (
	"strings"
	"testing"

	"github.com/petems/pr-wrangler/internal/config"
	"github.com/petems/pr-wrangler/internal/github"
)

func TestBuildRows_HidesMergedPRs(t *testing.T) {
	prs := []github.PR{
		{Number: 1, Title: "Open PR", State: github.PRStateOpen},
		{Number: 2, Title: "Merged PR", State: github.PRStateMerged},
		{Number: 3, Title: "Another open", State: github.PRStateOpen},
	}

	rows := buildRows(prs)
	if len(rows) != 2 {
		t.Fatalf("expected 2 rows (merged hidden), got %d", len(rows))
	}
	if rows[0].PR.Number != 1 {
		t.Errorf("first row: got PR #%d, want #1", rows[0].PR.Number)
	}
	if rows[1].PR.Number != 3 {
		t.Errorf("second row: got PR #%d, want #3", rows[1].PR.Number)
	}
}

func TestBuildRows_HidesClosedPRs(t *testing.T) {
	prs := []github.PR{
		{Number: 1, Title: "Open PR", State: github.PRStateOpen},
		{Number: 2, Title: "Closed PR", State: github.PRStateClosed},
	}

	rows := buildRows(prs)
	if len(rows) != 1 {
		t.Fatalf("expected 1 row (closed hidden), got %d", len(rows))
	}
	if rows[0].PR.Number != 1 {
		t.Errorf("expected PR #1, got #%d", rows[0].PR.Number)
	}
}

func TestBuildRows_AllMerged(t *testing.T) {
	prs := []github.PR{
		{Number: 1, State: github.PRStateMerged},
		{Number: 2, State: github.PRStateMerged},
	}

	rows := buildRows(prs)
	if len(rows) != 0 {
		t.Fatalf("expected 0 rows, got %d", len(rows))
	}
}

func TestBuildRows_Empty(t *testing.T) {
	rows := buildRows(nil)
	if len(rows) != 0 {
		t.Fatalf("expected 0 rows, got %d", len(rows))
	}
}

// --- renderCowsay tests ---

func TestRenderCowsay_ContainsCowArt(t *testing.T) {
	out := renderCowsay("*", 120, 30)
	for _, expected := range []string{
		"Mooo! Fetching PR's for ya pardner, Yee-haw!",
		`^__^`,
		`(oo)\_______`,
		`(__)\       )\/\`,
		`||----w |`,
	} {
		if !strings.Contains(out, expected) {
			t.Errorf("output missing %q", expected)
		}
	}
}

func TestRenderCowsay_ContainsTitle(t *testing.T) {
	out := renderCowsay("*", 120, 30)
	// The title contains "PR WRANGLER" in block letters; check for distinctive fragments
	if !strings.Contains(out, "\u2597\u2584\u2584\u2596") {
		t.Error("output missing title banner")
	}
}

func TestRenderCowsay_TitleHasNoBlankLineGaps(t *testing.T) {
	out := renderCowsay("*", 120, 30)
	lines := strings.Split(out, "\n")

	// Find the title section: consecutive non-empty lines containing block chars
	titleStart := -1
	titleEnd := -1
	for i, l := range lines {
		plain := stripANSI(l)
		hasBlock := strings.ContainsAny(plain, "\u2584\u2596\u2597\u2590\u258c\u259b\u2580\u2598\u259a\u259f\u2588\u2599\u259e\u259d\u259c\u2584")
		if hasBlock && titleStart == -1 {
			titleStart = i
		}
		if hasBlock {
			titleEnd = i
		}
	}

	if titleStart == -1 {
		t.Fatal("title section not found")
	}

	// Every line between titleStart and titleEnd should be non-empty (no gaps)
	for i := titleStart; i <= titleEnd; i++ {
		plain := stripANSI(lines[i])
		if strings.TrimSpace(plain) == "" {
			t.Errorf("blank line gap in title at line %d (between title lines %d-%d)", i, titleStart, titleEnd)
		}
	}
}

func TestRenderCowsay_TitleLinesHaveUniformPrefix(t *testing.T) {
	out := renderCowsay("*", 120, 30)
	lines := strings.Split(out, "\n")

	// Collect title lines (those with block characters)
	var titlePads []int
	for _, l := range lines {
		plain := stripANSI(l)
		if strings.ContainsAny(plain, "\u2584\u2596\u2597\u2590\u258c\u259b\u2580") {
			pad := len(plain) - len(strings.TrimLeft(plain, " "))
			titlePads = append(titlePads, pad)
		}
	}

	if len(titlePads) == 0 {
		t.Fatal("no title lines found")
	}

	// All title lines should have the same left padding
	for i, pad := range titlePads {
		if pad != titlePads[0] {
			t.Errorf("title line %d: padding %d, want %d (same as first line)", i, pad, titlePads[0])
		}
	}
}

func TestRenderCowsay_ContainsSpinner(t *testing.T) {
	out := renderCowsay("XYZ", 120, 30)
	if !strings.Contains(out, "XYZ") {
		t.Error("spinner string not found in output")
	}
}

func TestRenderCowsay_CowLinesHaveUniformPrefix(t *testing.T) {
	out := renderCowsay("*", 120, 30)
	lines := strings.Split(out, "\n")

	// Find the cow section by looking for the speech bubble
	inCow := false
	cowMinPad := -1
	for _, l := range lines {
		plain := stripANSI(l)
		if strings.Contains(plain, "______________________") {
			inCow = true
		}
		if !inCow || strings.TrimSpace(plain) == "" {
			continue
		}
		pad := len(plain) - len(strings.TrimLeft(plain, " "))
		if cowMinPad == -1 || pad < cowMinPad {
			cowMinPad = pad
		}
	}

	// Cow block is 54 chars wide (speech bubble), so prefix = (120-54)/2 = 33
	expectedPrefix := (120 - 54) / 2
	if cowMinPad != expectedPrefix {
		t.Errorf("cow minimum padding: got %d, want %d", cowMinPad, expectedPrefix)
	}
}

func TestRenderCowsay_CenteredVertically(t *testing.T) {
	out := renderCowsay("*", 120, 40)
	lines := strings.Split(out, "\n")

	// Count leading empty lines
	topPad := 0
	for _, l := range lines {
		if strings.TrimSpace(stripANSI(l)) == "" {
			topPad++
		} else {
			break
		}
	}

	// Total content: 4 title lines + 1 blank + 8 cow lines = 13
	totalContent := len(strings.Split(loadingTitle, "\n")) + 1 + len(strings.Split(cowsayLoading, "\n"))
	expected := (40 - totalContent) / 2
	if topPad != expected {
		t.Errorf("vertical padding: got %d, want %d", topPad, expected)
	}
}

func TestRenderCowsay_TitleCenteredIndependently(t *testing.T) {
	// Title lines should each be centered based on their own width,
	// not the cow block width
	out := renderCowsay("*", 120, 30)
	lines := strings.Split(out, "\n")

	for _, l := range lines {
		plain := stripANSI(l)
		trimmed := strings.TrimSpace(plain)
		if trimmed == "" {
			continue
		}
		// Check if this is a title line (contains block characters)
		if strings.ContainsRune(trimmed, '\u2584') {
			pad := len(plain) - len(strings.TrimLeft(plain, " "))
			if pad <= 0 {
				t.Errorf("title line should have centering padding: %q", plain)
			}
			break
		}
	}
}

func TestRenderCowsay_SmallTerminalNoPanic(t *testing.T) {
	// Should not panic or produce negative padding
	out := renderCowsay("*", 10, 5)
	if len(out) == 0 {
		t.Error("expected non-empty output")
	}
}

// --- claudeWindowAndCmd tests ---

func newTestModel() Model {
	return Model{
		config: config.DefaultConfig(),
	}
}

func TestClaudeWindowAndCmd_FixCI(t *testing.T) {
	m := newTestModel()
	r := &PRRow{
		PR:     github.PR{URL: "https://github.com/org/repo/pull/1"},
		Action: github.ActionFixCI,
	}
	window, cmd := m.claudeWindowAndCmd(r, "")
	if window != "ci-fix" {
		t.Errorf("window: got %q, want %q", window, "ci-fix")
	}
	if !strings.Contains(cmd, "CI checks are failing") {
		t.Errorf("cmd should contain 'CI checks are failing': %s", cmd)
	}
	if !strings.Contains(cmd, "https://github.com/org/repo/pull/1") {
		t.Errorf("cmd should contain PR URL: %s", cmd)
	}
}

func TestClaudeWindowAndCmd_AddressFeedback(t *testing.T) {
	m := newTestModel()
	r := &PRRow{
		PR:     github.PR{URL: "https://github.com/org/repo/pull/2"},
		Action: github.ActionAddressFeedback,
	}
	window, cmd := m.claudeWindowAndCmd(r, "")
	if window != "feedback" {
		t.Errorf("window: got %q, want %q", window, "feedback")
	}
	if !strings.Contains(cmd, "review feedback") {
		t.Errorf("cmd should contain 'review feedback': %s", cmd)
	}
}

func TestClaudeWindowAndCmd_ResolveConflicts(t *testing.T) {
	m := newTestModel()
	r := &PRRow{
		PR:     github.PR{URL: "https://github.com/org/repo/pull/3"},
		Action: github.ActionResolveConflicts,
	}
	window, cmd := m.claudeWindowAndCmd(r, "")
	if window != "conflicts" {
		t.Errorf("window: got %q, want %q", window, "conflicts")
	}
	if !strings.Contains(cmd, "merge conflicts") {
		t.Errorf("cmd should contain 'merge conflicts': %s", cmd)
	}
}

func TestClaudeWindowAndCmd_Default(t *testing.T) {
	m := newTestModel()
	r := &PRRow{
		PR:     github.PR{URL: "https://github.com/org/repo/pull/4"},
		Action: github.ActionMerge,
	}
	window, cmd := m.claudeWindowAndCmd(r, "")
	if window != "claude" {
		t.Errorf("window: got %q, want %q", window, "claude")
	}
	if !strings.Contains(cmd, "Continue working") {
		t.Errorf("cmd should contain 'Continue working': %s", cmd)
	}
}

func TestClaudeWindowAndCmd_CustomPrompt(t *testing.T) {
	m := newTestModel()
	r := &PRRow{
		PR:     github.PR{URL: "https://github.com/org/repo/pull/5"},
		Action: github.ActionFixCI,
	}
	window, cmd := m.claudeWindowAndCmd(r, "do something custom")
	if window != "claude" {
		t.Errorf("window: got %q, want %q", window, "claude")
	}
	if !strings.Contains(cmd, "do something custom") {
		t.Errorf("cmd should contain custom prompt: %s", cmd)
	}
}

// stripANSI removes ANSI escape sequences for measuring visible width.
func stripANSI(s string) string {
	var b strings.Builder
	inEsc := false
	for _, r := range s {
		if r == '\033' {
			inEsc = true
			continue
		}
		if inEsc {
			if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') {
				inEsc = false
			}
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}
