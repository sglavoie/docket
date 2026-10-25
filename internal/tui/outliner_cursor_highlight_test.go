package tui

import (
	"image/color"
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/tui/outliner"
	"github.com/sglavoie/docket/internal/tui/taskref"
	"github.com/sglavoie/docket/internal/tui/theme"
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
