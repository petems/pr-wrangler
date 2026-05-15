package tui

import (
	"fmt"
	"strings"
	"testing"

	"github.com/petems/pr-wrangler/internal/config"
	"github.com/petems/pr-wrangler/internal/github"
)

func TestTableChromeLines_Audit(t *testing.T) {
	// Use a wide terminal to avoid soft-wrapping affecting line counts.
	const width = 240

	prs := make([]github.PR, 50)
	for i := range prs {
		n := i + 1
		prs[i] = github.PR{
			Number:            n,
			Title:             fmt.Sprintf("PR %d", n),
			URL:               fmt.Sprintf("https://github.com/example/repo/pull/%d", n),
			RepoNameWithOwner: "example/repo",
			State:             github.PRStateOpen,
		}
	}

	rows := buildRows(prs, nil)
	if len(rows) < 2 {
		t.Fatalf("need at least 2 rows for audit, got %d", len(rows))
	}

	m := Model{
		config:  config.DefaultConfig(),
		styles:  NewStyles("default"),
		loading: false,
		width:   width,
		height:  5000, // ensure everything fits on one page (no page indicator)
		allRows: rows,
		rows:    rows,
	}

	baseView := m.viewContent()
	baseLines := strings.Count(baseView, "\n") + 1

	// When all rows fit on a single page, the table output is:
	//   visible body rows + 4 table chrome lines (top/header/separator/bottom)
	// and renderTable() does not emit the page indicator.
	baseChrome := baseLines - len(m.rows)
	if baseChrome <= 0 {
		t.Fatalf("unexpected baseChrome=%d (baseLines=%d rows=%d)", baseChrome, baseLines, len(m.rows))
	}

	// Now force pagination to measure the page indicator contribution.
	m.height = tableChromeLines + 3 // small page size, should guarantee multiple pages
	pageStart, pageEnd, _, totalPages := m.pageBounds()
	if totalPages <= 1 {
		t.Fatalf("expected multiple pages for audit, got totalPages=%d (pageSize=%d rows=%d)",
			totalPages, m.tablePageSize(), len(m.rows))
	}

	pagedView := m.viewContent()
	pagedLines := strings.Count(pagedView, "\n") + 1
	visibleBody := pageEnd - pageStart
	pagedChrome := pagedLines - visibleBody

	// page indicator line is prefixed with "\n" inside renderTable() when totalPages > 1
	// and lipgloss/table.Render() does not include a trailing newline, so it should cost 1 line.
	pageIndicatorLines := pagedChrome - baseChrome

	if strings.HasSuffix(m.renderTable(), "\n") {
		t.Fatalf("renderTable() unexpectedly has a trailing newline; audit assumptions would be wrong")
	}

	const transientReserve = 3
	want := baseChrome + pageIndicatorLines + transientReserve
	if tableChromeLines != want {
		t.Fatalf("tableChromeLines=%d, want %d (baseChrome=%d pageIndicatorLines=%d transientReserve=%d)",
			tableChromeLines, want, baseChrome, pageIndicatorLines, transientReserve)
	}
	if pageIndicatorLines != 1 {
		t.Fatalf("expected pageIndicatorLines=1, got %d (baseChrome=%d pagedChrome=%d)", pageIndicatorLines, baseChrome, pagedChrome)
	}
}
