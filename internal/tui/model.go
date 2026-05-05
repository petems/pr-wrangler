package tui

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"charm.land/lipgloss/v2/table"
	"github.com/charmbracelet/x/ansi"
	"github.com/petems/pr-wrangler/internal/config"
	"github.com/petems/pr-wrangler/internal/github"
	"github.com/petems/pr-wrangler/internal/session"
	"github.com/petems/pr-wrangler/internal/tmux"
)

type RowType string

const (
	RowTypePR    RowType = "pr"
	RowTypeLocal RowType = "local"
)

type PRRow struct {
	PR           github.PR
	Status       github.PRStatus
	Action       github.Action
	RowType      RowType
	AgentStatus  string
	TmuxSession  string
	WorktreePath string
	SAMLError    *github.SAMLAuthError
}

type Model struct {
	ghClient     *github.GHClient
	sessionMgr   *tmux.SessionManager
	sessionStore *session.Store
	config       config.Config

	styles Styles

	width  int
	height int

	allRows   []PRRow
	rows      []PRRow
	selected  int // highlighted row index in rows
	loading   bool
	lastError error

	spinner spinner.Model

	// PR fetch progress (detail phase). progressCh is non-nil while a fetch
	// is in flight; progressTotal == 0 means search is still running.
	progressCh    <-chan tea.Msg
	progressDone  int
	progressTotal int

	// Overlays
	showHelp bool

	notification string

	// Sessions
	prSessions map[int]tmux.PRSession

	// SAML errors for PRs that couldn't be fetched, ordered by original
	// search-result position so they can be interleaved with the PR list.
	samlErrors []github.SAMLErrorEntry
}

func NewModel(ghClient *github.GHClient, sessionMgr *tmux.SessionManager, sessionStore *session.Store, cfg config.Config) Model {
	styles := NewStyles(cfg.ColorScheme)

	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = styles.Loading

	return Model{
		ghClient:     ghClient,
		sessionMgr:   sessionMgr,
		sessionStore: sessionStore,
		config:       cfg,
		styles:       styles,
		loading:      true,
		spinner:      s,
		prSessions:   make(map[int]tmux.PRSession),
	}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(
		m.spinner.Tick,
		m.refreshCmd(),
		m.discoverSessionsCmd(),
	)
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "r":
			m.loading = true
			return m, m.refreshCmd()
		case "?":
			m.showHelp = !m.showHelp
		case "enter", "c":
			return m, m.switchToSession()
		case "o":
			return m, m.openSelectedPR()
		case "a":
			return m, m.openSAMLAuthURL()
		case "up", "k":
			if m.selected > 0 {
				m.selected--
			}
		case "down", "j":
			if m.selected < len(m.rows)-1 {
				m.selected++
			}
		case "home", "g":
			m.selected = 0
		case "end", "G":
			if len(m.rows) > 0 {
				m.selected = len(m.rows) - 1
			}
		case "pgup", "ctrl+u":
			step := m.tablePageSize()
			m.selected -= step
			if m.selected < 0 {
				m.selected = 0
			}
		case "pgdown", "ctrl+d":
			step := m.tablePageSize()
			m.selected += step
			if m.selected >= len(m.rows) {
				m.selected = len(m.rows) - 1
			}
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case prsFetchStartedMsg:
		m.progressCh = msg.progressCh
		m.progressDone = 0
		m.progressTotal = 0
		return m, waitForFetchMsgCmd(msg.progressCh)

	case prsProgressMsg:
		// Drop messages from a superseded fetch. Keep draining their
		// channel so the old goroutine isn't blocked on a full buffer.
		if msg.progressCh != m.progressCh {
			return m, waitForFetchMsgCmd(msg.progressCh)
		}
		m.progressDone = msg.done
		m.progressTotal = msg.total
		return m, waitForFetchMsgCmd(m.progressCh)

	case prsLoadedMsg:
		// Stale completion — drain to channel close and discard.
		if msg.progressCh != m.progressCh {
			return m, waitForFetchMsgCmd(msg.progressCh)
		}
		m.loading = false
		m.progressCh = nil
		m.progressDone = 0
		m.progressTotal = 0
		if msg.err != nil {
			m.lastError = msg.err
		} else {
			m.samlErrors = msg.samlErrors
			m.allRows = buildRows(msg.prs, msg.samlErrors)
			m.applyFilters()
			if m.selected >= len(m.rows) {
				m.selected = 0
			}
		}

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case sessionsDiscoveredMsg:
		m.prSessions = msg.sessions

	case worktreeReadyMsg:
		if msg.warning != "" {
			m.notification = msg.warning
		}
		if msg.warningError != nil {
			m.lastError = fmt.Errorf("%s: %w", msg.warning, msg.warningError)
		}
		return m, ensureSessionCmd(m.sessionMgr, msg.sess, msg.windowName, msg.claudeCmd)

	case sessionReadyMsg:
		// Session/window created — now suspend Bubble Tea and switch tmux client
		return m, switchClientCmd(msg.sessionName, m.sessionMgr.InsideTmux())

	case sessionCreatedMsg:
		m.notification = fmt.Sprintf("Switched to session %s [%s]", msg.sessionName, msg.windowName)

	case sessionErrorMsg:
		m.lastError = msg.err
	}

	return m, tea.Batch(cmds...)
}

