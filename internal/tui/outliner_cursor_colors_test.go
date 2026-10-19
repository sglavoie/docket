package tui

import (
	"strings"
	"testing"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/chrome"
	"github.com/sglavoie/tt/internal/tui/outliner"
	"github.com/sglavoie/tt/internal/tui/query"
	"github.com/sglavoie/tt/internal/tui/taskref"
	"github.com/sglavoie/tt/internal/tui/textfit"
	"github.com/sglavoie/tt/internal/tui/theme"
)

// TestCursorTaskRowKeepsEveryColorItEarned is the inverse of what SelectedLine
// guarantees: the row under the cursor is the one row that must still say what
// it is, so the badges it earned survive intact. Its title is the one thing that
// was never earned — a title is neutral text on every row, cursor included — and
// the tag axis is Peek's, so the cursor does not buy a chip back.
func TestCursorTaskRowKeepsEveryColorItEarned(t *testing.T) {
	task := &models.Task{
		ID:            "t1",
		ShortID:       1,
		Title:         "Ship the release",
		Priority:      2,
		Tags:          []string{"infra"},
		WorkflowState: models.StateDoing,
	}
	outlinerView, project := cursorHighlightOutliner(t, task)
	row := outliner.OutlinerRow{Task: task, ProjectID: project.ID}

	line := outlinerView.RenderOutlinerTaskRow(row, true, 80, 0, nil)

	for name, want := range map[string]string{
		"state badge":    theme.GetStateBadge(models.StateDoing),
		"priority badge": chrome.RenderPriorityBadge(2),
	} {
		if !strings.Contains(line, want) {
			t.Fatalf("cursor row lost its %s:\n%q", name, line)
		}
	}
	if !strings.Contains(line, theme.NormalStyle.Render(task.Title)) {
		t.Fatalf("cursor row did not render its title as ordinary text:\n%q", line)
	}
	for name, unwanted := range map[string]string{
		"priority title color": theme.GetTitlePriorityStyle(2).Render(task.Title),
		"selection style":      theme.SelectedTaskTitleStyle.Render(task.Title),
	} {
		if strings.Contains(line, unwanted) {
			t.Fatalf("cursor row repainted its title with the %s:\n%q", name, line)
		}
	}
	if strings.Contains(ansi.Strip(line), "#infra") {
		t.Fatalf("cursor row spent a column on a tag chip Peek already names:\n%q", ansi.Strip(line))
	}
}

// TestCursorRowKeepsItsSearchMatchHighlight holds the highlight through the
// band: the marked row draws the same highlighted run an unmarked one does, with
// the band under it and nothing of it rewritten.
func TestCursorRowKeepsItsSearchMatchHighlight(t *testing.T) {
	task := &models.Task{ID: "t1", ShortID: 1, Title: "Ship the release", Priority: 3}
	outlinerView, project := cursorHighlightOutliner(t, task)
	outlinerView.SetSearchQuery("release")
	row := outliner.OutlinerRow{Task: task, ProjectID: project.ID}

	line := outlinerView.RenderOutlinerTaskRow(row, true, 80, 0, nil)

	positions := query.SearchSubstringPositions(task.Title, "release", false)
	if len(positions) == 0 {
		t.Fatal("fixture query matched nothing")
	}
	want := textfit.HighlightText(task.Title, positions, theme.NormalStyle)
	if !strings.Contains(withoutTheCursorBand(t, line), want) {
		t.Fatalf("cursor row dropped its search-match highlight:\n%q", line)
	}
	if !strings.Contains(line, backgroundParams(t, theme.ColorCursorRowBg)) {
		t.Fatalf("cursor row carried the highlight but no band:\n%q", line)
	}
}

