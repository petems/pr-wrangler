package tui

import (
	"fmt"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/petems/pr-wrangler/internal/cache"
	"github.com/petems/pr-wrangler/internal/config"
	"github.com/petems/pr-wrangler/internal/github"
)

func sendKey(t *testing.T, m Model, key tea.KeyPressMsg) Model {
	t.Helper()
	updated, _ := m.Update(key)
	next, ok := updated.(Model)
	if !ok {
		t.Fatalf("Update returned %T, want Model", updated)
	}
	return next
}

func themePickerKeyPress() tea.KeyPressMsg {
	return tea.KeyPressMsg(tea.Key{Text: "t", Code: 't'})
}

func specialKeyPress(code rune) tea.KeyPressMsg {
	return tea.KeyPressMsg(tea.Key{Code: code})
}

func drainFetchCmd(t *testing.T, cmd tea.Cmd) {
	t.Helper()
	if cmd == nil {
		t.Fatal("fetch command was nil")
	}
	msg := cmd()
	started, ok := msg.(prsFetchStartedMsg)
	if !ok {
		t.Fatalf("fetch command returned %T, want prsFetchStartedMsg", msg)
	}
	timeout := time.After(2 * time.Second)
	for {
		select {
		case <-timeout:
			t.Fatal("timed out waiting for prsLoadedMsg")
		case msg, ok := <-started.progressCh:
			if !ok {
				t.Fatal("fetch channel closed before prsLoadedMsg")
			}
			if _, ok := msg.(prsLoadedMsg); ok {
				return
			}
		}
	}
}

func TestFetchPRsCmdUsesCacheUnlessRefreshBypasses(t *testing.T) {
	fetcher := &MockPRFetcher{PRs: []github.PR{{Number: 1, Title: "cached"}}}
	m := NewModel(fetcher, nil, nil, nil, config.DefaultConfig())

	drainFetchCmd(t, m.fetchPRsCmd(false))
	fetcher.PRs = []github.PR{{Number: 1, Title: "fresh"}}
	drainFetchCmd(t, m.fetchPRsCmd(false))

	if got := len(fetcher.Queries); got != 1 {
		t.Fatalf("non-refresh fetches should use cached result; fetch calls = %d, want 1", got)
	}

	drainFetchCmd(t, m.refreshCmd())
	if got := len(fetcher.Queries); got != 2 {
		t.Fatalf("refresh should bypass cache; fetch calls = %d, want 2", got)
	}
}

func TestModelDisableCacheSkipsDiskPreloadAndInMemoryCache(t *testing.T) {
	prCache := cache.NewCache(t.TempDir() + "/pr-cache.json")
	prCache.SetForQuery("author:@me is:open", []github.PR{{Number: 1, Title: "from disk"}}, nil)
	fetcher := &MockPRFetcher{PRs: []github.PR{{Number: 2, Title: "first fetch"}}}

	m := NewModelWithOptions(fetcher, nil, nil, prCache, config.DefaultConfig(), ModelOptions{DisableCache: true})
	if !m.loading {
		t.Fatal("disabled cache should skip disk preload and keep model loading")
	}
	if len(m.rows) != 0 {
		t.Fatalf("disabled cache should not populate rows from disk cache, got %d rows", len(m.rows))
	}

	drainFetchCmd(t, m.fetchPRsCmd(false))
	fetcher.PRs = []github.PR{{Number: 3, Title: "second fetch"}}
	drainFetchCmd(t, m.fetchPRsCmd(false))

	if got := len(fetcher.Queries); got != 2 {
		t.Fatalf("disabled cache should fetch every time; fetch calls = %d, want 2", got)
	}
}

func TestThemePicker_OpensAtCurrentScheme(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.ColorScheme = "solarized"
	m := NewModel(nil, nil, nil, nil, cfg)

	m = sendKey(t, m, themePickerKeyPress())

	if !m.showThemePicker {
		t.Fatal("expected picker to be open")
	}
	if got, want := ThemeNames[m.themePickerIndex], "solarized"; got != want {
		t.Errorf("highlighted theme: got %q, want %q", got, want)
	}
}