// cowsayDashboard is the static cowsay shown in the main dashboard view.
const cowsayDashboard = "" +
	" _______________________________________\n" +
	"< Mooo! Welcome to PR Wrangler, pardner >\n" +
	" ---------------------------------------\n" +
	"        \\   ^__^\n" +
	"         \\  (oo)\\_______\n" +
	"            (__)\\       )\\/\\\n" +
	"                ||----w |\n" +
	"                ||     ||"

func (m Model) View() tea.View {
	v := tea.NewView(m.viewContent())
	v.AltScreen = true
	return v
}

func (m Model) viewContent() string {
	if m.loading && len(m.allRows) == 0 {
		return m.renderLoadingScreen()
	}

	var b strings.Builder
	b.WriteString(cowsayDashboard + "\n\n")
	b.WriteString(m.styles.Title.Render("PR Wrangler"))
	if q := m.configuredQuery(); q != "" {
		b.WriteString(m.styles.Help.Render(fmt.Sprintf("  [query: %s]", q)))
	}
	b.WriteString("\n\n")
	if w := queryWarning(m.configuredQuery()); w != "" {
		b.WriteString(m.styles.Warning.Render(w))
		b.WriteString("\n")
	}

	b.WriteString(m.renderTable())
	b.WriteString("\n")

	if m.lastError != nil {
		b.WriteString(m.styles.Error.Render(fmt.Sprintf("Error: %s", renderError(m.lastError))))
		b.WriteString("\n")
	}

	if m.notification != "" {
		b.WriteString(m.styles.Help.Render(m.notification))
		b.WriteString("\n")
	}

	b.WriteString(m.buildHelpLine())

	if m.showHelp {
		b.WriteString("\n\n")
		b.WriteString(m.renderHelp())
	}

	return b.String()
}

// loadingTitle is the block-letter banner shown above the cowsay during loading.
const loadingTitle = "" +
	"▗▄▄▖ ▗▄▄▖     ▗▖ ▗▖▗▄▄▖  ▗▄▖ ▗▖  ▗▖ ▗▄▄▖▗▖   ▗▄▄▄▖▗▄▄▖ \n" +
	"▐▌ ▐▌▐▌ ▐▌    ▐▌ ▐▌▐▌ ▐▌▐▌ ▐▌▐▛▚▖▐▌▐▌   ▐▌   ▐▌   ▐▌ ▐▌\n" +
	"▐▛▀▘ ▐▛▀▚▖    ▐▌ ▐▌▐▛▀▚▖▐▛▀▜▌▐▌ ▝▜▌▐▌▝▜▌▐▌   ▐▛▀▀▘▐▛▀▚▖\n" +
	"▐▌   ▐▌ ▐▌    ▐▙█▟▌▐▌ ▐▌▐▌ ▐▌▐▌  ▐▌▝▚▄▞▘▐▙▄▄▖▐▙▄▄▖▐▌ ▐▌"

// cowsayLoading is the static cowsay template. Use %s for the spinner character.
const cowsayLoading = "" +
	" ____________________________________________________\n" +
	"< %s Mooo! Fetching PR's for ya pardner, Yee-haw! %s >\n" +
	" ----------------------------------------------------\n" +
	"        \\   ^__^\n" +
	"         \\  (oo)\\_______\n" +
	"            (__)\\       )\\/\\\n" +
	"                ||----w |\n" +
	"                ||     ||"

