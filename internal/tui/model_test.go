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

	rows := buildRows(prs, nil)
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

	rows := buildRows(prs, nil)
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

	rows := buildRows(prs, nil)
	if len(rows) != 0 {
		t.Fatalf("expected 0 rows, got %d", len(rows))
	}
}

func TestBuildRows_Empty(t *testing.T) {
	rows := buildRows(nil, nil)
	if len(rows) != 0 {
		t.Fatalf("expected 0 rows, got %d", len(rows))
	}
}

func TestBuildRows_WithSAMLErrors(t *testing.T) {
	// Original positions: 0=PR#1, 1=SAML#30, 2=PR#2. Expected output order
	// after interleaving: [PR#1, SAML#30, PR#2].
	prs := []github.PR{
		{Number: 1, Title: "Open PR 1", State: github.PRStateOpen, RepoNameWithOwner: "org/repo1"},
		{Number: 2, Title: "Open PR 2", State: github.PRStateOpen, RepoNameWithOwner: "org/repo3"},
	}

	samlErrors := []github.SAMLErrorEntry{
		{
			Index:             1,
			RepoNameWithOwner: "org/repo2",
			PRNumber:          30,
			Err: &github.SAMLAuthError{
				Message: "Resource protected by organization SAML",
				AuthURL: "https://github.com/enterprises/example-org/sso?authorization_request=ABC123",
			},
		},
	}

	rows := buildRows(prs, samlErrors)
	if len(rows) != 3 {
		t.Fatalf("expected 3 rows, got %d", len(rows))
	}

	if rows[0].PR.Number != 1 {
		t.Errorf("rows[0]: expected PR#1, got #%d", rows[0].PR.Number)
	}
	if rows[1].Status != github.PRStatusSAMLRequired || rows[1].PR.Number != 30 {
		t.Errorf("rows[1]: expected SAML PR#30, got status=%q number=%d", rows[1].Status, rows[1].PR.Number)
	}
	if rows[2].PR.Number != 2 {
		t.Errorf("rows[2]: expected PR#2, got #%d", rows[2].PR.Number)
	}

	saml := rows[1]
	if saml.Action != github.ActionAuthorizeSAML {
		t.Errorf("SAML row action: got %q, want %q", saml.Action, github.ActionAuthorizeSAML)
	}
	if saml.SAMLError == nil || saml.SAMLError.AuthURL != "https://github.com/enterprises/example-org/sso?authorization_request=ABC123" {
		t.Errorf("SAML row AuthURL: got %+v", saml.SAMLError)
	}
}

func TestBuildRows_SAMLWithoutAuthURL_ActionNone(t *testing.T) {
	// When parseSAMLError can't extract a URL, the row should not show
	// the "Authorize SAML" affordance — pressing 'a' would no-op.
	prs := []github.PR{}
	samlErrors := []github.SAMLErrorEntry{
		{
			Index:             0,
			RepoNameWithOwner: "org/repo",
			PRNumber:          7,
			Err:               &github.SAMLAuthError{Message: "SAML"}, // AuthURL empty
		},
	}

	rows := buildRows(prs, samlErrors)
	if len(rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(rows))
	}
	if rows[0].Action != github.ActionNone {
		t.Errorf("expected ActionNone for SAML row without URL, got %q", rows[0].Action)
	}
}

// --- table render / OSC 8 leakage regression ---

// countOSC8Tokens returns (openers, closers) found in s. A closer is one of
// the fixed-form sequences "\x1b]8;;\x1b\\" (ST-terminated) or "\x1b]8;;\x07"
// (BEL-terminated). An opener is "\x1b]8;;<url>" followed by a terminator
// where <url> is non-empty. If upstream truncation ever cuts a cell
// mid-escape, the closer goes missing and openers > closers, which is the
// regression class introduced by bubble-table v0.19.2.
func countOSC8Tokens(s string) (openers, closers int) {
	stClose := "\x1b]8;;\x1b\\"
	belClose := "\x1b]8;;\x07"
	closers = strings.Count(s, stClose) + strings.Count(s, belClose)
	openers = strings.Count(s, "\x1b]8;;") - closers
	return openers, closers
}

func newRenderableTestModel(t *testing.T) Model {
	t.Helper()
	prs := []github.PR{
		{
			Number:            123,
			Title:             "Fix the thing that has a fairly long title to force truncation",
			URL:               "https://github.com/octocat/hello-world/pull/123",
			RepoNameWithOwner: "octocat/hello-world",
			State:             github.PRStateOpen,
		},
		{
			Number:            456,
			Title:             "Another PR with stuff",
			URL:               "https://github.com/octocat/spoon-knife/pull/456",
			RepoNameWithOwner: "octocat/spoon-knife",
			State:             github.PRStateOpen,
		},
	}
	m := Model{
		styles: NewStyles("default"),
		width:  140,
		height: 40,
		rows:   buildRows(prs, nil),
	}
	return m
}

// TestRenderTable_NoOSC8Leakage is the regression test for the PR #15 bug:
// cells in the PR table must never leave OSC 8 hyperlinks unterminated.
// lipgloss/table uses charmbracelet/x/ansi for width math, so its truncation
// path is OSC 8-aware. If anyone reverts to a renderer that isn't, this
// test catches it.
func TestRenderTable_NoOSC8Leakage(t *testing.T) {
	defer withHyperlinks(t, true)()

	m := newRenderableTestModel(t)
	rendered := m.renderTable()

	openers, closers := countOSC8Tokens(rendered)
	if openers != closers {
		t.Errorf("OSC 8 leakage in rendered table: %d openers, %d closers (must be equal)\nrendered=%q", openers, closers, rendered)
	}
	// Sanity: we should actually be emitting hyperlinks (else the test
	// doesn't exercise the wrapping path).
	if openers == 0 {
		t.Errorf("expected at least one OSC 8 hyperlink in rendered output, got 0")
	}
}