func TestThemePicker_DownClampsAtEnd(t *testing.T) {
	m := NewModel(nil, nil, nil, nil, config.DefaultConfig())
	m = sendKey(t, m, themePickerKeyPress())

	for i := 0; i < len(ThemeNames)+3; i++ {
		m = sendKey(t, m, specialKeyPress(tea.KeyDown))
	}
	if m.themePickerIndex != len(ThemeNames)-1 {
		t.Errorf("index after over-scroll down: got %d, want %d", m.themePickerIndex, len(ThemeNames)-1)
	}

	for i := 0; i < len(ThemeNames)+3; i++ {
		m = sendKey(t, m, specialKeyPress(tea.KeyUp))
	}
	if m.themePickerIndex != 0 {
		t.Errorf("index after over-scroll up: got %d, want 0", m.themePickerIndex)
	}
}

func TestThemePicker_EnterAppliesAndClosesPicker(t *testing.T) {
	m := NewModel(nil, nil, nil, nil, config.DefaultConfig())
	oldText := m.styles.TableText

	m = sendKey(t, m, themePickerKeyPress())
	m = sendKey(t, m, specialKeyPress(tea.KeyDown))
	m = sendKey(t, m, specialKeyPress(tea.KeyEnter))

	if m.showThemePicker {
		t.Fatal("expected picker to close after enter")
	}
	if got, want := m.config.ColorScheme, ThemeNames[1]; got != want {
		t.Errorf("config scheme: got %q, want %q", got, want)
	}
	if m.styles.TableText == oldText {
		t.Error("expected styles to update after applying theme")
	}
}

func openPickerWithSize(t *testing.T, width, height int) Model {
	t.Helper()
	m := NewModel(nil, nil, nil, nil, config.DefaultConfig())
	updated, _ := m.Update(tea.WindowSizeMsg{Width: width, Height: height})
	next, ok := updated.(Model)
	if !ok {
		t.Fatalf("Update returned %T, want Model", updated)
	}
	return sendKey(t, next, themePickerKeyPress())
}

func TestThemePicker_OverlayCentredAfterWindowSize(t *testing.T) {
	m := openPickerWithSize(t, 120, 30)

	out := m.View().Content
	if !strings.Contains(out, "Select Theme") {
		t.Fatal("rendered view should include picker title when picker is open")
	}
	if !strings.Contains(out, "╭") || !strings.Contains(out, "╮") {
		t.Fatal("rendered view should include rounded border characters for the modal frame")
	}

	// Modal placement: the picker frame should sit somewhere inside the
	// viewport with non-empty content both above and below it. An appended
	// picker (the no-window-size fallback path) would put the top frame
	// border at the very bottom of the output instead.
	lines := strings.Split(out, "\n")
	topBorderRow := -1
	for i, l := range lines {
		if strings.Contains(l, "╭") && strings.Contains(l, "╮") {
			topBorderRow = i
			break
		}
	}
	if topBorderRow <= 0 {
		t.Fatalf("expected modal top border below row 0, got row %d", topBorderRow)
	}
	if topBorderRow >= len(lines)-2 {
		t.Fatalf("modal top border at row %d in %d-line output looks appended, not overlaid", topBorderRow, len(lines))
	}
}

func TestThemePicker_OverlayDoesNotOverflowNarrowViewport(t *testing.T) {
	// Pick a viewport narrower than the picker's natural width so the
	// renderer must clamp the frame to fit.
	const viewportWidth = 40
	m := openPickerWithSize(t, viewportWidth, 20)

	// Inspect the picker frame directly (rather than the full composed
	// view, which can include dashboard content wider than the viewport).
	frame := m.renderThemePicker(viewportWidth)
	for i, line := range strings.Split(frame, "\n") {
		if w := lipgloss.Width(line); w > viewportWidth {
			t.Fatalf("picker line %d width %d exceeds viewport width %d: %q", i, w, viewportWidth, line)
		}
	}
}