// renderCowsay builds the centered loading screen with the title banner
// and cowsay for the given dimensions.
func renderCowsay(styles Styles, spinnerStr string, width, height int) string {
	titleLines := strings.Split(loadingTitle, "\n")
	cow := fmt.Sprintf(cowsayLoading, spinnerStr, spinnerStr)
	cowLines := strings.Split(cow, "\n")

	// Use plain placeholder for width measurement (avoids ANSI inflation)
	plainCow := fmt.Sprintf(cowsayLoading, "*", "*")
	plainCowLines := strings.Split(plainCow, "\n")

	// Find widest line across both title and cow
	maxWidth := 0
	for _, l := range titleLines {
		if w := lipgloss.Width(strings.TrimRight(l, " ")); w > maxWidth {
			maxWidth = w
		}
	}
	for _, l := range plainCowLines {
		if w := len(l); w > maxWidth {
			maxWidth = w
		}
	}

	// Blank line between title and cow
	totalLines := len(titleLines) + 1 + len(cowLines)

	// Center vertically
	padTop := (height - totalLines) / 2
	if padTop < 0 {
		padTop = 0
	}

	var b strings.Builder
	for i := 0; i < padTop; i++ {
		b.WriteByte('\n')
	}

	// Render title as a block (uniform left padding based on widest title line)
	titleMaxWidth := 0
	for _, l := range titleLines {
		if w := lipgloss.Width(strings.TrimRight(l, " ")); w > titleMaxWidth {
			titleMaxWidth = w
		}
	}
	titlePadLeft := (width - titleMaxWidth) / 2
	if titlePadLeft < 0 {
		titlePadLeft = 0
	}
	titlePrefix := strings.Repeat(" ", titlePadLeft)

	for _, line := range titleLines {
		b.WriteString(titlePrefix)
		b.WriteString(styles.Banner.Render(line))
		b.WriteByte('\n')
	}

	b.WriteByte('\n')

	// Render cow lines (centered as a block based on widest cow line)
	cowMaxWidth := 0
	for _, l := range plainCowLines {
		if len(l) > cowMaxWidth {
			cowMaxWidth = len(l)
		}
	}
	cowPadLeft := (width - cowMaxWidth) / 2
	if cowPadLeft < 0 {
		cowPadLeft = 0
	}
	cowPrefix := strings.Repeat(" ", cowPadLeft)

	for _, line := range cowLines {
		b.WriteString(cowPrefix)
		b.WriteString(styles.Loading.Render(line))
		b.WriteByte('\n')
	}

	return b.String()
}

func (m Model) renderLoadingScreen() string {
	width := m.width
	if width == 0 {
		width = 80
	}
	height := m.height
	if height == 0 {
		height = 24
	}

	var b strings.Builder
	b.WriteString(renderCowsay(m.styles, m.spinner.View(), width, height))

	// Centered status/query/warning under the cow — only while a fetch is
	// in flight.
	if m.progressCh != nil {
		var status string
		if m.progressTotal == 0 {
			status = fmt.Sprintf("%s Searching for PRs...", m.spinner.View())
		} else {
			status = renderProgressBar(m.progressDone, m.progressTotal, progressBarWidth(width))
		}
		b.WriteString("\n")
		b.WriteString(centerLine(m.styles.Loading.Render(status), width))
		b.WriteString("\n")
		if q := github.EffectiveQuery(m.configuredQuery()); q != "" {
			b.WriteString(centerLine(m.styles.Help.Render(fmt.Sprintf("query: %s", q)), width))
			b.WriteString("\n")
		}
		if w := queryWarning(m.configuredQuery()); w != "" {
			b.WriteString(centerLine(m.styles.Warning.Render(w), width))
			b.WriteString("\n")
		}
	}

	// Pin the config source line to the lower-left of the screen.
	if src := configSourceLabel(m.config); src != "" {
		body := b.String()
		used := strings.Count(body, "\n")
		pad := height - used - 1
		if pad < 0 {
			pad = 0
		}
		b.WriteString(strings.Repeat("\n", pad))
		b.WriteString(m.styles.Help.Render(src))
	}

	return b.String()
}

// configSourceLabel describes where the active config came from. Returns
// "config: <path>" when a file was loaded, or "no config file at <path>
// (using defaults)" when falling back to DefaultConfig.
func configSourceLabel(cfg config.Config) string {
	if cfg.Path == "" {
		return ""
	}
	if cfg.Loaded {
		return fmt.Sprintf("config: %s", cfg.Path)
	}
	return fmt.Sprintf("no config file at %s (using defaults)", cfg.Path)
}

