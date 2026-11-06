package outliner

import (
	"strings"
	"testing"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/tui/chrome"
)

func nowCardOutliner(t *testing.T, width, height int, tasks ...*models.Task) *OutlinerModel {
	t.Helper()
	project := primaryRowsProject("site", tasks...)
	m, _ := primaryRowsOutliner(project)
	m.SetSize(width, height)
	m.RebuildRows()
	return m
}

func doingTask(id, title string, started time.Time) *models.Task {
	task := primaryRowsTask(id, models.StateDoing)
	task.Title = title
	task.StateChangedAt = started
	return task
}

// The lead is the work that entered Doing last, and the others are counted on
// the progress row rather than drawn.
func TestNowCardLeadsWithTheMostRecentlyStartedDoingTask(t *testing.T) {
	now := time.Now()
	older := doingTask("older", "Older work", now.Add(-3*time.Hour))
	newer := doingTask("newer", "Newer work", now.Add(-12*time.Minute))
	newer.ResumeNote = "Pick up at the parser"
	idle := primaryRowsTask("idle", models.StateBacklog)
	m := nowCardOutliner(t, 100, 40, older, newer, idle)

	if !m.NowCardVisible() {
		t.Fatal("the card is down over a scope holding Doing work")
	}
	if m.nowCard.task != newer || m.nowCard.more != 1 {
		t.Fatalf("lead = %v, more = %d; want the newer task and one more", m.nowCard.task, m.nowCard.more)
	}
	if got := m.NowCardRows(); got != 3 {
		t.Fatalf("rows = %d, want lead, resume, and progress", got)
	}
	lines := chrome.RenderNowCard(m.nowCardSpec(now), 100)
	if len(lines) != m.NowCardRows() {
		t.Fatalf("drew %d lines, charged %d", len(lines), m.NowCardRows())
	}
	plain := ansi.Strip(strings.Join(lines, "\n"))
	for _, want := range []string{chrome.NowCardRail + "Newer work", "site · 12m in Doing", "↳ Pick up at the parser", "+1 more in Doing"} {
		if !strings.Contains(plain, want) {
			t.Fatalf("card lacks %q:\n%s", want, plain)
		}
	}
	for _, line := range lines {
		if w := lipgloss.Width(line); w != 100 {
			t.Fatalf("line is %d cells, want 100: %q", w, ansi.Strip(line))
		}
	}
}

// A lone Doing task with nothing to measure spends one row, and the header
// charges exactly what the card draws.
func TestNowCardChargesOnlyTheRowsItDraws(t *testing.T) {
	m := nowCardOutliner(t, 80, 30, doingTask("only", "Only work", time.Now()))
	if got := m.NowCardRows(); got != 1 {
		t.Fatalf("rows = %d, want one", got)
	}
	if got, want := m.outlinerHostHeaderRows(), 1+m.contextRowRows()+1; got != want {
		t.Fatalf("host header rows = %d, want %d", got, want)
	}
}

func TestNowCardStandsDownOutsideTheHomeViews(t *testing.T) {
	m := nowCardOutliner(t, 80, 30, doingTask("only", "Only work", time.Now()))
	m.PrimaryMode = chrome.ModeInbox
	m.RebuildRows()
	if m.NowCardVisible() || m.NowCardRows() != 0 {
		t.Fatal("the card stands in Inbox")
	}
	m.PrimaryMode = chrome.ModeToday
	m.RebuildRows()
	if !m.NowCardVisible() {
		t.Fatal("the card is down in Today")
	}
}

func TestNowCardGivesShortTerminalsTheirRows(t *testing.T) {
	m := nowCardOutliner(t, 80, nowCardMinHeight-1, doingTask("only", "Only work", time.Now()))
	if m.NowCardVisible() {
		t.Fatal("the card stands in a short terminal")
	}
	m.SetSize(80, nowCardMinHeight)
	if !m.NowCardVisible() {
		t.Fatal("the card is down at its minimum height")
	}
}

func TestNowCardWithoutDoingWorkDrawsNothing(t *testing.T) {
	m := nowCardOutliner(t, 80, 30, primaryRowsTask("idle", models.StateBacklog))
	if m.NowCardVisible() || m.NowCardRows() != 0 {
		t.Fatal("the card stands with nothing in Doing")
	}
}

// A narrow row keeps the title and drops the right-hand note whole rather than
// shortening an age into a different one.
func TestNowCardJustifyDropsTheNoteBeforeTheTitle(t *testing.T) {
	line := chrome.NowCardJustify("A fairly long task title", lipgloss.NewStyle(), "website · 3h in Doing", 30)
	plain := ansi.Strip(line)
	if strings.Contains(plain, "Doing") {
		t.Fatalf("the note survived a row too narrow for it: %q", plain)
	}
	if !strings.HasPrefix(plain, "A fairly long task title") || lipgloss.Width(line) != 30 {
		t.Fatalf("line = %q (%d cells)", plain, lipgloss.Width(line))
	}
}