// TestCursorRowKeepsItsGutterForegroundsOnTheBand is the other half of the
// shape: the connectors and fold arrow past column 0 keep the foregrounds they
// wear on every other row, and the band is the only thing the cursor adds to
// them. Only the bar replaces what it covers, the selection mark included.
func TestCursorRowKeepsItsGutterForegroundsOnTheBand(t *testing.T) {
	task := &models.Task{ID: "t1", ShortID: 1, Title: "Ship", WorkflowState: models.StateBacklog}
	outlinerView, project := cursorHighlightOutliner(t, task)
	prefix := "│   ├── "
	row := outliner.OutlinerRow{Task: task, ProjectID: project.ID, Prefix: prefix}
	outlinerView.ReplaceSelection([]taskref.TaskIdentity{outlinerView.TaskIdentityForRow(row)})

	line := outlinerView.RenderOutlinerTaskRow(row, true, 80, 0, nil)

	banded := func(style lipgloss.Style) lipgloss.Style {
		return style.Background(theme.ColorCursorRowBg)
	}
	if want := banded(theme.MutedStyle).Render(prefix); !strings.Contains(line, want) {
		t.Fatalf("cursor row repainted its tree connectors:\n%q", line)
	}
	if !strings.HasPrefix(line, theme.CursorBarStyle.Render("✓")) {
		t.Fatalf("selected cursor row does not open with its mark under the bar:\n%q", line)
	}
}

// TestCursorRowBandOwnsTheBackgroundItStandsOn is background precedence, stated
// once: the band is the marked row's background and the overdue row tint is not
// applied over or under it. Nothing about the deadline is lost to that, which is
// the other half of the claim — the tint was never the only thing saying the task
// is late, and the badge that says so keeps its own colors on top of the band.
func TestCursorRowBandOwnsTheBackgroundItStandsOn(t *testing.T) {
	overdue := time.Now().AddDate(0, 0, -3)
	task := &models.Task{
		ID: "overdue", ShortID: 2, Title: "Past due", DueDate: &overdue,
		WorkflowState: models.StateBacklog,
	}

	outlinerView, project := cursorHighlightOutliner(t, task)
	row := outliner.OutlinerRow{Task: task, ProjectID: project.ID}
	marked := outlinerView.RenderOutlinerTaskRow(row, true, 80, 0, nil)
	ordinary := outlinerView.RenderOutlinerTaskRow(row, false, 80, 0, nil)

	tint := backgroundParams(t, theme.OverdueRowStyle.GetBackground())
	if !strings.Contains(ordinary, tint) {
		t.Fatalf("the overdue row wears no tint to begin with:\n%q", ordinary)
	}
	if strings.Contains(marked, tint) {
		t.Fatalf("the marked overdue row wears both the band and the row tint:\n%q", marked)
	}
	if !strings.Contains(marked, backgroundParams(t, theme.ColorCursorRowBg)) {
		t.Fatalf("the marked overdue row wears no band:\n%q", marked)
	}
	assertCursorRowKeepsItsForegrounds(t, marked, ordinary)
}

// TestCursorRowKeepsEverySemanticColorOnItsBand is the whole legibility claim in
// one row. State, priority, a missed deadline, an owed review, a reference
// census, tags and a search match all reach a row as foregrounds and badges of
// their own, and the band is drawn under every one of them, so a row carrying all
// of them at once paints exactly the colors it painted unmarked. Both themes are
// checked because each derives its own band, and a band that swallowed a signal
// would swallow a different one in each.
func TestCursorRowKeepsEverySemanticColorOnItsBand(t *testing.T) {
	for _, themeName := range []string{"dark", "light"} {
		t.Run(themeName, func(t *testing.T) {
			overdue := time.Now().AddDate(0, 0, -3)
			task := &models.Task{
				ID: "loud", ShortID: 3, Title: "Ship the release",
				Priority: 1, DueDate: &overdue, Review: true,
				Tags:          []string{"infra"},
				WorkflowState: models.StateWaiting,
				Dependencies:  []models.TaskReference{{ProjectID: "project", TaskID: "t1"}},
			}
			blocker := &models.Task{ID: "t1", ShortID: 1, Title: "Land the branch", WorkflowState: models.StateBacklog}
			outlinerView, project := cursorHighlightOutlinerTheme(t, themeName, blocker, task)
			outlinerView.SetSearchQuery("release")
			row := outliner.OutlinerRow{Task: task, ProjectID: project.ID}

			marked := outlinerView.RenderOutlinerTaskRow(row, true, 120, 0, nil)
			ordinary := outlinerView.RenderOutlinerTaskRow(row, false, 120, 0, nil)

			if !strings.Contains(marked, backgroundParams(t, theme.ColorCursorRowBg)) {
				t.Fatalf("the marked row wears no band:\n%q", marked)
			}
			assertCursorRowKeepsItsForegrounds(t, marked, ordinary)
			if plainRowText(marked) != plainRowText(ordinary) {
				t.Fatalf("the band changed what the row says:\nmarked   %q\nordinary %q",
					plainRowText(marked), plainRowText(ordinary))
			}
		})
	}
}