// queryWarning returns a non-empty warning string when the configured query
// has no state qualifier — those queries can return up to GitHub's 1000-result
// search cap and trip secondary rate limits.
func queryWarning(query string) string {
	if query == "" {
		return ""
	}
	lower := strings.ToLower(query)
	if strings.Contains(lower, "is:open") ||
		strings.Contains(lower, "is:closed") ||
		strings.Contains(lower, "is:merged") {
		return ""
	}
	return "Warning: query has no is:open / is:closed / is:merged filter — may return up to 1000 PRs and trip GitHub rate limits"
}

func centerLine(s string, width int) string {
	pad := (width - lipgloss.Width(s)) / 2
	if pad < 0 {
		pad = 0
	}
	return strings.Repeat(" ", pad) + s
}

// configuredQuery returns the user's configured search query, or "" if none.
func (m Model) configuredQuery() string {
	if len(m.config.Views) == 0 {
		return ""
	}
	return m.config.Views[0].Query
}

// progressBarWidth picks a sensible bar width for the given terminal width.
func progressBarWidth(termWidth int) int {
	const max = 40
	w := termWidth - 20
	if w > max {
		return max
	}
	if w < 10 {
		return 10
	}
	return w
}

// renderProgressBar produces "[█████░░░░░] 12/47 PRs".
func renderProgressBar(done, total, width int) string {
	if total <= 0 {
		return ""
	}
	if done > total {
		done = total
	}
	filled := done * width / total
	if filled > width {
		filled = width
	}
	bar := strings.Repeat("█", filled) + strings.Repeat("░", width-filled)
	return fmt.Sprintf("[%s] %d/%d PRs", bar, done, total)
}

func (m *Model) refreshCmd() tea.Cmd {
	return fetchPRsCmd(m.ghClient, m.configuredQuery())
}

func (m *Model) discoverSessionsCmd() tea.Cmd {
	return discoverSessionsCmd(m.sessionMgr)
}

func (m *Model) applyFilters() {
	m.rows = m.allRows
	// TODO: implement filtering
}

// Layout constants for the table view.
const (
	// nonTitleColumnsWidth reserves columns/padding around the Title column:
	// indicator (2) + repo (20) + pr (8) + status (20) + action (20) plus
	// per-column borders and padding.
	nonTitleColumnsWidth = 82
	minTitleColumnWidth  = 10
	// tableChromeLines reserves rows of the terminal for non-table chrome
	// in View() so the cowsay header stays visible:
	//   - cowsay block: 8 lines + 1 trailing blank = 9
	//   - title line + 1 trailing blank          = 2
	//   - table chrome (top border, header,
	//     header separator, bottom border)       = 4
	//   - page indicator + blank                 = 2
	//   - blank line after table + help line     = 2
	//   - reserve for transient warning/error/
	//     notification rows                      = 3
	tableChromeLines = 22
	minPageSize      = 1
)

// titleColumnWidth returns the width to allocate to the Title column.
func (m Model) titleColumnWidth() int {
	w := m.width - nonTitleColumnsWidth
	if w < minTitleColumnWidth {
		return minTitleColumnWidth
	}
	return w
}

// tablePageSize returns the table page size based on the available
// terminal height, leaving room for title, table header/footer, and the
// help line.
func (m Model) tablePageSize() int {
	if m.height <= tableChromeLines+minPageSize {
		return minPageSize
	}
	return m.height - tableChromeLines
}

// columnWidths returns the per-column widths in column order.
func (m Model) columnWidths() []int {
	return []int{2, 20, 8, m.titleColumnWidth(), 20, 20}
}

// pageBounds returns the [start, end) row indices for the visible page,
// keeping the selected row inside the window.
func (m Model) pageBounds() (start, end, page, totalPages int) {
	pageSize := m.tablePageSize()
	if pageSize < 1 {
		pageSize = 1
	}
	totalPages = (len(m.rows) + pageSize - 1) / pageSize
	if totalPages < 1 {
		totalPages = 1
	}
	page = m.selected / pageSize
	start = page * pageSize
	end = start + pageSize
	if end > len(m.rows) {
		end = len(m.rows)
	}
	return start, end, page + 1, totalPages
}