func TestThemePicker_NavigationKeysAreModal(t *testing.T) {
	m := NewModel(nil, nil, nil, nil, config.DefaultConfig())
	m.rows = []PRRow{
		{PR: github.PR{Number: 1, Title: "first"}},
		{PR: github.PR{Number: 2, Title: "second"}},
		{PR: github.PR{Number: 3, Title: "third"}},
	}
	m.selected = 1

	m = sendKey(t, m, themePickerKeyPress())
	if !m.showThemePicker {
		t.Fatal("picker should be open")
	}
	startSelected := m.selected

	// Down/up keys must move the picker index, not the dashboard selection.
	m = sendKey(t, m, specialKeyPress(tea.KeyDown))
	m = sendKey(t, m, specialKeyPress(tea.KeyDown))
	m = sendKey(t, m, specialKeyPress(tea.KeyUp))
	if m.selected != startSelected {
		t.Errorf("dashboard selection changed while picker open: got %d, want %d", m.selected, startSelected)
	}
	if m.themePickerIndex == 0 {
		t.Error("picker index did not advance after Down keys")
	}

	// Dashboard shortcuts ('r' refresh, 'o' open) must be intercepted: the
	// picker stays open and the model returns no command (i.e. no refresh
	// or browser open fires while the modal is up).
	assertIntercepted := func(label, text string, code rune) {
		t.Helper()
		beforeIdx := m.themePickerIndex
		updated, cmd := m.Update(tea.KeyPressMsg(tea.Key{Text: text, Code: code}))
		next, ok := updated.(Model)
		if !ok {
			t.Fatalf("%s: Update returned %T, want Model", label, updated)
		}
		m = next
		if cmd != nil {
			t.Fatalf("%s should be intercepted while picker is open; got cmd %T", label, cmd)
		}
		if !m.showThemePicker {
			t.Errorf("%s should leave picker open", label)
		}
		if m.themePickerIndex != beforeIdx {
			t.Errorf("%s shifted picker index: got %d, want %d", label, m.themePickerIndex, beforeIdx)
		}
	}
	assertIntercepted("'r'", "r", 'r')
	assertIntercepted("'o'", "o", 'o')
}

func TestThemePicker_EscCancelsWithoutChange(t *testing.T) {
	m := NewModel(nil, nil, nil, nil, config.DefaultConfig())
	originalScheme := m.config.ColorScheme
	originalText := m.styles.TableText

	m = sendKey(t, m, themePickerKeyPress())
	m = sendKey(t, m, specialKeyPress(tea.KeyDown))
	m = sendKey(t, m, specialKeyPress(tea.KeyEsc))

	if m.showThemePicker {
		t.Fatal("expected picker to close after esc")
	}
	if m.config.ColorScheme != originalScheme {
		t.Errorf("config scheme changed on cancel: got %q, want %q", m.config.ColorScheme, originalScheme)
	}
	if m.styles.TableText != originalText {
		t.Error("styles should not change on cancel")
	}
}

func TestHelpOverlay_InterceptsDashboardKeys(t *testing.T) {
	m := NewModel(nil, nil, nil, nil, config.DefaultConfig())
	m.rows = []PRRow{
		{PR: github.PR{Number: 1, State: github.PRStateOpen}},
		{PR: github.PR{Number: 2, State: github.PRStateOpen}},
	}
	m.selected = 0
	m.showHelp = true

	m = sendKey(t, m, specialKeyPress(tea.KeyDown))

	if !m.showHelp {
		t.Fatal("expected help overlay to remain open")
	}
	if m.selected != 0 {
		t.Errorf("selected after down with help open: got %d, want 0", m.selected)
	}

	m = sendKey(t, m, specialKeyPress(tea.KeyEsc))

	if m.showHelp {
		t.Fatal("expected esc to close help overlay")
	}
}

func TestSelection_PageDownWithNoRowsStaysNonNegative(t *testing.T) {
	m := Model{}

	m = sendKey(t, m, specialKeyPress(tea.KeyPgDown))

	if m.selected != 0 {
		t.Errorf("selected after pgdown with no rows: got %d, want 0", m.selected)
	}
}

