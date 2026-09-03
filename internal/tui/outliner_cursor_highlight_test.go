package tui

import (
	"image/color"
	"strings"
	"testing"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/tt/internal/models"
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

func cursorHighlightOutliner(t *testing.T, tasks ...*models.Task) (*OutlinerModel, *models.Project) {
	t.Helper()
	return cursorHighlightOutlinerTheme(t, "dark", tasks...)
}

func cursorHighlightOutlinerTheme(t *testing.T, themeName string, tasks ...*models.Task) (*OutlinerModel, *models.Project) {
	t.Helper()
	theme.ApplyTheme(theme.ThemeConfig{Name: themeName})
	t.Cleanup(func() { theme.ApplyTheme(theme.ThemeConfig{}) })

	project := renderFixtureProject(t, "project", "project", "Project", tasks)
	return NewOutlinerModel(project, nil, newTestConfig()), project
}

// TestCursorTaskRowKeepsEveryColorItEarned is the inverse of what SelectedLine
// guarantees: the row under the cursor is the one row that must still say what
// it is, so its chip, badge and priority title survive intact.
func TestCursorTaskRowKeepsEveryColorItEarned(t *testing.T) {
	task := &models.Task{
		ID:            "t1",
		ShortID:       1,
		Title:         "Ship the release",
		Priority:      2,
		Tags:          []string{"infra"},
		WorkflowState: models.StateDoing,
	}
	outliner, project := cursorHighlightOutliner(t, task)
	row := outlinerRow{task: task, ProjectID: project.ID}

	line := outliner.renderOutlinerTaskRow(row, true, 80, 80, 0, nil)

	for name, want := range map[string]string{
		"tag chip":    theme.TagChipStyle("infra").Render("#infra"),
		"state badge": theme.GetStateBadge(models.StateDoing),
		"title":       theme.GetTitlePriorityStyle(2).Render(task.Title),
	} {
		if !strings.Contains(line, want) {
			t.Fatalf("cursor row lost its %s:\n%q", name, line)
		}
	}
	if strings.Contains(line, theme.SelectedTaskTitleStyle.Render(task.Title)) {
		t.Fatalf("cursor row repainted its title with the selection style:\n%q", line)
	}
}

func TestCursorRowKeepsItsSearchMatchHighlight(t *testing.T) {
	task := &models.Task{ID: "t1", ShortID: 1, Title: "Ship the release", Priority: 3}
	outliner, project := cursorHighlightOutliner(t, task)
	outliner.SetSearchQuery("release")
	row := outlinerRow{task: task, ProjectID: project.ID}

	line := outliner.renderOutlinerTaskRow(row, true, 80, 80, 0, nil)

	positions := query.SearchSubstringPositions(task.Title, "release", false)
	if len(positions) == 0 {
		t.Fatal("fixture query matched nothing")
	}
	want := textfit.HighlightText(task.Title, positions, theme.GetTitlePriorityStyle(3))
	if !strings.Contains(line, want) {
		t.Fatalf("cursor row dropped its search-match highlight:\n%q", line)
	}
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
	outliner, project := cursorHighlightOutliner(t, task)
	taskRow := outlinerRow{task: task, ProjectID: project.ID, prefix: "│   ├── "}
	rows := map[string]func(width int) string{
		"task": func(width int) string {
			return outliner.renderOutlinerTaskRow(taskRow, true, width, width, 0, nil)
		},
		"checklist": func(width int) string {
			row := taskRow
			row.IsChecklistItem = true
			return outliner.renderOutlinerChecklistRow(row, true, width, 0)
		},
		"section header": func(width int) string {
			return outliner.renderOutlinerSectionHeaderRow(outlinerRow{
				IsSectionHeader: true,
				SectionName:     "Backlog",
				SectionCount:    3,
			}, true, width, 0)
		},
		"project header": func(width int) string {
			return outliner.renderOutlinerProjectHeaderRow(outlinerRow{
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
// the edge is not widened and wears no dim tint at all.
func TestCursorRowContentReachingTheEdgeGainsNoFill(t *testing.T) {
	task := &models.Task{
		ID:      "t1",
		ShortID: 1,
		Title:   strings.Repeat("long title ", 12),
	}
	outliner, project := cursorHighlightOutliner(t, task)
	row := outlinerRow{task: task, ProjectID: project.ID}

	line := outliner.renderOutlinerTaskRow(row, true, 60, 60, 0, nil)

	if got := lipgloss.Width(line); got != 60 {
		t.Fatalf("full-width cursor row rendered %d columns", got)
	}
	if strings.Contains(line, backgroundParams(t, theme.ColorSelectedDimBg)) {
		t.Fatalf("a cursor row whose content fills the width still painted a trailing fill:\n%q", line)
	}
}

// cursorRowsByType renders the same fixture as each of the four Outliner row
// types, so an invariant about the mark is stated once against all of them.
func cursorRowsByType(outliner *OutlinerModel, project *models.Project, task *models.Task) map[string]string {
	checklistRow := outlinerRow{task: task, ProjectID: project.ID, IsChecklistItem: true}
	task.Checklist = []models.ChecklistItem{{Text: "Draft the notes"}}
	return map[string]string{
		"task": outliner.renderOutlinerTaskRow(
			outlinerRow{task: task, ProjectID: project.ID}, true, 80, 80, 0, nil),
		"checklist": outliner.renderOutlinerChecklistRow(checklistRow, true, 80, 0),
		"section header": outliner.renderOutlinerSectionHeaderRow(outlinerRow{
			IsSectionHeader: true,
			SectionName:     "Backlog",
			SectionCount:    3,
		}, true, 80, 0),
		"project header": outliner.renderOutlinerProjectHeaderRow(outlinerRow{
			IsProjectHeader: true,
			ProjectID:       project.ID,
			SectionCount:    3,
		}, true, 80, 0),
	}
}

// TestCursorRowMarksItsBarAndTrail is the invariant itself: every Outliner row
// type says where the cursor is with a bright leading bar and a dim trailing
// fill, and never by repainting what sits between them. Both themes are checked
// because `T` derives the pair from the theme's own paper in opposite directions.
func TestCursorRowMarksItsBarAndTrail(t *testing.T) {
	for _, themeName := range []string{"dark", "light"} {
		t.Run(themeName, func(t *testing.T) {
			task := &models.Task{ID: "t1", ShortID: 1, Title: "Ship", WorkflowState: models.StateBacklog}
			outliner, project := cursorHighlightOutlinerTheme(t, themeName, task)
			barBg := backgroundParams(t, theme.ColorCursorBarBg)
			trailBg := backgroundParams(t, theme.ColorSelectedDimBg)
			if barBg == trailBg {
				t.Fatalf("the %s theme derived one color for both cursor regions: %q", themeName, barBg)
			}

			for name, line := range cursorRowsByType(outliner, project, task) {
				if !strings.Contains(line, barBg) {
					t.Fatalf("%s cursor row did not paint its bar:\n%q", name, line)
				}
				if !strings.Contains(line, trailBg) {
					t.Fatalf("%s cursor row did not paint its trailing fill:\n%q", name, line)
				}
			}
		})
	}
}

// TestCursorBarIsOneColumn holds the mark's whole shape. Only the row's leftmost
// column is bright, and it is the same column on every row type — a blank margin
// everywhere but the section header, whose fold arrow it reverses. Painting a
// second column is what made the mark compete with the row it points at.
func TestCursorBarIsOneColumn(t *testing.T) {
	task := &models.Task{ID: "t1", ShortID: 1, Title: "Ship", WorkflowState: models.StateBacklog}
	outliner, project := cursorHighlightOutliner(t, task)
	barBg := backgroundParams(t, theme.ColorCursorBarBg)
	column0 := map[string]string{
		"task":           theme.RenderCursorPrefix(),
		"checklist":      theme.RenderCursorPrefix(),
		"section header": theme.IconExpanded,
		"project header": " ",
	}

	for name, line := range cursorRowsByType(outliner, project, task) {
		bar := theme.CursorBarStyle.Render(column0[name])
		if got := lipgloss.Width(bar); got != 1 {
			t.Fatalf("%s cursor bar covers %d columns, want 1", name, got)
		}
		if !strings.HasPrefix(line, bar) {
			t.Fatalf("%s cursor row does not open with its bar:\n%q", name, line)
		}
		if got := strings.Count(line, barBg); got != 1 {
			t.Fatalf("%s cursor row painted the bar color in %d places, want 1:\n%q", name, got, line)
		}
	}
}

// TestCursorRowLeavesTheRestOfItsGutterOrdinary is the other half of the shape:
// the connectors, fold arrow and multi-select box past column 0 keep the colors
// they wear on every other row, so only the bar moves with the cursor.
func TestCursorRowLeavesTheRestOfItsGutterOrdinary(t *testing.T) {
	task := &models.Task{ID: "t1", ShortID: 1, Title: "Ship", WorkflowState: models.StateBacklog}
	outliner, project := cursorHighlightOutliner(t, task)
	prefix := "│   ├── "
	row := outlinerRow{task: task, ProjectID: project.ID, prefix: prefix}
	outliner.ReplaceSelection([]taskref.TaskIdentity{outliner.taskIdentityForRow(row)})

	line := outliner.renderOutlinerTaskRow(row, true, 80, 80, 0, nil)

	for name, want := range map[string]string{
		"multi-select box":  theme.SelectionCheckStyle.Render("[x]"),
		"tree connectors":   theme.MutedStyle.Render(prefix),
		"unpainted spacing": theme.SelectionCheckStyle.Render("[x]") + " " + theme.MutedStyle.Render(prefix),
	} {
		if !strings.Contains(line, want) {
			t.Fatalf("cursor row repainted its %s:\n%q", name, line)
		}
	}
	if !strings.Contains(ansi.Strip(line), "[x]") {
		t.Fatalf("selected cursor row lost its [x] entirely:\n%q", ansi.Strip(line))
	}
}

// TestCursorRowKeepsItsRowTint records the improvement the seam buys: the cursor
// owns the bar and the fill, not the content, so a waiting or overdue row is
// still visibly waiting or overdue while you stand on it.
func TestCursorRowKeepsItsRowTint(t *testing.T) {
	overdue := time.Now().AddDate(0, 0, -3)
	cases := map[string]struct {
		task *models.Task
		tint lipgloss.Style
	}{
		"waiting": {
			task: &models.Task{
				ID: "waiting", ShortID: 1, Title: "Blocked on review",
				WorkflowState: models.StateWaiting, StateChangedAt: time.Now().AddDate(0, 0, -2),
			},
			tint: theme.WaitingRowStyle,
		},
		"overdue": {
			task: &models.Task{
				ID: "overdue", ShortID: 2, Title: "Past due", DueDate: &overdue,
				WorkflowState: models.StateBacklog,
			},
			tint: theme.OverdueRowStyle,
		},
	}

	for name, tc := range cases {
		outliner, project := cursorHighlightOutliner(t, tc.task)
		row := outlinerRow{task: tc.task, ProjectID: project.ID}
		line := outliner.renderOutlinerTaskRow(row, true, 80, 80, 0, nil)

		if !strings.Contains(line, backgroundParams(t, tc.tint.GetBackground())) {
			t.Fatalf("%s cursor row dropped its row tint:\n%q", name, line)
		}
		if !strings.Contains(line, backgroundParams(t, theme.ColorCursorBarBg)) {
			t.Fatalf("%s cursor row dropped its bar:\n%q", name, line)
		}
	}
}

// TestCursorFlashWidensTheMark records what a pulse spends: the whole gutter
// floods rather than the bar changing hue, so a flash reads as more of the same
// mark instead of a different one.
func TestCursorFlashWidensTheMark(t *testing.T) {
	task := &models.Task{ID: "t1", ShortID: 1, Title: "Ship", WorkflowState: models.StateBacklog}
	outliner, project := cursorHighlightOutliner(t, task)
	outliner.cursorFlash = true
	prefix := "│   ├── "
	row := outlinerRow{task: task, ProjectID: project.ID, prefix: prefix}

	line := outliner.renderOutlinerTaskRow(row, true, 80, 80, 0, nil)

	flashBg := backgroundParams(t, theme.ColorFlashBg)
	if got := strings.Count(line, flashBg); got < 2 {
		t.Fatalf("flashing cursor row painted %d gutter pieces, want the bar and the columns past it:\n%q", got, line)
	}
	if !strings.Contains(line, theme.MutedStyle.Background(theme.ColorFlashBg).Render(prefix)) {
		t.Fatalf("flashing cursor row left its tree connectors unpainted:\n%q", line)
	}
	if !strings.Contains(line, backgroundParams(t, theme.ColorFlashDimBg)) {
		t.Fatalf("flashing cursor row kept the resting trailing fill:\n%q", line)
	}
	if strings.Contains(line, backgroundParams(t, theme.ColorCursorBarBg)) {
		t.Fatalf("flashing cursor row painted the resting bar color too:\n%q", line)
	}
	if strings.Contains(line, backgroundParams(t, theme.ColorSelectedDimBg)) {
		t.Fatalf("flashing cursor row painted both trailing fills at once:\n%q", line)
	}
}