func (m Model) renderTable() string {
	if len(m.rows) == 0 {
		return m.styles.Help.Render("(no rows)")
	}

	start, end, page, totalPages := m.pageBounds()
	widths := m.columnWidths()

	rows := make([][]string, 0, end-start)
	urls := make([][]string, 0, end-start)
	for i := start; i < end; i++ {
		r := m.rows[i]
		indicator := " "
		if i == m.selected {
			indicator = ">"
		}
		repoCell := truncateAnsi(extractRepoName(r.PR.RepoNameWithOwner), widths[1])
		prCell := fmt.Sprintf("#%d", r.PR.Number)
		titleCell := truncateAnsi(r.PR.Title, widths[3])
		statusCell := truncateAnsi(r.Status.String(), widths[4])
		actionCell := truncateAnsi(r.Action.String(), widths[5])

		rows = append(rows, []string{indicator, repoCell, prCell, titleCell, statusCell, actionCell})

		// Per-cell URLs. Empty string = no hyperlink for that cell.
		repoURL := ""
		if owner := r.PR.RepoNameWithOwner; owner != "" {
			repoURL = "https://github.com/" + owner
		}
		prURL := r.PR.URL
		statusURL := ""
		if r.PR.URL != "" {
			statusURL = r.PR.URL + "/checks"
		}
		urls = append(urls, []string{"", repoURL, prURL, prURL, statusURL, ""})
	}

	headers := []string{" ", "Repo", "PR", "Title", "Status", "Action"}
	headerStyle := lipgloss.NewStyle().Bold(true).Foreground(m.styles.TableText)
	baseStyle := lipgloss.NewStyle().Foreground(m.styles.TableText)
	indicatorStyle := m.styles.Indicator
	selectedStyle := m.styles.SelectedRow

	t := table.New().
		Border(lipgloss.RoundedBorder()).
		BorderStyle(m.styles.Help).
		Headers(headers...).
		Rows(rows...).
		StyleFunc(func(row, col int) lipgloss.Style {
			width := widths[col]
			s := lipgloss.NewStyle().Width(width).Padding(0, 1)
			if row == table.HeaderRow {
				return s.Inherit(headerStyle)
			}
			// row is the visible-row index (0 = first body row)
			rowIdx := start + row
			isSelected := rowIdx == m.selected
			cell := s.Inherit(baseStyle)
			if col == 0 {
				cell = s.Inherit(indicatorStyle)
			}
			if isSelected {
				cell = cell.Inherit(selectedStyle)
			}
			if row >= 0 && row < len(urls) && col < len(urls[row]) {
				if u := urls[row][col]; u != "" {
					cell = cell.Hyperlink(u)
				}
			}
			return cell
		})

	out := t.Render()

	// Page indicator
	if totalPages > 1 {
		out += "\n" + m.styles.Help.Render(fmt.Sprintf("page %d/%d  (%d rows)", page, totalPages, len(m.rows)))
	}
	return out
}

// truncateAnsi returns s shortened to at most width display columns. It uses
// charmbracelet/x/ansi.Truncate which is OSC 8-aware, so it's safe to call on
// strings that already contain hyperlink wrappers.
func truncateAnsi(s string, width int) string {
	if width <= 0 {
		return ""
	}
	if ansi.StringWidth(s) <= width {
		return s
	}
	if width <= 1 {
		return ansi.Truncate(s, width, "")
	}
	return ansi.Truncate(s, width, "…")
}

func (m *Model) openSelectedPR() tea.Cmd {
	if m.selected < 0 || m.selected >= len(m.rows) {
		return nil
	}
	r := m.rows[m.selected]
	if r.PR.URL == "" {
		return nil
	}
	return func() tea.Msg {
		openBrowser(r.PR.URL)
		return nil
	}
}

func (m *Model) openSAMLAuthURL() tea.Cmd {
	if m.selected < 0 || m.selected >= len(m.rows) {
		return nil
	}
	r := m.rows[m.selected]

	// Only open SAML auth URL if this is a SAML-protected PR
	if r.SAMLError == nil || r.SAMLError.AuthURL == "" {
		return nil
	}

	return func() tea.Msg {
		openBrowser(r.SAMLError.AuthURL)
		return nil
	}
}