func TestSelection_LoadedRowsClampNegativeSelection(t *testing.T) {
	progressCh := make(chan tea.Msg)
	m := Model{selected: -1, progressCh: progressCh}

	updated, _ := m.Update(prsLoadedMsg{
		progressCh: progressCh,
		prs: []github.PR{
			{Number: 1, Title: "Open PR", State: github.PRStateOpen},
		},
	})
	next, ok := updated.(Model)
	if !ok {
		t.Fatalf("Update returned %T, want Model", updated)
	}

	if next.selected != 0 {
		t.Errorf("selected after loading rows: got %d, want 0", next.selected)
	}
}

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
		"https://github.com/octocat/hello-world/pull/123",
		"https://github.com/octocat/spoon-knife/pull/456/checks",
	} {
		if !strings.Contains(rendered, want) {
			t.Errorf("rendered table missing hyperlink URL %q", want)
		}
	}
}

func TestRenderTable_HyperlinksDisabled(t *testing.T) {
	defer withHyperlinks(t, false)()

	m := newRenderableTestModel(t)
	rendered := m.renderTable()

	if strings.Contains(rendered, "\x1b]8;;") {
		t.Errorf("rendered table emitted OSC 8 hyperlink escapes while disabled: %q", rendered)
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
	m.config.AgentCommands["followup"] = "custom followup {{pr_url}} {{pr_number}} {{repo_nwo}}"
	r := &PRRow{
		PR:     github.PR{URL: "https://github.com/org/repo/pull/4", Number: 4, RepoNameWithOwner: "org/repo"},
		Action: github.ActionMerge,
	}
	window, cmd := m.claudeWindowAndCmd(r, "")
	if window != "claude" {
		t.Errorf("window: got %q, want %q", window, "claude")
	}
	if !strings.Contains(cmd, "custom followup https://github.com/org/repo/pull/4 4 org/repo") {
		t.Errorf("cmd should contain configured followup command with replacements: %s", cmd)
	}
}

func TestClaudeWindowAndCmd_ActionNoneUsesConfiguredFollowup(t *testing.T) {
	m := newTestModel()
	m.config.AgentCommands["followup"] = "custom followup {{pr_url}}"
	r := &PRRow{
		PR:     github.PR{URL: "https://github.com/org/repo/pull/6"},
		Action: github.ActionNone,
	}
	window, cmd := m.claudeWindowAndCmd(r, "")
	if window != "claude" {
		t.Errorf("window: got %q, want %q", window, "claude")
	}
	if !strings.Contains(cmd, "custom followup https://github.com/org/repo/pull/6") {
		t.Errorf("cmd should contain configured followup command with replacements: %s", cmd)
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

// --- demo mode ---

// runCmdAndDrain executes a tea.Cmd and, if the resulting message is a
// tea.BatchMsg, recursively runs every nested Cmd. Without this, a single
// cmd() call returns the BatchMsg envelope but never executes its members,
// which would let a nil-deref panic in a sub-Cmd slip past
// TestNewDemoModel_InitAndKeypressesDoNotPanic.
func runCmdAndDrain(cmd tea.Cmd) {
	if cmd == nil {
		return
	}
	msg := cmd()
	if batch, ok := msg.(tea.BatchMsg); ok {
		for _, nested := range batch {
			runCmdAndDrain(nested)
		}
	}
}

// TestNewDemoModel_InitAndKeypressesDoNotPanic is the regression test for the
// nil-pointer panic in fetchPRsCmd when Init() ran refreshCmd() against the
// demo model's nil *github.GHClient. The earlier test suite called Update()
// on hand-built Model literals but never exercised the demo construction
// path, so the panic only surfaced at program start.
//
// This test runs every keypress that previously dispatched a Cmd against
// ghClient/sessionMgr/sessionStore (refresh, session switch, navigation),
// then drains every returned Cmd (including BatchMsg members) so any nil
// deref reintroduced in that path triggers a goroutine panic the runtime
// surfaces as a test failure.
func TestNewDemoModel_InitAndKeypressesDoNotPanic(t *testing.T) {
	m := NewDemoModel(config.DefaultConfig(), 10)
	// Defence-in-depth: even if the demo guards on 'o'/'a' regress, this
	// stub keeps the test from shelling out to the real OS browser.
	m.browserOpener = func(string) {}

	if m.ghClient != nil || m.sessionMgr != nil || m.sessionStore != nil {
		t.Fatalf("demo model should leave clients nil, got ghClient=%v sessionMgr=%v sessionStore=%v",
			m.ghClient, m.sessionMgr, m.sessionStore)
	}
	if !m.demoMode {
		t.Fatal("demo model should set demoMode=true")
	}
	if len(m.rows) == 0 {
		t.Fatal("demo model should have populated rows")
	}

	// Init() must not return any Cmd that, when executed (or whose batched
	// sub-Cmds are executed), hits the nil ghClient/sessionMgr.
	runCmdAndDrain(m.Init())

	// These keypresses previously dispatched commands that dereferenced
	// nil clients. With the demo guards in place each must be a safe no-op
	// or surface a notification. Construct the Enter key explicitly via
	// tea.KeyEnter — otherwise tea.Key{Code: rune("enter"[0])} would send
	// 'e' and the actual enter handler would never be exercised.
	presses := []tea.KeyPressMsg{
		tea.KeyPressMsg(tea.Key{Text: "r", Code: 'r'}),
		tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}),
		tea.KeyPressMsg(tea.Key{Text: "c", Code: 'c'}),
		tea.KeyPressMsg(tea.Key{Text: "j", Code: 'j'}),
		tea.KeyPressMsg(tea.Key{Text: "k", Code: 'k'}),
		tea.KeyPressMsg(tea.Key{Text: "?", Code: '?'}),
		tea.KeyPressMsg(tea.Key{Text: "a", Code: 'a'}),
		tea.KeyPressMsg(tea.Key{Text: "o", Code: 'o'}),
	}
	for _, press := range presses {
		updated, cmd := m.Update(press)
		next, ok := updated.(Model)
		if !ok {
			t.Fatalf("press %+v: Update returned %T, want Model", press, updated)
		}
		m = next
		runCmdAndDrain(cmd)
	}
}

func TestNewDemoModel_CountControlsRenderedRows(t *testing.T) {
	tests := []int{0, 1, 3, 4, 7, 8, 10, 50}

	for _, count := range tests {
		m := NewDemoModel(config.DefaultConfig(), count)
		if len(m.rows) != count {
			t.Fatalf("count %d: rows=%d, want %d", count, len(m.rows), count)
		}
		if len(m.allRows) != count {
			t.Fatalf("count %d: allRows=%d, want %d", count, len(m.allRows), count)
		}
		for _, entry := range m.samlErrors {
			if entry.Index >= count {
				t.Fatalf("count %d: kept out-of-range SAML index %d", count, entry.Index)
			}
		}
	}
}

// TestDemoModel_RefreshIsNoOp guards against accidentally re-enabling the
// network refresh path in demo mode. Pressing 'r' must not flip loading=true
// (which would hide the populated rows behind the cowsay loading screen) or
// dispatch a fetch command.
func TestDemoModel_RefreshIsNoOp(t *testing.T) {
	m := NewDemoModel(config.DefaultConfig(), 10)
	rowsBefore := len(m.rows)

	updated, cmd := m.Update(tea.KeyPressMsg(tea.Key{Text: "r", Code: 'r'}))
	next, ok := updated.(Model)
	if !ok {
		t.Fatalf("Update returned %T, want Model", updated)
	}

	if next.loading {
		t.Error("demo refresh must not flip loading=true")
	}
	if len(next.rows) != rowsBefore {
		t.Errorf("rows changed unexpectedly: before=%d after=%d", rowsBefore, len(next.rows))
	}
	if next.notification == "" {
		t.Error("demo refresh should set a notification explaining it's disabled")
	}
	if cmd != nil {
		t.Fatalf("demo refresh must return a nil Cmd; got %T", cmd)
	}
}

// TestDemoModel_OpenPRIsNoOp guards against the demo TUI launching a real
// browser to mock PR URLs (which 404 for anyone except the repo owner).
func TestDemoModel_OpenPRIsNoOp(t *testing.T) {
	m := NewDemoModel(config.DefaultConfig(), 10)
	called := false
	m.browserOpener = func(string) { called = true }

	updated, cmd := m.Update(tea.KeyPressMsg(tea.Key{Text: "o", Code: 'o'}))
	next, ok := updated.(Model)
	if !ok {
		t.Fatalf("Update returned %T, want Model", updated)
	}
	runCmdAndDrain(cmd)

	if called {
		t.Error("demo 'o' must not invoke openBrowser")
	}
	if next.notification == "" {
		t.Error("demo 'o' should set a notification explaining it's disabled")
	}
	if cmd != nil {
		t.Errorf("demo 'o' must return nil Cmd; got %T", cmd)
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

func viewPickerKeyPress() tea.KeyPressMsg {
	return tea.KeyPressMsg(tea.Key{Text: "v", Code: 'v'})
}

func openViewPickerWithSize(t *testing.T, width, height int) Model {
	t.Helper()
	m := newMultiViewModel(t, 3)
	updated, _ := m.Update(tea.WindowSizeMsg{Width: width, Height: height})
	next, ok := updated.(Model)
	if !ok {
		t.Fatalf("Update returned %T, want Model", updated)
	}
	return sendKey(t, next, viewPickerKeyPress())
}

func newMultiViewModel(t *testing.T, n int) Model {
	t.Helper()
	cfg := config.DefaultConfig()
	cfg.Views = make([]config.View, n)
	for i := 0; i < n; i++ {
		cfg.Views[i] = config.View{Name: fmt.Sprintf("view-%d", i), Query: fmt.Sprintf("q-%d", i)}
	}
	return NewModel(nil, nil, nil, nil, cfg)
}

func TestViewPicker_OpensAtActiveView(t *testing.T) {
	m := newMultiViewModel(t, 3)
	m.activeViewIndex = 2

	m = sendKey(t, m, viewPickerKeyPress())

	if !m.showViewPicker {
		t.Fatal("expected picker to be open")
	}
	if m.viewPickerIndex != 2 {
		t.Errorf("viewPickerIndex: got %d, want 2", m.viewPickerIndex)
	}
}

func TestViewPicker_SingleViewIsNoop(t *testing.T) {
	m := newMultiViewModel(t, 1)

	m = sendKey(t, m, viewPickerKeyPress())

	if m.showViewPicker {
		t.Error("picker should not open with only one view")
	}
}

func TestViewPicker_NavigationBounded(t *testing.T) {
	m := newMultiViewModel(t, 3)
	m = sendKey(t, m, viewPickerKeyPress())

	m = sendKey(t, m, specialKeyPress(tea.KeyUp))
	if m.viewPickerIndex != 0 {
		t.Errorf("up at index 0 should be a no-op, got %d", m.viewPickerIndex)
	}

	for i := 0; i < 5; i++ {
		m = sendKey(t, m, specialKeyPress(tea.KeyDown))
	}
	if m.viewPickerIndex != 2 {
		t.Errorf("after over-scroll down: got %d, want 2", m.viewPickerIndex)
	}
}

func TestViewPicker_OverlayCentredAfterWindowSize(t *testing.T) {
	m := openViewPickerWithSize(t, 120, 30)

	out := m.View().Content
	if !strings.Contains(out, "Select View") {
		t.Fatal("rendered view should include picker title when picker is open")
	}
	if !strings.Contains(out, "╭") || !strings.Contains(out, "╮") {
		t.Fatal("rendered view should include rounded border characters for the modal frame")
	}

	lines := strings.Split(out, "\n")
	topBorderRow := -1
	for i, l := range lines {
		if strings.Contains(l, "╭") && strings.Contains(l, "╮") {
			topBorderRow = i
			break
		}
	}
	if topBorderRow <= 0 {
		t.Fatalf("expected modal top border below row 0, got row %d", topBorderRow)
	}
	if topBorderRow >= len(lines)-2 {
		t.Fatalf("modal top border at row %d in %d-line output looks appended, not overlaid", topBorderRow, len(lines))
	}
}

func TestViewPicker_OverlayDoesNotOverflowNarrowViewport(t *testing.T) {
	const viewportWidth = 40
	m := openViewPickerWithSize(t, viewportWidth, 20)

	frame := m.renderViewPicker(viewportWidth)
	for i, line := range strings.Split(frame, "\n") {
		if w := lipgloss.Width(line); w > viewportWidth {
			t.Fatalf("picker line %d width %d exceeds viewport width %d: %q", i, w, viewportWidth, line)
		}
	}
}

func TestViewPicker_EnterCommitsAndFetches(t *testing.T) {
	m := newMultiViewModel(t, 3)
	m.activeViewIndex = 0
	m.selected = 5
	m.allRows = make([]PRRow, 10)
	m.rows = m.allRows
	m.showViewPicker = true
	m.viewPickerIndex = 2

	updated, cmd := m.Update(specialKeyPress(tea.KeyEnter))
	next, ok := updated.(Model)
	if !ok {
		t.Fatalf("Update returned %T, want Model", updated)
	}

	if next.showViewPicker {
		t.Error("picker should be closed after enter")
	}
	if next.activeViewIndex != 2 {
		t.Errorf("activeViewIndex: got %d, want 2", next.activeViewIndex)
	}
	if next.selected != 0 {
		t.Errorf("selected: got %d, want 0", next.selected)
	}
	if !next.loading {
		t.Error("expected loading=true after view switch")
	}
	if cmd == nil {
		t.Fatal("expected fetch cmd, got nil")
	}
}

func TestViewPicker_EnterOnActiveViewIsNoFetch(t *testing.T) {
	m := newMultiViewModel(t, 3)
	m.activeViewIndex = 1
	m.showViewPicker = true
	m.viewPickerIndex = 1

	updated, cmd := m.Update(specialKeyPress(tea.KeyEnter))
	next, ok := updated.(Model)
	if !ok {
		t.Fatalf("Update returned %T, want Model", updated)
	}

	if next.showViewPicker {
		t.Error("picker should be closed")
	}
	if cmd != nil {
		t.Error("expected no fetch when re-selecting active view")
	}
}

func TestViewPicker_EscCancels(t *testing.T) {
	m := newMultiViewModel(t, 3)
	m.activeViewIndex = 0
	m.showViewPicker = true
	m.viewPickerIndex = 2

	m = sendKey(t, m, specialKeyPress(tea.KeyEsc))

	if m.showViewPicker {
		t.Error("picker should be closed after esc")
	}
	if m.activeViewIndex != 0 {
		t.Errorf("activeViewIndex changed to %d after cancel", m.activeViewIndex)
	}
}

func TestViewPicker_DemoModeAppliesViewWithoutFetch(t *testing.T) {
	m := NewDemoModel(config.DefaultConfig(), 5)
	if len(m.config.Views) < 2 {
		t.Fatalf("demo model should seed multiple views, got %d", len(m.config.Views))
	}
	m.activeViewIndex = 0
	m.showViewPicker = true
	m.viewPickerIndex = 1

	updated, cmd := m.Update(specialKeyPress(tea.KeyEnter))
	next, ok := updated.(Model)
	if !ok {
		t.Fatalf("Update returned %T, want Model", updated)
	}
	if next.activeViewIndex != 1 {
		t.Errorf("activeViewIndex: got %d, want 1", next.activeViewIndex)
	}
	if cmd != nil {
		t.Error("demo mode should not return a fetch cmd from the view picker")
	}
	if next.notification == "" {
		t.Error("expected a demo-mode notification after applying view")
	}
}

func TestViewPicker_EnterHydratesFromDiskCache(t *testing.T) {
	// Switching views should consult the on-disk cache for the new view's
	// query and render cached rows instead of flashing the loading screen.
	prCache := cache.NewCache(t.TempDir() + "/pr-cache.json")
	prCache.SetForQuery("q-2",
		[]github.PR{{Number: 99, Title: "cached for q-2"}},
		nil,
	)

	cfg := config.DefaultConfig()
	cfg.Views = []config.View{
		{Name: "view-0", Query: "q-0", Default: true},
		{Name: "view-1", Query: "q-1"},
		{Name: "view-2", Query: "q-2"},
	}

	m := NewModel(&MockPRFetcher{}, nil, nil, prCache, cfg)
	m.activeViewIndex = 0
	m.showViewPicker = true
	m.viewPickerIndex = 2

	updated, cmd := m.Update(specialKeyPress(tea.KeyEnter))
	next, ok := updated.(Model)
	if !ok {
		t.Fatalf("Update returned %T, want Model", updated)
	}
	if next.loading {
		t.Error("expected loading=false after disk-cache hit")
	}
	if !next.refreshing {
		t.Error("expected refreshing=true after disk-cache hit")
	}
	if len(next.rows) != 1 || next.rows[0].PR.Number != 99 {
		t.Errorf("expected rows hydrated from disk cache, got %+v", next.rows)
	}
	if cmd == nil {
		t.Fatal("expected background fetch cmd")
	}
}

func TestViewPicker_EnterMissingDiskCacheShowsLoading(t *testing.T) {
	// When the disk cache has no entry for the new view, we should fall back
	// to the loading screen rather than holding stale rows from the previous
	// view.
	prCache := cache.NewCache(t.TempDir() + "/pr-cache.json")
	cfg := config.DefaultConfig()
	cfg.Views = []config.View{
		{Name: "view-0", Query: "q-0", Default: true},
		{Name: "view-1", Query: "q-1"},
	}

	m := NewModel(&MockPRFetcher{}, nil, nil, prCache, cfg)
	m.activeViewIndex = 0
	m.allRows = []PRRow{{}, {}}
	m.rows = m.allRows
	m.showViewPicker = true
	m.viewPickerIndex = 1

	updated, _ := m.Update(specialKeyPress(tea.KeyEnter))
	next, ok := updated.(Model)
	if !ok {
		t.Fatalf("Update returned %T, want Model", updated)
	}
	if !next.loading {
		t.Error("expected loading=true on disk-cache miss")
	}
	if len(next.allRows) != 0 {
		t.Errorf("expected rows cleared on disk-cache miss, got %d", len(next.allRows))
	}
}

func TestViewPicker_EnterInvalidatesInFlightFetch(t *testing.T) {
	// Regression: switching views while a fetch is in flight must drop the
	// previous fetch's progressCh so a late prsLoadedMsg can't be cached
	// under the newly-selected view's query.
	m := newMultiViewModel(t, 3)
	m.activeViewIndex = 0

	staleCh := make(chan tea.Msg, 1)
	var staleRecv <-chan tea.Msg = staleCh
	m.progressCh = staleRecv
	m.progressDone = 4
	m.progressTotal = 10

	m.showViewPicker = true
	m.viewPickerIndex = 2

	updated, cmd := m.Update(specialKeyPress(tea.KeyEnter))
	next, ok := updated.(Model)
	if !ok {
		t.Fatalf("Update returned %T, want Model", updated)
	}
	if next.progressCh != nil {
		t.Error("expected progressCh to be cleared after view switch")
	}
	if next.progressDone != 0 || next.progressTotal != 0 {
		t.Errorf("expected progress counters reset, got done=%d total=%d", next.progressDone, next.progressTotal)
	}
	if cmd == nil {
		t.Fatal("expected fetch cmd, got nil")
	}
}

func TestViewPicker_OpensClosingOtherOverlays(t *testing.T) {
	m := newMultiViewModel(t, 3)
	m.showHelp = true
	m.showThemePicker = true

	m = sendKey(t, m, viewPickerKeyPress())

	// Pre-existing overlays should be closed; either the early-return guard
	// above blocked 'v' (in which case the picker won't have opened), or 'v'
	// reached the dispatch block and we closed them defensively. Either way,
	// help/theme picker must not stay open alongside the view picker.
	if m.showViewPicker && (m.showHelp || m.showThemePicker) {
		t.Errorf("overlays should be mutually exclusive: viewPicker=%v help=%v theme=%v",
			m.showViewPicker, m.showHelp, m.showThemePicker)
	}
}

func TestConfiguredQuery_UsesActiveViewIndex(t *testing.T) {
	m := newMultiViewModel(t, 3)

	m.activeViewIndex = 0
	if got, want := m.configuredQuery(), "q-0"; got != want {
		t.Errorf("active=0: got %q, want %q", got, want)
	}
	m.activeViewIndex = 2
	if got, want := m.configuredQuery(), "q-2"; got != want {
		t.Errorf("active=2: got %q, want %q", got, want)
	}
	m.activeViewIndex = 99
	if got := m.configuredQuery(); got != "" {
		t.Errorf("out-of-range index should return \"\", got %q", got)
	}
	m.activeViewIndex = -1
	if got := m.configuredQuery(); got != "" {
		t.Errorf("negative index should return \"\", got %q", got)
	}
}
