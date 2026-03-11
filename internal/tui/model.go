package tui

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/evertras/bubble-table/table"
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
}

type Model struct {
	ghClient     *github.GHClient
	sessionMgr   *tmux.SessionManager
	sessionStore *session.Store
	config       config.Config

	width  int
	height int

	allRows   []PRRow
	rows      []PRRow
	table     table.Model
	loading   bool
	lastError error

	spinner spinner.Model

	// Filtering
	repoFilter   string
	statusFilter string
	searchFilter string

	// Overlays
	showHelp     bool
	showPRDetail bool
	prDetailIdx  int

	notification string

	// Sessions
	prSessions map[int]tmux.PRSession
}

func NewModel(ghClient *github.GHClient, sessionMgr *tmux.SessionManager, sessionStore *session.Store, cfg config.Config) Model {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = loadingStyle

	m := Model{
		ghClient:     ghClient,
		sessionMgr:   sessionMgr,
		sessionStore: sessionStore,
		config:       cfg,
		loading:      true,
		spinner:      s,
		prSessions:   make(map[int]tmux.PRSession),
	}
	m.table = m.rebuildTable()
	return m
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
	case tea.KeyMsg:
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
		}

		var cmd tea.Cmd
		m.table, cmd = m.table.Update(msg)
		return m, cmd

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.table = m.rebuildTable()

	case prsLoadedMsg:
		m.loading = false
		if msg.err != nil {
			m.lastError = msg.err
		} else {
			m.allRows = buildRows(msg.prs, m.config.ServiceLabelPrefix)
			m.applyFilters()
			m.table = m.rebuildTable()
		}

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case sessionsDiscoveredMsg:
		m.prSessions = msg.sessions
		m.table = m.rebuildTable()

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

func (m Model) View() string {
	if m.loading && len(m.allRows) == 0 {
		return m.renderLoadingScreen()
	}

	var b strings.Builder
	b.WriteString(titleStyle.Render("PR Wrangler"))
	b.WriteString("\n")

	b.WriteString(m.table.View())
	b.WriteString("\n")

	if m.lastError != nil {
		b.WriteString(errorStyle.Render(fmt.Sprintf("Error: %v", m.lastError)))
		b.WriteString("\n")
	}

	if m.notification != "" {
		b.WriteString(helpStyle.Render(m.notification))
		b.WriteString("\n")
	}

	b.WriteString(m.buildHelpLine())

	if m.showHelp {
		// TODO: render help overlay
	}

	return b.String()
}

// loadingTitle is the block-letter banner shown above the cowsay during loading.
const loadingTitle = "" +
	"\u2597\u2584\u2584\u2596 \u2597\u2584\u2584\u2596     \u2597\u2596 \u2597\u2596\u2597\u2584\u2584\u2596  \u2597\u2584\u2596 \u2597\u2596  \u2597\u2596 \u2597\u2584\u2584\u2596\u2597\u2596   \u2597\u2584\u2584\u2584\u2596\u2597\u2584\u2584\u2596 \n" +
	"\u2590\u258c \u2590\u258c\u2590\u258c \u2590\u258c    \u2590\u258c \u2590\u258c\u2590\u258c \u2590\u258c\u2590\u258c \u2590\u258c\u2590\u259b\u259a\u2596\u2590\u258c\u2590\u258c   \u2590\u258c   \u2590\u258c   \u2590\u258c \u2590\u258c\n" +
	"\u2590\u259b\u2580\u2598 \u2590\u259b\u2580\u259a\u2596    \u2590\u258c \u2590\u258c\u2590\u259b\u2580\u259a\u2596\u2590\u259b\u2580\u259c\u258c\u2590\u258c \u259d\u259c\u258c\u2590\u258c\u259d\u259c\u258c\u2590\u258c   \u2590\u259b\u2580\u2580\u2598\u2590\u259b\u2580\u259a\u2596\n" +
	"\u2590\u258c   \u2590\u258c \u2590\u258c    \u2590\u2599\u2588\u259f\u258c\u2590\u258c \u2590\u258c\u2590\u258c \u2590\u258c\u2590\u258c  \u2590\u258c\u259d\u259a\u2584\u259e\u2598\u2590\u2599\u2584\u2584\u2596\u2590\u2599\u2584\u2584\u2596\u2590\u258c \u2590\u258c"

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
func renderCowsay(spinnerStr string, width, height int) string {
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

	// Centre vertically
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
		b.WriteString(bannerStyle.Render(line))
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
		b.WriteString(loadingStyle.Render(line))
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
	return renderCowsay(m.spinner.View(), width, height)
}

func (m *Model) refreshCmd() tea.Cmd {
	query := m.config.Views[0].Query // Default for now
	return fetchPRsCmd(m.ghClient, query)
}

func (m *Model) discoverSessionsCmd() tea.Cmd {
	return discoverSessionsCmd(m.sessionMgr)
}

func (m *Model) applyFilters() {
	m.rows = m.allRows
	// TODO: implement filtering
}

func (m Model) rebuildTable() table.Model {
	columns := []table.Column{
		table.NewColumn("repo", "Repo", 20),
		table.NewColumn("pr", "PR", 8),
		table.NewColumn("title", "Title", m.width-80),
		table.NewColumn("status", "Status", 20),
		table.NewColumn("action", "Action", 20),
	}

	var tableRows []table.Row
	for _, r := range m.rows {
		tableRows = append(tableRows, table.NewRow(table.RowData{
			"repo":   truncate(extractRepoName(r.PR.RepoNameWithOwner), 20),
			"pr":     fmt.Sprintf("#%d", r.PR.Number),
			"title":  truncate(r.PR.Title, m.width-80),
			"status": r.Status.String(),
			"action": r.Action.String(),
		}))
	}

	t := table.New(columns).
		WithRows(tableRows).
		Focused(true).
		WithBaseStyle(lipgloss.NewStyle().Foreground(white))

	return t
}

func buildRows(prs []github.PR, servicePrefix string) []PRRow {
	var rows []PRRow
	for _, pr := range prs {
		if pr.State == github.PRStateMerged || pr.State == github.PRStateClosed {
			continue
		}
		status := github.DetermineStatus(pr)
		action := github.DetermineAction(status)
		rows = append(rows, PRRow{
			PR:      pr,
			Status:  status,
			Action:  action,
			RowType: RowTypePR,
		})
	}
	return rows
}

func (m *Model) openSelectedPR() tea.Cmd {
	idx := m.table.GetHighlightedRowIndex()
	if idx < 0 || idx >= len(m.rows) {
		return nil
	}
	r := m.rows[idx]
	return func() tea.Msg {
		openBrowser(r.PR.URL)
		return nil
	}
}

func (m Model) switchToSession() tea.Cmd {
	idx := m.table.GetHighlightedRowIndex()
	if idx < 0 || idx >= len(m.rows) {
		return func() tea.Msg {
			return sessionErrorMsg{err: fmt.Errorf("no PR selected")}
		}
	}
	r := m.rows[idx]

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

func (m Model) buildHelpLine() string {
	return helpStyle.Render("q: quit | r: refresh | enter/c: claude session | o: open | ?: help")
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	if max <= 3 {
		return s[:max]
	}
	return s[:max-3] + "..."
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
	}

	if cmdTemplate == "" {
		cmdTemplate = "claude --permission-mode acceptEdits 'Continue working on this PR: {{pr_url}} - Review the current state, check for any issues, and make progress on remaining work.'"
	}

	cmd := strings.ReplaceAll(cmdTemplate, "{{pr_url}}", r.PR.URL)
	return windowName, cmd
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