func (m Model) switchToSession() tea.Cmd {
	if m.selected < 0 || m.selected >= len(m.rows) {
		return func() tea.Msg {
			return sessionErrorMsg{err: fmt.Errorf("no PR selected")}
		}
	}
	r := m.rows[m.selected]

	if r.Status == github.PRStatusSAMLRequired {
		msg := "authorize SAML for this org outside the app, then refresh"
		if r.SAMLError != nil && r.SAMLError.AuthURL != "" {
			msg = "authorize SAML first (press 'a' to open auth URL)"
		}
		return func() tea.Msg {
			return sessionErrorMsg{err: fmt.Errorf("%s", msg)}
		}
	}

	repoName := extractRepoName(r.PR.RepoNameWithOwner)
	repoDir := m.sessionMgr.RepoDir(repoName)
	if _, err := os.Stat(repoDir); err != nil {
		return func() tea.Msg {
			return sessionErrorMsg{err: fmt.Errorf("repo dir not found: %s (clone the repo first)", repoDir)}
		}
	}

	windowName, claudeCmd := m.claudeWindowAndCmd(&r, "")
	sessionName := tmux.SanitizeSessionName(r.PR.Title, r.PR.Number)

	sess := tmux.PRSession{
		SessionName: sessionName,
		PRNumber:    r.PR.Number,
		PRTitle:     r.PR.Title,
		WorkDir:     repoDir,
		Branch:      r.PR.HeadRefName,
		PRURL:       r.PR.URL,
	}

	return ensureWorktreeCmd(m.sessionMgr, sess, repoDir, windowName, claudeCmd)
}

type helpEntry struct {
	shortKey  string
	longKey   string
	shortDesc string
	longDesc  string
}

var helpEntries = []helpEntry{
	{"q", "q / ctrl+c", "quit", "quit"},
	{"r", "r", "refresh", "refresh PRs"},
	{"enter/c", "enter / c", "claude session", "open or switch to Claude session"},
	{"o", "o", "open", "open selected PR in browser"},
	{"a", "a", "authorize SAML", "open SAML authorization URL for selected PR"},
	{"j/k", "j / k / ↑ / ↓", "navigate", "move selection up/down"},
	{"?", "?", "help", "toggle help"},
}

func (m Model) buildHelpLine() string {
	parts := make([]string, len(helpEntries))
	for i, e := range helpEntries {
		parts[i] = fmt.Sprintf("%s: %s", e.shortKey, e.shortDesc)
	}
	return m.styles.Help.Render(strings.Join(parts, " | "))
}

func (m Model) renderHelp() string {
	keyWidth := 0
	for _, e := range helpEntries {
		if w := lipgloss.Width(e.longKey); w > keyWidth {
			keyWidth = w
		}
	}
	lines := make([]string, 0, len(helpEntries)+1)
	lines = append(lines, m.styles.HelpCategory.Render("Keyboard"))
	for _, e := range helpEntries {
		padding := strings.Repeat(" ", keyWidth-lipgloss.Width(e.longKey)+2)
		lines = append(lines, m.styles.Help.Render(e.longKey+padding+e.longDesc))
	}
	return lipgloss.JoinVertical(lipgloss.Left, lines...)
}

func extractRepoName(nameWithOwner string) string {
	parts := strings.SplitN(nameWithOwner, "/", 2)
	if len(parts) == 2 {
		return parts[1]
	}
	return nameWithOwner
}

func (m Model) claudeWindowAndCmd(r *PRRow, customPrompt string) (string, string) {
	if customPrompt != "" {
		return "claude", fmt.Sprintf("claude --permission-mode acceptEdits '%s'", customPrompt)
	}

	cmdTemplate := m.config.AgentCommands["followup"]
	windowName := "claude"

	switch r.Action {
	case github.ActionFixCI:
		cmdTemplate = m.config.AgentCommands["fix-ci"]
		windowName = "ci-fix"
	case github.ActionAddressFeedback:
		cmdTemplate = m.config.AgentCommands["address-feedback"]
		windowName = "feedback"
	case github.ActionResolveConflicts:
		cmdTemplate = m.config.AgentCommands["resolve-conflicts"]
		windowName = "conflicts"
	case github.ActionReviewComments:
		cmdTemplate = m.config.AgentCommands["review-comments"]
		windowName = "reviews"
	}

	if cmdTemplate == "" {
		cmdTemplate = "claude --permission-mode acceptEdits 'Continue working on this PR: {{pr_url}} - Review the current state, check for any issues, and make progress on remaining work.'"
	}

	// Prefix with GITHUB_TOKEN so the JS CLI (and any subprocess) uses our managed token
	tokenPrefix := ""
	if m.ghClient.Token() != "" {
		escapedToken := strings.ReplaceAll(m.ghClient.Token(), "'", "'\"'\"'")
		tokenPrefix = fmt.Sprintf("GITHUB_TOKEN='%s' ", escapedToken)
	}

	cmd := strings.ReplaceAll(cmdTemplate, "{{pr_url}}", r.PR.URL)
	cmd = strings.ReplaceAll(cmd, "{{pr_number}}", fmt.Sprintf("%d", r.PR.Number))
	cmd = strings.ReplaceAll(cmd, "{{repo_nwo}}", r.PR.RepoNameWithOwner)
	return windowName, tokenPrefix + cmd
}

