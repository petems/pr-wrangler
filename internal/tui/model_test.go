package tui

import (
	"context"
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/petems/pr-wrangler/internal/config"
	"github.com/petems/pr-wrangler/internal/github"
	"github.com/petems/pr-wrangler/internal/tmux"
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

func TestThemePicker_OpensAtCurrentScheme(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.ColorScheme = "solarized"
	m := NewModel(nil, nil, nil, cfg)

	m = sendKey(t, m, themePickerKeyPress())

	if !m.showThemePicker {
		t.Fatal("expected picker to be open")
	}
	if got, want := ThemeNames[m.themePickerIndex], "solarized"; got != want {
		t.Errorf("highlighted theme: got %q, want %q", got, want)
	}
}

func TestThemePicker_DownClampsAtEnd(t *testing.T) {
	m := NewModel(nil, nil, nil, config.DefaultConfig())
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
	m := NewModel(nil, nil, nil, config.DefaultConfig())
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

func TestThemePicker_EscCancelsWithoutChange(t *testing.T) {
	m := NewModel(nil, nil, nil, config.DefaultConfig())
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
		"https://github.com/octocat/hello-world",
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

// TestTablePageSize_ReservesRowsForTmuxBanner ensures the banner's footer
// rows are subtracted from the available table height; otherwise the table
// would overflow the screen when the banner is visible.
func TestTablePageSize_ReservesRowsForTmuxBanner(t *testing.T) {
	t.Setenv("TMUX", "1") // make InsideTmux() return true

	cfg := config.DefaultConfig()
	cfg.ShowTmuxBanner = true
	sessionMgr := tmux.NewSessionManager(nil, "/tmp", "/tmp")

	m := NewModel(nil, sessionMgr, nil, cfg)
	m.height = 60

	want := 60 - (tableChromeLines + tmuxBannerLines)
	if got := m.tablePageSize(); got != want {
		t.Errorf("tablePageSize with banner = %d, want %d", got, want)
	}
}

func TestShouldShowTmuxBanner(t *testing.T) {
	sessionMgr := tmux.NewSessionManager(nil, "/tmp", "/tmp")

	cases := []struct {
		name       string
		tmuxEnv    string
		sessionMgr *tmux.SessionManager
		enabled    bool
		want       bool
	}{
		{"inside tmux and enabled shows banner", "1", sessionMgr, true, true},
		{"inside tmux but disabled hides banner", "1", sessionMgr, false, false},
		{"outside tmux hides banner even if enabled", "", sessionMgr, true, false},
		{"nil sessionMgr hides banner", "1", nil, true, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("TMUX", tc.tmuxEnv)
			cfg := config.DefaultConfig()
			cfg.ShowTmuxBanner = tc.enabled
			m := Model{config: cfg, sessionMgr: tc.sessionMgr}
			if got := m.shouldShowTmuxBanner(); got != tc.want {
				t.Errorf("shouldShowTmuxBanner = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestRenderTmuxBanner_IncludesLabelDetachHintAndPRInfo(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Views = []config.View{{Name: "Team Triage", Query: "team:foo"}}

	m := Model{
		config: cfg,
		styles: NewStyles("default"),
		width:  120,
		rows: []PRRow{{
			PR: github.PR{
				Number:            42,
				RepoNameWithOwner: "petems/pr-wrangler",
			},
		}},
		selected: 0,
	}

	out := stripANSI(m.renderTmuxBanner())

	for _, want := range []string{"PR Wrangler", "Team Triage", "pr-wrangler#42", "Ctrl+B D to detach"} {
		if !strings.Contains(out, want) {
			t.Errorf("banner missing %q\ngot:\n%s", want, out)
		}
	}
}

func TestRenderTmuxBanner_OmitsPRInfoWhenNoRows(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Views = []config.View{{Name: "Empty", Query: "x"}}

	m := Model{
		config: cfg,
		styles: NewStyles("default"),
		width:  120,
	}

	out := stripANSI(m.renderTmuxBanner())

	if !strings.Contains(out, "Ctrl+B D to detach") {
		t.Errorf("banner missing detach hint:\n%s", out)
	}
	if strings.Contains(out, "#") {
		t.Errorf("banner should not include PR info when no rows:\n%s", out)
	}
}

// TestRenderTmuxBanner_BoundsBothLinesToPaneWidth guards against terminal
// wrapping when the banner is rendered into a narrow pane or with long
// view/repo names. If either line exceeds the configured width, the table
// page-size reservation (tmuxBannerLines) is wrong by however many extra
// rows the wrap consumes, so the PR table can scroll off-screen.
func TestRenderTmuxBanner_BoundsBothLinesToPaneWidth(t *testing.T) {
	cases := []struct {
		name     string
		width    int
		viewName string
		repo     string
		prNumber int
	}{
		{"narrow pane truncates short content", 20, "My PRs", "petems/pr-wrangler", 42},
		{"medium pane with long view name", 60, strings.Repeat("VeryLongViewName-", 5), "petems/pr-wrangler", 999999},
		{"medium pane with long repo name", 60, "My PRs", "org/" + strings.Repeat("very-long-repo-name-", 4), 1},
		{"wide pane fits everything", 200, "My PRs", "petems/pr-wrangler", 42},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := config.DefaultConfig()
			cfg.Views = []config.View{{Name: tc.viewName, Query: "x"}}

			m := Model{
				config: cfg,
				styles: NewStyles("default"),
				width:  tc.width,
				rows: []PRRow{{
					PR: github.PR{Number: tc.prNumber, RepoNameWithOwner: tc.repo},
				}},
				selected: 0,
			}

			out := stripANSI(m.renderTmuxBanner())
			lines := strings.Split(out, "\n")
			if len(lines) != 2 {
				t.Fatalf("banner must render exactly 2 lines, got %d:\n%s", len(lines), out)
			}
			for i, line := range lines {
				if w := ansi.StringWidth(line); w > tc.width {
					t.Errorf("line %d width=%d exceeds pane width=%d\nline: %q", i+1, w, tc.width, line)
				}
			}
		})
	}
}

// recordingRunner is a minimal CommandRunner for ensureSessionCmd tests. It
// records every call and lets the test pre-program responses for specific
// invocations (e.g. has-session returning success vs failure).
type recordingRunner struct {
	calls         [][]string
	failHas       bool // when true, has-session returns an error (= session missing)
	failList      bool // when true, list-windows returns an error
	failSetOption bool // when true, any set-option returns an error
}

func (r *recordingRunner) Run(_ context.Context, name string, args ...string) ([]byte, error) {
	call := append([]string{name}, args...)
	r.calls = append(r.calls, call)
	if len(args) > 0 && args[0] == "has-session" && r.failHas {
		return nil, fmt.Errorf("no session")
	}
	if len(args) > 0 && args[0] == "list-windows" && r.failList {
		return nil, fmt.Errorf("no session")
	}
	if len(args) > 0 && args[0] == "set-option" && r.failSetOption {
		return nil, fmt.Errorf("set-option failed")
	}
	return nil, nil
}

func (r *recordingRunner) callContains(args ...string) bool {
	for _, c := range r.calls {
		if len(c) < len(args) {
			continue
		}
		match := true
		for i, want := range args {
			if c[i] != want {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}

// TestEnsureSessionCmd_SetsStatusLeftOnCreate verifies the fix for the bug
// where the banner was missing inside the PR's tmux session. When a brand-new
// session is created, ensureSessionCmd must call set-option for status-left
// so the banner is visible from any window inside that session.
func TestEnsureSessionCmd_SetsStatusLeftOnCreate(t *testing.T) {
	runner := &recordingRunner{failHas: true} // forces session creation path
	mgr := tmux.NewSessionManager(runner, "/home/test", "/home/test/src")

	sess := tmux.PRSession{
		SessionName: "fix-bug-42",
		PRNumber:    42,
		WorkDir:     "/home/test/src/repo",
		Branch:      "fix-bug",
	}
	const banner = "#[fg=cyan,bold]── PR Wrangler ── #[default]My PRs | repo#42 "

	cmd := ensureSessionCmd(mgr, sess, "claude", "claude --help", banner)
	if cmd == nil {
		t.Fatal("ensureSessionCmd returned nil cmd")
	}
	if msg := cmd(); msg == nil {
		t.Fatal("expected non-nil message from ensureSessionCmd")
	}

	if !runner.callContains("tmux", "set-option", "-t", "fix-bug-42", "status-left", banner) {
		t.Errorf("expected status-left to be set after session creation\ncalls: %v", runner.calls)
	}
	if !runner.callContains("tmux", "set-option", "-t", "fix-bug-42", "status-left-length", "200") {
		t.Errorf("expected status-left-length to be raised\ncalls: %v", runner.calls)
	}
}

// TestEnsureSessionCmd_AlwaysAppliesStatusLeftOnExistingSession is the
// direct regression test for the user-reported bug: pre-existing PR sessions
// (created before pr-wrangler grew the banner feature, or under an older
// version) were never getting status-left applied because the previous
// implementation only set it on creation. Now we always apply, so users see
// the banner after upgrading without recreating every session.
func TestEnsureSessionCmd_AlwaysAppliesStatusLeftOnExistingSession(t *testing.T) {
	runner := &recordingRunner{failHas: false} // has-session succeeds → session exists
	runner.failList = true                     // window doesn't exist → new-window path
	mgr := tmux.NewSessionManager(runner, "/home/test", "/home/test/src")

	sess := tmux.PRSession{SessionName: "existing", PRNumber: 42, WorkDir: "/tmp"}

	cmd := ensureSessionCmd(mgr, sess, "claude", "claude --help", "some-banner")
	msg := cmd()

	if !runner.callContains("tmux", "set-option", "-t", "existing", "status-left", "some-banner") {
		t.Errorf("expected status-left to be applied even on existing session\ncalls: %v", runner.calls)
	}
	if ready, ok := msg.(sessionReadyMsg); ok {
		if !ready.statusLeftSet || ready.statusLeftErr != nil {
			t.Errorf("statusLeft application should be reported as successful, got set=%v err=%v",
				ready.statusLeftSet, ready.statusLeftErr)
		}
	}
}

// TestEnsureSessionCmd_SurfacesStatusLeftErrors covers the second part of
// the fix: when tmux is missing or set-option fails, we no longer swallow
// the error. The model surfaces it as a notification so the user can see
// why the banner isn't appearing.
func TestEnsureSessionCmd_SurfacesStatusLeftErrors(t *testing.T) {
	runner := &recordingRunner{failHas: true, failSetOption: true}
	mgr := tmux.NewSessionManager(runner, "/home/test", "/home/test/src")

	sess := tmux.PRSession{SessionName: "fresh", PRNumber: 42, WorkDir: "/tmp"}

	cmd := ensureSessionCmd(mgr, sess, "claude", "claude --help", "banner")
	msg := cmd()

	ready, ok := msg.(sessionReadyMsg)
	if !ok {
		t.Fatalf("expected sessionReadyMsg, got %T", msg)
	}
	if ready.statusLeftErr == nil {
		t.Error("expected statusLeftErr to be set when set-option fails")
	}
	if ready.statusLeftSet {
		t.Error("statusLeftSet should be false when application failed")
	}
}

// TestEnsureSessionCmd_SkipsStatusLeftWhenBannerEmpty covers the disabled-flag
// path: tmuxStatusLeftBanner returns "" when ShowTmuxBanner is false, and
// ensureSessionCmd must not issue any set-option calls in that case.
func TestEnsureSessionCmd_SkipsStatusLeftWhenBannerEmpty(t *testing.T) {
	runner := &recordingRunner{failHas: true}
	mgr := tmux.NewSessionManager(runner, "/home/test", "/home/test/src")

	sess := tmux.PRSession{SessionName: "new-sess", PRNumber: 42, WorkDir: "/tmp"}

	cmd := ensureSessionCmd(mgr, sess, "claude", "claude --help", "")
	_ = cmd()

	for _, c := range runner.calls {
		if len(c) >= 2 && c[1] == "set-option" {
			t.Errorf("must not call set-option when banner is empty\ncalls: %v", runner.calls)
		}
	}
}

// TestWithStartupNotification_PersistsInRender ensures the initial warning
// set from main.go (e.g. "tmux not installed") survives into the first
// frame of the TUI and is not clobbered by a window-size or fetch update.
func TestWithStartupNotification_PersistsInRender(t *testing.T) {
	cfg := config.DefaultConfig()
	m := NewModel(nil, nil, nil, cfg).WithStartupNotification("tmux not found")
	m.loading = false
	m.rows = []PRRow{{PR: github.PR{Number: 1, RepoNameWithOwner: "x/y", Title: "t"}}}
	m.width = 120
	m.height = 30

	out := stripANSI(m.View().Content)
	if !strings.Contains(out, "tmux not found") {
		t.Errorf("startup notification missing from first frame:\n%s", out)
	}
}

// TestTmuxStatusLeftBanner_BuildsTmuxFormatStringWithPRInfo covers the
// regression where the banner was only rendered in pr-wrangler's TUI, so
// switching to the PR session (e.g. into Claude Code) left no banner visible.
// The fix: when the session is created, we also write a status-left value
// onto that session so the banner shows up in tmux's own status bar.
func TestTmuxStatusLeftBanner_BuildsTmuxFormatStringWithPRInfo(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.ShowTmuxBanner = true
	cfg.Views = []config.View{{Name: "My PRs", Query: "x"}}

	m := Model{config: cfg}
	row := PRRow{PR: github.PR{Number: 42, RepoNameWithOwner: "petems/pr-wrangler"}}

	got := m.tmuxStatusLeftBanner(row)

	for _, want := range []string{"PR Wrangler", "My PRs", "pr-wrangler#42", "#[fg=", "#[default]"} {
		if !strings.Contains(got, want) {
			t.Errorf("status-left banner missing %q\ngot: %q", want, got)
		}
	}
}

// TestTmuxStatusLeftBanner_DisabledReturnsEmpty ensures the feature gate
// works: when ShowTmuxBanner is false, ensureSessionCmd receives "" and
// doesn't overwrite the session's status-left at all.
func TestTmuxStatusLeftBanner_DisabledReturnsEmpty(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.ShowTmuxBanner = false

	m := Model{config: cfg}
	row := PRRow{PR: github.PR{Number: 42, RepoNameWithOwner: "petems/pr-wrangler"}}

	if got := m.tmuxStatusLeftBanner(row); got != "" {
		t.Errorf("expected empty banner when disabled, got %q", got)
	}
}

// TestTmuxStatusLeftBanner_HandlesMissingRepoOrPR guards against malformed
// or partial PR data. The banner should still render the "PR Wrangler"
// prefix even when there's no view name or no repo/number.
func TestTmuxStatusLeftBanner_HandlesMissingRepoOrPR(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.ShowTmuxBanner = true
	cfg.Views = nil

	m := Model{config: cfg}
	row := PRRow{} // no repo, no PR number, no view

	got := m.tmuxStatusLeftBanner(row)

	if !strings.Contains(got, "PR Wrangler") {
		t.Errorf("banner should still show PR Wrangler label even with empty PR: %q", got)
	}
	if strings.Contains(got, "#0") {
		t.Errorf("banner must not render #0 when PR number is missing: %q", got)
	}
}

// TestRenderTmuxBanner_ZeroWidthUsesFallback covers the initial-render path
// where WindowSizeMsg has not arrived yet. We still want a reasonable banner
// rather than a zero-width string.
func TestRenderTmuxBanner_ZeroWidthUsesFallback(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Views = []config.View{{Name: "My PRs", Query: "x"}}

	m := Model{
		config: cfg,
		styles: NewStyles("default"),
		width:  0,
	}

	out := stripANSI(m.renderTmuxBanner())
	if !strings.Contains(out, "PR Wrangler") || !strings.Contains(out, "Ctrl+B D to detach") {
		t.Errorf("banner with zero width still needs label and detach hint:\n%s", out)
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
	m := NewDemoModel(config.DefaultConfig())

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

// TestDemoModel_RefreshIsNoOp guards against accidentally re-enabling the
// network refresh path in demo mode. Pressing 'r' must not flip loading=true
// (which would hide the populated rows behind the cowsay loading screen) or
// dispatch a fetch command.
func TestDemoModel_RefreshIsNoOp(t *testing.T) {
	m := NewDemoModel(config.DefaultConfig())
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