// plainRowText is what a row says, with the trailing columns left out: a marked
// row pads its width and an unmarked one stops at its content, and that
// difference is the fill rather than anything the row spells.
func plainRowText(line string) string {
	return strings.TrimRight(ansi.Strip(line), " ")
}

// assertCursorRowKeepsItsForegrounds holds a marked row to every foreground the
// same row painted unmarked. The set is taken off the unmarked render rather than
// named, because the row's signals are whatever it earned and a named list would
// stop covering one the moment the row's ladder dropped it.
func assertCursorRowKeepsItsForegrounds(t *testing.T, marked, ordinary string) {
	t.Helper()
	for _, foreground := range uxDailyUseForegrounds(t, ordinary) {
		if !strings.Contains(marked, foreground) {
			t.Fatalf("the band swallowed the %s the row paints unmarked:\nmarked   %q\nordinary %q",
				foreground, marked, ordinary)
		}
	}
}

// TestWaitingRowWearsNoTint is the other half of that rule. A row tint is the
// loudest mark the list has, so the overdue row above is the only row that earns
// one: an ordinary Waiting task is the quietest work in the workspace and says
// so through its badge alone. The age that badge used to carry is Peek's, which
// is the quieter claim, not a weaker one.
func TestWaitingRowWearsNoTint(t *testing.T) {
	task := &models.Task{
		ID: "waiting", ShortID: 1, Title: "Blocked on review",
		WorkflowState: models.StateWaiting, StateChangedAt: time.Now().AddDate(0, 0, -2),
	}
	outlinerView, project := cursorHighlightOutliner(t, task)
	row := outliner.OutlinerRow{Task: task, ProjectID: project.ID}

	for name, selected := range map[string]bool{"cursor": true, "ordinary": false} {
		line := outlinerView.RenderOutlinerTaskRow(row, selected, 80, 0, nil)
		if strings.Contains(line, backgroundParams(t, theme.OverdueRowStyle.GetBackground())) {
			t.Fatalf("%s waiting row painted the overdue row tint:\n%q", name, line)
		}
		if !strings.Contains(line, theme.GetStateBadge(models.StateWaiting)) {
			t.Fatalf("%s waiting row lost the badge that says it is waiting:\n%q", name, line)
		}
		if strings.Contains(ansi.Strip(line), "waiting") {
			t.Fatalf("%s waiting row spelled the age its badge already implies:\n%q", name, ansi.Strip(line))
		}
	}

	outlinerView.SetSize(80, 24)
	outlinerView.PeekMode = true
	peek := ansi.Strip(strings.Join(outlinerView.PeekDocumentBodyForRow(row), "\n"))
	if !strings.Contains(peek, "Waiting 2d") {
		t.Fatalf("Peek did not carry the age the row stopped naming:\n%s", peek)
	}
}

// TestWaitingRowKeepsTheOverdueTintItEarns holds the seam the other way: Waiting
// no longer reinterprets a deadline, so a Waiting task that missed one is tinted
// for the deadline exactly like any other task.
func TestWaitingRowKeepsTheOverdueTintItEarns(t *testing.T) {
	overdue := time.Now().AddDate(0, 0, -3)
	task := &models.Task{
		ID: "waiting-overdue", ShortID: 1, Title: "Blocked and late", DueDate: &overdue,
		WorkflowState: models.StateWaiting, StateChangedAt: time.Now().AddDate(0, 0, -2),
	}
	outlinerView, project := cursorHighlightOutliner(t, task)
	row := outliner.OutlinerRow{Task: task, ProjectID: project.ID}

	line := outlinerView.RenderOutlinerTaskRow(row, false, 80, 0, nil)

	if !strings.Contains(line, backgroundParams(t, theme.OverdueRowStyle.GetBackground())) {
		t.Fatalf("overdue waiting row dropped the tint its deadline earned:\n%q", line)
	}
}