// renderError formats an error for the TUI status line. When the error chain
// contains a *github.SAMLAuthError with an AuthURL, the URL is replaced with
// an OSC 8 hyperlink so users can Cmd/Ctrl+Click straight to GitHub's SAML
// authorization page. The underlying error string from SAMLAuthError.Error()
// is left unchanged for non-TUI callers (logs, tests).
func renderError(err error) string {
	if err == nil {
		return ""
	}
	msg := err.Error()
	var samlErr *github.SAMLAuthError
	if errors.As(err, &samlErr) && samlErr.AuthURL != "" {
		// Replace the bare URL embedded by SAMLAuthError.Error() with a
		// hyperlinked version. Keeps surrounding "(authorize at: ...)" text
		// intact when SAMLAuthError is the leaf, and also handles the case
		// where the error has been wrapped by fmt.Errorf("%w", ...).
		msg = strings.ReplaceAll(msg, samlErr.AuthURL, Link(samlErr.AuthURL, samlErr.AuthURL))
	}
	return msg
}

func openBrowser(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "linux":
		cmd = exec.Command("xdg-open", url)
	default:
		return
	}
	_ = cmd.Start()
}

// buildRows interleaves successful PRs and SAML-protected placeholders back
// into a single list in the original search-API ordering. prs is in original
// order with SAML positions removed; samlErrors carries each SAML failure
// tagged with its original index. Merged/closed PRs are filtered out.
func buildRows(prs []github.PR, samlErrors []github.SAMLErrorEntry) []PRRow {
	var rows []PRRow
	prIdx := 0
	samlIdx := 0
	originalPos := 0

	for prIdx < len(prs) || samlIdx < len(samlErrors) {
		samlMatches := samlIdx < len(samlErrors) && samlErrors[samlIdx].Index == originalPos
		// Consume a SAML entry when its Index lines up, OR when prs is
		// exhausted but SAML entries remain (only possible if the invariant
		// from FetchPRs has drifted; treat as defensive fallback).
		if samlMatches || prIdx >= len(prs) {
			rows = append(rows, samlPlaceholderRow(samlErrors[samlIdx]))
			samlIdx++
			originalPos++
			continue
		}
		// Bounds verified above: the prsExhausted branch above continues, so
		// reaching this line implies prIdx < len(prs).
		pr := prs[prIdx] // #nosec G602 -- guarded by prsExhausted check above
		if pr.State != github.PRStateMerged && pr.State != github.PRStateClosed {
			status := github.DetermineStatus(pr)
			rows = append(rows, PRRow{
				PR:      pr,
				Status:  status,
				Action:  github.DetermineAction(status),
				RowType: RowTypePR,
			})
		}
		prIdx++
		originalPos++
	}

	return rows
}

// samlPlaceholderRow builds a PRRow for a SAML-protected PR that couldn't be
// fetched. Action falls back to ActionNone when no authorization URL was
// extracted, since pressing 'a' would otherwise no-op silently.
func samlPlaceholderRow(entry github.SAMLErrorEntry) PRRow {
	action := github.ActionAuthorizeSAML
	if entry.Err == nil || entry.Err.AuthURL == "" {
		action = github.ActionNone
	}
	return PRRow{
		PR: github.PR{
			Number:            entry.PRNumber,
			Title:             "SAML Authorization Required",
			RepoNameWithOwner: entry.RepoNameWithOwner,
			State:             github.PRStateOpen,
		},
		Status:    github.PRStatusSAMLRequired,
		Action:    action,
		RowType:   RowTypePR,
		SAMLError: entry.Err,
	}
}
