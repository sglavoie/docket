package tui

import (
	"image/color"
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

// backgroundParams returns the SGR parameters lipgloss emits for a background
// color. A gutter piece merges its own foreground into one sequence, so a test
// asks for the parameters rather than a standalone escape.
func backgroundParams(t *testing.T, c color.Color) string {
	t.Helper()
	rendered := lipgloss.NewStyle().Background(c).Render(" ")
	open, _, ok := strings.Cut(rendered, "m")
	if !ok {
		t.Fatalf("background render carried no SGR sequence: %q", rendered)
	}
	return strings.TrimPrefix(open, "\x1b[")
}

func cursorHighlightOutliner(t *testing.T, tasks ...*models.Task) (*outliner.OutlinerModel, *models.Project) {
	t.Helper()
	return cursorHighlightOutlinerTheme(t, "dark", tasks...)
}

func cursorHighlightOutlinerTheme(t *testing.T, themeName string, tasks ...*models.Task) (*outliner.OutlinerModel, *models.Project) {
	t.Helper()
	theme.ApplyTheme(theme.ThemeConfig{Name: themeName})
	t.Cleanup(func() { theme.ApplyTheme(theme.ThemeConfig{}) })

	project := renderFixtureProject(t, "project", "project", "Project", tasks)
	return outliner.NewOutlinerModel(project, nil, newTestConfig()), project
}

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

// withoutTheCursorBand strips the band's own arming sequences off a marked row,
// leaving the bytes the row drew. PaintBackground re-arms the band after every
// reset the content carries, so a run rendered one rune at a time — a search
// match, a struck-through canceled title — is no longer one contiguous string on
// the line even though every rune of it still carries its own style whole.
// Removing the arms is how a test asks about the run instead of about the band.
func withoutTheCursorBand(t *testing.T, line string) string {
	t.Helper()
	for _, bg := range []color.Color{theme.ColorCursorRowBg, theme.ColorFlashDimBg} {
		line = strings.ReplaceAll(line, "\x1b["+backgroundParams(t, bg)+"m", "")
	}
	return line
}

// TestCursorRowIsExactlyRowWidth holds the contract attachScrollbarAtWidth
// depends on: the marked row is neither padded past the scrollbar column nor
// left short of it, at every width and on every row type.
func TestCursorRowIsExactlyRowWidth(t *testing.T) {
	task := &models.Task{
		ID:            "t1",
		ShortID:       1,
		Title:         "Ship the release",
		WorkflowState: models.StateBacklog,
		Checklist:     []models.ChecklistItem{{Text: "Draft the notes"}},
	}
	outlinerView, project := cursorHighlightOutliner(t, task)
	taskRow := outliner.OutlinerRow{Task: task, ProjectID: project.ID, Prefix: "│   ├── "}
	rows := map[string]func(width int) string{
		"task": func(width int) string {
			return outlinerView.RenderOutlinerTaskRow(taskRow, true, width, 0, nil)
		},
		"checklist": func(width int) string {
			row := taskRow
			row.IsChecklistItem = true
			return outlinerView.RenderOutlinerChecklistRow(row, true, width, 0)
		},
		"section header": func(width int) string {
			return outlinerView.RenderOutlinerSectionHeaderRow(outliner.OutlinerRow{
				IsSectionHeader: true,
				SectionName:     "Backlog",
				SectionCount:    3,
			}, true, width, 0)
		},
		"project header": func(width int) string {
			return outlinerView.RenderOutlinerProjectHeaderRow(outliner.OutlinerRow{
				IsProjectHeader: true,
				ProjectID:       project.ID,
				SectionCount:    3,
			}, true, width, 0)
		},
	}

	for name, render := range rows {
		for _, width := range []int{4, 12, 24, 40, 80, 120} {
			if got := lipgloss.Width(render(width)); got != width {
				t.Fatalf("%s cursor row at width %d rendered %d columns", name, width, got)
			}
		}
	}
}

// TestCursorRowContentReachingTheEdgeGainsNoFill keeps the trailing fill honest:
// it only ever covers columns the row left blank, so a row whose content runs to
// the edge is not widened past it and spends no column on padding. The fill and
// the band are one color on a task row now, so the claim is read off the drawn
// text rather than off a background: the last column carries content.
func TestCursorRowContentReachingTheEdgeGainsNoFill(t *testing.T) {
	task := &models.Task{
		ID:      "t1",
		ShortID: 1,
		Title:   strings.Repeat("long-title-", 12),
	}
	outlinerView, project := cursorHighlightOutliner(t, task)
	row := outliner.OutlinerRow{Task: task, ProjectID: project.ID}

	line := outlinerView.RenderOutlinerTaskRow(row, true, 60, 0, nil)

	if got := lipgloss.Width(line); got != 60 {
		t.Fatalf("full-width cursor row rendered %d columns", got)
	}
	if plain := ansi.Strip(line); strings.HasSuffix(plain, " ") {
		t.Fatalf("a cursor row whose content fills the width still padded its last column:\n%q", plain)
	}
}

// cursorRowsByType renders the same fixture as each of the four Outliner row
// types, so an invariant about the mark is stated once against all of them.
func cursorRowsByType(outlinerView *outliner.OutlinerModel, project *models.Project, task *models.Task) map[string]string {
	checklistRow := outliner.OutlinerRow{Task: task, ProjectID: project.ID, IsChecklistItem: true}
	task.Checklist = []models.ChecklistItem{{Text: "Draft the notes"}}
	return map[string]string{
		"task": outlinerView.RenderOutlinerTaskRow(
			outliner.OutlinerRow{Task: task, ProjectID: project.ID}, true, 80, 0, nil),
		"checklist": outlinerView.RenderOutlinerChecklistRow(checklistRow, true, 80, 0),
		"section header": outlinerView.RenderOutlinerSectionHeaderRow(outliner.OutlinerRow{
			IsSectionHeader: true,
			SectionName:     "Backlog",
			SectionCount:    3,
		}, true, 80, 0),
		"project header": outlinerView.RenderOutlinerProjectHeaderRow(outliner.OutlinerRow{
			IsProjectHeader: true,
			ProjectID:       project.ID,
			SectionCount:    3,
		}, true, 80, 0),
	}
}

// TestCursorRowMarksItsBarAndTrail is the invariant itself: every Outliner row
// type says where the cursor is with a leading bar, a task row carries the band
// across the width behind it, and every other row type keeps the dim trailing
// fill it always had. Both themes are checked because `T` derives all three from
// the theme's own paper in opposite directions.
//
// The band is a task row's alone. A section header, a project header and a
// checklist row are not the current *task*, so tinting them would make the band
// mean "the cursor is here" — which is the bar's job — rather than "this is the
// work you are on".
func TestCursorRowMarksItsBarAndTrail(t *testing.T) {
	for _, themeName := range []string{"dark", "light"} {
		t.Run(themeName, func(t *testing.T) {
			task := &models.Task{ID: "t1", ShortID: 1, Title: "Ship", WorkflowState: models.StateBacklog}
			outlinerView, project := cursorHighlightOutlinerTheme(t, themeName, task)
			band := backgroundParams(t, theme.ColorCursorRowBg)
			trailBg := backgroundParams(t, theme.ColorSelectedDimBg)
			if band == trailBg {
				t.Fatalf("the %s theme derived one color for the band and the trail: %q", themeName, band)
			}

			for name, line := range cursorRowsByType(outlinerView, project, task) {
				if got := cursorMarks(line); got != 1 {
					t.Fatalf("%s cursor row carries %d bars, want 1:\n%q", name, got, line)
				}
				wantBand := name == "task"
				if got := strings.Contains(line, band); got != wantBand {
					t.Fatalf("%s cursor row band = %v, want %v:\n%q", name, got, wantBand, line)
				}
				if got := strings.Contains(line, trailBg); got == wantBand {
					t.Fatalf("%s cursor row trailing fill = %v, want %v:\n%q", name, got, !wantBand, line)
				}
			}
		})
	}
}

// cursorMarks counts the inverse-video sequences on a line, which is how many
// cursor bars it drew. The attribute is what makes the bar the one mark that
// survives a profile carrying no color, so counting it is also the one count that
// does not depend on which theme derived which hue.
func cursorMarks(line string) int {
	return strings.Count(line, "\x1b[7")
}

// TestCursorBarIsOneColumn holds the mark's whole shape. Only the row's leftmost
// column is inverted, and it is the same column on every row type — a blank
// margin everywhere but the section header, whose fold arrow it reverses.
// Painting a second column is what made the mark compete with the row it points
// at.
func TestCursorBarIsOneColumn(t *testing.T) {
	task := &models.Task{ID: "t1", ShortID: 1, Title: "Ship", WorkflowState: models.StateBacklog}
	outlinerView, project := cursorHighlightOutliner(t, task)
	column0 := map[string]string{
		"task":           theme.RenderCursorPrefix(),
		"checklist":      theme.RenderCursorPrefix(),
		"section header": theme.IconExpanded,
		"project header": " ",
	}

	for name, line := range cursorRowsByType(outlinerView, project, task) {
		bar := theme.CursorBarStyle.Render(column0[name])
		if got := lipgloss.Width(bar); got != 1 {
			t.Fatalf("%s cursor bar covers %d columns, want 1", name, got)
		}
		if !strings.HasPrefix(line, bar) {
			t.Fatalf("%s cursor row does not open with its bar:\n%q", name, line)
		}
		if got := cursorMarks(line); got != 1 {
			t.Fatalf("%s cursor row inverted %d columns, want 1:\n%q", name, got, line)
		}
	}
}

// TestCursorBarSurvivesARowWithoutColor is the monochrome half of the mark, one
// level up from the theme's own contract: the band, the badges and the deadline
// color all go when a terminal paints nothing, so the row is held to still saying
// where the cursor is — and to saying it only there.
func TestCursorBarSurvivesARowWithoutColor(t *testing.T) {
	task := &models.Task{ID: "t1", ShortID: 1, Title: "Ship", WorkflowState: models.StateBacklog}
	outlinerView, project := cursorHighlightOutliner(t, task)
	row := outliner.OutlinerRow{Task: task, ProjectID: project.ID}

	marked := uxClarityMonochrome(t, outlinerView.RenderOutlinerTaskRow(row, true, 80, 0, nil))
	ordinary := uxClarityMonochrome(t, outlinerView.RenderOutlinerTaskRow(row, false, 80, 0, nil))

	if got := strings.Count(marked, "\x1b[7m"); got != 1 {
		t.Fatalf("the marked row kept %d inverted columns without color, want 1:\n%q", got, marked)
	}
	if strings.Contains(ordinary, "\x1b[7m") {
		t.Fatalf("an ordinary row is inverted too, so the mark says nothing:\n%q", ordinary)
	}
	if plainRowText(marked) != plainRowText(ordinary) {
		t.Fatalf("the mark changed what the row says:\nmarked   %q\nordinary %q",
			plainRowText(marked), plainRowText(ordinary))
	}
}

// TestCursorRowStaysTellableFromAVisualSelection keeps the two axes apart. A row
// can be the one you are on, a member of an explicit selection, both, or
// neither, and the band and the mark are what say which: a selected row the
// cursor left wears its mark and no band, and the row the cursor is on wears the
// band whether or not it was selected. The mark takes the leading column and
// none of its own, so every one of those rows starts its title where an
// unselected row does.
func TestCursorRowStaysTellableFromAVisualSelection(t *testing.T) {
	selected := &models.Task{ID: "t1", ShortID: 1, Title: "Selected", WorkflowState: models.StateBacklog}
	elsewhere := &models.Task{ID: "t2", ShortID: 2, Title: "Elsewhere", WorkflowState: models.StateBacklog}
	outlinerView, project := cursorHighlightOutliner(t, selected, elsewhere)
	selectedRow := outliner.OutlinerRow{Task: selected, ProjectID: project.ID}
	cursorRow := outliner.OutlinerRow{Task: elsewhere, ProjectID: project.ID}
	before := ansi.Strip(outlinerView.RenderOutlinerTaskRow(selectedRow, false, 80, 0, nil))
	outlinerView.ReplaceSelection([]taskref.TaskIdentity{outlinerView.TaskIdentityForRow(selectedRow)})

	const mark = "✓"
	band := backgroundParams(t, theme.ColorCursorRowBg)
	left := outlinerView.RenderOutlinerTaskRow(selectedRow, false, 80, 0, nil)
	if strings.Contains(left, band) {
		t.Fatalf("a selected row the cursor left wears the band:\n%q", left)
	}
	if !strings.HasPrefix(ansi.Strip(left), mark) {
		t.Fatalf("a selected row the cursor left lost its mark:\n%q", ansi.Strip(left))
	}
	if strings.Contains(ansi.Strip(left), "[x]") {
		t.Fatalf("a selected row still draws a checkbox:\n%q", ansi.Strip(left))
	}
	if got, want := ansi.Strip(left), mark+strings.TrimPrefix(before, " "); got != want {
		t.Fatalf("selecting a row moved what it draws:\nbefore %q\nafter  %q", before, got)
	}

	under := outlinerView.RenderOutlinerTaskRow(cursorRow, true, 80, 0, nil)
	if !strings.Contains(under, band) {
		t.Fatalf("the cursor row wears no band while a selection stands:\n%q", under)
	}
	if strings.Contains(ansi.Strip(under), mark) {
		t.Fatalf("an unselected cursor row claims membership in the selection:\n%q", ansi.Strip(under))
	}

	both := outlinerView.RenderOutlinerTaskRow(selectedRow, true, 80, 0, nil)
	if !strings.Contains(both, band) || !strings.HasPrefix(both, theme.CursorBarStyle.Render(mark)) {
		t.Fatalf("a selected row under the cursor states only one of the two:\n%q", both)
	}
	if got := cursorMarks(both); got != 1 {
		t.Fatalf("a selected cursor row inverted %d columns, want 1:\n%q", got, both)
	}
}

// TestSelectionMarkReadsWithoutColor is why the mark is a glyph. The cursor's
// bar replaces the mark's own style, and a profile carrying no color drops what
// is left of it, so membership has to survive as text in both: the mark on a
// selected row, and a blank on every row that is not one.
func TestSelectionMarkReadsWithoutColor(t *testing.T) {
	selected := &models.Task{ID: "t1", ShortID: 1, Title: "Selected", WorkflowState: models.StateBacklog}
	elsewhere := &models.Task{ID: "t2", ShortID: 2, Title: "Elsewhere", WorkflowState: models.StateBacklog}
	outlinerView, project := cursorHighlightOutliner(t, selected, elsewhere)
	selectedRow := outliner.OutlinerRow{Task: selected, ProjectID: project.ID}
	otherRow := outliner.OutlinerRow{Task: elsewhere, ProjectID: project.ID}
	outlinerView.ReplaceSelection([]taskref.TaskIdentity{outlinerView.TaskIdentityForRow(selectedRow)})

	for _, cursor := range []bool{false, true} {
		member := ansi.Strip(outlinerView.RenderOutlinerTaskRow(selectedRow, cursor, 80, 0, nil))
		if !strings.HasPrefix(member, "✓") {
			t.Fatalf("cursor=%v: a selected row says so in color alone:\n%q", cursor, member)
		}
		other := ansi.Strip(outlinerView.RenderOutlinerTaskRow(otherRow, cursor, 80, 0, nil))
		if !strings.HasPrefix(other, " ") || strings.Contains(other, "✓") {
			t.Fatalf("cursor=%v: an unselected row draws something in the mark's column:\n%q", cursor, other)
		}
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

// TestCursorFlashWidensTheMark records what a pulse spends: the whole gutter
// floods and the band brightens with it, so a flash reads as more of the same
// mark instead of a different one. The bar keeps its own shape through it — it is
// the flash's colors under the same inverted column — which is what keeps the
// mark in one place while a pulse is running and on a terminal with no color.
func TestCursorFlashWidensTheMark(t *testing.T) {
	task := &models.Task{ID: "t1", ShortID: 1, Title: "Ship", WorkflowState: models.StateBacklog}
	outlinerView, project := cursorHighlightOutliner(t, task)
	outlinerView.CursorFlash = true
	prefix := "│   ├── "
	row := outliner.OutlinerRow{Task: task, ProjectID: project.ID, Prefix: prefix}

	line := outlinerView.RenderOutlinerTaskRow(row, true, 80, 0, nil)

	if !strings.HasPrefix(line, theme.CursorFlashBarStyle.Render(theme.RenderCursorPrefix())) {
		t.Fatalf("flashing cursor row does not open with its bar:\n%q", line)
	}
	if got := cursorMarks(line); got != 1 {
		t.Fatalf("flashing cursor row inverted %d columns, want 1:\n%q", got, line)
	}
	if !strings.Contains(line, theme.MutedStyle.Background(theme.ColorFlashBg).Render(prefix)) {
		t.Fatalf("flashing cursor row left its tree connectors unpainted:\n%q", line)
	}
	if !strings.Contains(line, backgroundParams(t, theme.ColorFlashDimBg)) {
		t.Fatalf("flashing cursor row kept the resting band:\n%q", line)
	}
	if strings.Contains(line, backgroundParams(t, theme.ColorCursorRowBg)) {
		t.Fatalf("flashing cursor row painted both bands at once:\n%q", line)
	}
	if strings.Contains(line, backgroundParams(t, theme.ColorSelectedDimBg)) {
		t.Fatalf("flashing cursor row painted the resting trailing fill:\n%q", line)
	}
}