// TestRenderTable_HyperlinksPresent verifies that per-cell hyperlinks are
// being wrapped at all — guards against a silent regression where Style
// .Hyperlink() stops being applied.
func TestRenderTable_HyperlinksPresent(t *testing.T) {
	defer withHyperlinks(t, true)()

	m := newRenderableTestModel(t)
	rendered := m.renderTable()

	for _, want := range []string{
		"https://github.com/octocat/hello-world",
		"https://github.com/octocat/hello-world/pull/123",
		"https://github.com/octocat/spoon-knife/pull/456/checks",
	} {
		if !strings.Contains(rendered, want) {
			t.Errorf("rendered table missing hyperlink URL %q", want)
		}
	}
}

// --- renderCowsay tests ---

func TestRenderCowsay_ContainsCowArt(t *testing.T) {
	out := renderCowsay(NewStyles("default"), "*", 120, 30)
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
	out := renderCowsay(NewStyles("default"), "*", 120, 30)
	// The title contains "PR WRANGLER" in block letters; check for distinctive fragments
	if !strings.Contains(out, "\u2597\u2584\u2584\u2596") {
		t.Error("output missing title banner")
	}
}

func TestRenderCowsay_TitleHasNoBlankLineGaps(t *testing.T) {
	out := renderCowsay(NewStyles("default"), "*", 120, 30)
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
	out := renderCowsay(NewStyles("default"), "*", 120, 30)
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
	out := renderCowsay(NewStyles("default"), "XYZ", 120, 30)
	if !strings.Contains(out, "XYZ") {
		t.Error("spinner string not found in output")
	}
}

func TestRenderCowsay_CowLinesHaveUniformPrefix(t *testing.T) {
	out := renderCowsay(NewStyles("default"), "*", 120, 30)
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
	out := renderCowsay(NewStyles("default"), "*", 120, 40)
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
	out := renderCowsay(NewStyles("default"), "*", 120, 30)
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
	out := renderCowsay(NewStyles("default"), "*", 10, 5)
	if len(out) == 0 {
		t.Error("expected non-empty output")
	}
}

// --- claudeWindowAndCmd tests ---

func newTestModel() Model {
	return Model{
		ghClient: github.NewGHClientWithToken("test-token"),
		config:   config.DefaultConfig(),
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

// --- titleColumnWidth / tablePageSize ---

func TestTitleColumnWidth(t *testing.T) {
	cases := []struct {
		name  string
		width int
		want  int
	}{
		{"zero falls back to minimum", 0, minTitleColumnWidth},
		{"clamped at minimum when narrow", 80, minTitleColumnWidth},
		{"scales with width", 200, 200 - nonTitleColumnsWidth},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := Model{width: tc.width}
			if got := m.titleColumnWidth(); got != tc.want {
				t.Errorf("titleColumnWidth(width=%d) = %d, want %d", tc.width, got, tc.want)
			}
		})
	}
}

func TestTablePageSize(t *testing.T) {
	cases := []struct {
		name   string
		height int
		want   int
	}{
		{"zero falls back to minimum", 0, minPageSize},
		{"short terminal falls back to minimum", tableChromeLines + minPageSize, minPageSize},
		{"normal terminal scales with height", 30, 30 - tableChromeLines},
		{"tall terminal scales with height", 60, 60 - tableChromeLines},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := Model{height: tc.height}
			if got := m.tablePageSize(); got != tc.want {
				t.Errorf("tablePageSize(height=%d) = %d, want %d", tc.height, got, tc.want)
			}
		})
	}
}

// stripANSI removes ANSI escape sequences for measuring visible width.
// Handles both SGR-style CSI sequences (ESC [ ... <letter>) and OSC sequences
// such as OSC 8 hyperlinks (ESC ] ... ESC \ or BEL terminator). The latter is
// required so any OSC 8 wrapping introduced by Link() in this package doesn't
// confuse downstream tests that compare against plain text.
func stripANSI(s string) string {
	const (
		stateText = iota
		stateEsc  // saw ESC, deciding what's next
		stateCSI  // inside ESC [ ... <terminator>
		stateOSC  // inside ESC ] ... ESC \  (or BEL)
		stateOSCEsc
	)

	var b strings.Builder
	state := stateText
	for _, r := range s {
		switch state {
		case stateText:
			if r == '\033' {
				state = stateEsc
				continue
			}
			b.WriteRune(r)
		case stateEsc:
			switch r {
			case '[':
				state = stateCSI
			case ']':
				state = stateOSC
			case '\\':
				// Lone ST after a malformed sequence; bail out to text.
				state = stateText
			default:
				// ESC <single byte> (e.g. ESC =): consume one byte and resume.
				state = stateText
			}
		case stateCSI:
			if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') {
				state = stateText
			}
		case stateOSC:
			switch r {
			case '\007': // BEL terminator
				state = stateText
			case '\033':
				state = stateOSCEsc
			}
		case stateOSCEsc:
			// We just saw ESC inside an OSC: the next rune should be '\' (ST).
			// Either way, return to text.
			state = stateText
		}
	}
	return b.String()
}
