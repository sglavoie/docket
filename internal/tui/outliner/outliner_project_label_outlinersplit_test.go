package outliner

import (
	"strings"
	"testing"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/chrome"
)

// labelledRow renders one task row plain, at width, from the projection the
// model is already in and through that projection's own measured geometry. The
// geometry is the point: a row rendered against nothing has a label beside its
// title, and a row rendered against the projection has one in a column.
func labelledRow(t *testing.T, m *OutlinerModel, taskID string, width int) string {
	t.Helper()
	row, found := rowForTaskID(m, taskID)
	if !found {
		t.Fatalf("task %q has no row", taskID)
	}
	return ansi.Strip(m.RenderOutlinerTaskRow(row, false, width, 0, m.ComputeOutlinerRowLayout(m.Rows, width)))
}

// labelColumn is the display column a rendered row seats a fragment in. The
// connectors, the state badge and the ellipsis are all multi-byte, so a byte
// offset into the line is not the column the terminal draws it in, and a column
// is what every alignment claim here is about.
func labelColumn(t *testing.T, line, fragment string) int {
	t.Helper()
	index := strings.Index(line, fragment)
	if index < 0 {
		t.Fatalf("row %q carries no %q", line, fragment)
	}
	return lipgloss.Width(line[:index])
}

// TestCrossProjectRowNamesItsOwnProjectAfterItsTitle is the whole of what
// replaced the heading: each row of a project group carries the project, so a
// lane holding two projects is readable without a row spent on either one. The
// label follows the title and every row seats it in one column, which is what
// makes the titles scannable as a list and the projects scannable as a column
// rather than the two interleaving down the lane.
func TestCrossProjectRowNamesItsOwnProjectAfterItsTitle(t *testing.T) {
	fixture := newRowGroupFixture()
	outliner, _ := primaryRowsOutliner(fixture.alpha, fixture.beta)
	outliner.SelectOutlinerMode(chrome.ModeAll)

	columns := map[string]int{}
	for taskID, want := range map[string]string{
		fixture.alphaBacklog.ID: "[alpha]",
		fixture.alphaWaiting.ID: "[alpha]",
		fixture.betaBacklog.ID:  "[beta]",
	} {
		line := labelledRow(t, outliner, taskID, 80)
		label := strings.Index(line, want)
		if label < 0 {
			t.Fatalf("row for %q = %q, want the label %q", taskID, line, want)
		}
		if title := strings.Index(line, taskID); label < title {
			t.Fatalf("row for %q put the label before the title: %q", taskID, line)
		}
		columns[taskID] = labelColumn(t, line, want)
	}

	distinct := map[int]bool{}
	for _, column := range columns {
		distinct[column] = true
	}
	if len(distinct) != 1 {
		t.Fatalf("the labels start in %d different columns, so they are segments rather than a column: %v", len(distinct), columns)
	}
}

// TestProjectColumnHoldsStillWhileTheListScrolls is the column's other half.
// It is measured over the whole projection rather than over the rows a window
// holds, so every window of that projection puts the label in the same column;
// a measurement taken from the window would move the label — and every title
// with it — each time a longer one scrolled into view.
func TestProjectColumnHoldsStillWhileTheListScrolls(t *testing.T) {
	short := primaryRowsTask("s", models.StateBacklog)
	long := primaryRowsTask("a-considerably-longer-task-title", models.StateBacklog)
	alpha := primaryRowsProject("alpha", short, long)
	outliner, _ := primaryRowsOutliner(alpha, newRowGroupFixture().beta)
	outliner.SelectOutlinerMode(chrome.ModeAll)

	whole := outliner.ComputeOutlinerRowLayout(outliner.Rows, 80)
	if whole == nil || whole.ProjectColumn == 0 {
		t.Fatalf("the projection measured no project column: %+v", whole)
	}
	for index := range outliner.Rows {
		window := outliner.ComputeOutlinerRowLayout(outliner.Rows[index:], 80)
		if window == nil {
			continue
		}
		if window.ProjectColumn != 0 && window.ProjectColumn > whole.ProjectColumn {
			t.Fatalf("a window starting at row %d wants column %d, past the projection's %d",
				index, window.ProjectColumn, whole.ProjectColumn)
		}
	}

	row, found := rowForTaskID(outliner, "s")
	if !found {
		t.Fatal("the short task has no row")
	}
	line := ansi.Strip(outliner.RenderOutlinerTaskRow(row, false, 80, 0, whole))
	if column := labelColumn(t, line, "[alpha]"); column != whole.ProjectColumn+1 {
		t.Fatalf("the short row seats its label in column %d rather than the measured column %d: %q",
			column, whole.ProjectColumn+1, line)
	}
}

func TestProjectColumnAlignsRowsWithDifferentIndicators(t *testing.T) {
	long := primaryRowsTask("long", models.StateBacklog)
	long.Title = strings.Repeat("long title ", 20)
	review := primaryRowsTask("review", models.StateBacklog)
	today := time.Now()
	review.NextCheckIn = &today
	due := primaryRowsTask("due", models.StateBacklog)
	due.DueDate = &today
	alpha := primaryRowsProject("katana", long, review)
	beta := primaryRowsProject("analytics", due)
	outliner, _ := primaryRowsOutliner(alpha, beta)
	outliner.SelectOutlinerMode(chrome.ModeAll)

	for _, width := range []int{60, 80, 120} {
		layout := outliner.ComputeOutlinerRowLayout(outliner.Rows, width)
		for _, task := range []*models.Task{long, review, due} {
			row, found := rowForTaskID(outliner, task.ID)
			if !found {
				t.Fatalf("task %q has no row", task.ID)
			}
			for _, selected := range []bool{false, true} {
				line := ansi.Strip(outliner.RenderOutlinerTaskRow(row, selected, width, 0, layout))
				label := ansi.Strip(outliner.taskProjectLabel(row, width))
				if column := labelColumn(t, line, label); column != layout.ProjectColumn+1 {
					t.Fatalf("width %d, task %q: label in column %d, want %d: %q",
						width, task.ID, column, layout.ProjectColumn+1, line)
				}
				if task == review && !strings.Contains(line, "review today") {
					t.Fatalf("width %d lost the review badge: %q", width, line)
				}
				if lipgloss.Width(line) > width {
					t.Fatalf("row width = %d, exceeds %d: %q", lipgloss.Width(line), width, line)
				}
			}
		}
	}
}

// TestSingleProjectFormsOmitTheProjectLabel is the redundancy rule: both forms
// that already name their project in the chrome are exactly the forms that
// carry no project half on their rows, so neither repeats it per row.
func TestSingleProjectFormsOmitTheProjectLabel(t *testing.T) {
	fixture := newRowGroupFixture()

	scoped, _ := primaryRowsOutliner(fixture.alpha, fixture.beta)
	scoped.PushProjectScope(fixture.alpha.ID)
	scoped.SelectOutlinerMode(chrome.ModeAll)
	if line := labelledRow(t, scoped, fixture.alphaBacklog.ID, 80); strings.Contains(line, "[alpha]") {
		t.Fatalf("the project-scoped row repeated its scope: %q", line)
	}

	single, _ := primaryRowsOutliner(fixture.alpha)
	single.Project = fixture.alpha
	single.SelectOutlinerMode(chrome.ModeAll)
	if line := labelledRow(t, single, fixture.alphaBacklog.ID, 80); strings.Contains(line, "[alpha]") {
		t.Fatalf("the single-project row repeated its project: %q", line)
	}
	if layout := single.ComputeOutlinerRowLayout(single.Rows, 80); layout != nil && layout.ProjectColumn != 0 {
		t.Fatalf("a projection naming no project still measured a column: %+v", layout)
	}
}

// TestNestedRowsInheritTheProjectFromTheirConnectors keeps the label to the rows
// that answer a question the structure has not: a child seated under its own
// parent is silent, and the parent above it still names the project. The column
// stays reserved on that child, because the indicators sit after it and a
// dropped field would leave them ragged down the lane.
func TestNestedRowsInheritTheProjectFromTheirConnectors(t *testing.T) {
	parent := primaryRowsTask("alpha-parent", models.StateBacklog)
	child := primaryRowsTask("alpha-child", models.StateBacklog)
	child.ParentID = &parent.ID
	alpha := primaryRowsProject("alpha", parent, child)
	outliner, _ := primaryRowsOutliner(alpha, newRowGroupFixture().beta)
	outliner.SelectOutlinerMode(chrome.ModeAll)

	if line := labelledRow(t, outliner, parent.ID, 80); !strings.Contains(line, "[alpha]") {
		t.Fatalf("the parent row lost its label: %q", line)
	}
	if line := labelledRow(t, outliner, child.ID, 80); strings.Contains(line, "[alpha]") {
		t.Fatalf("the child row restated the project its connectors already give it: %q", line)
	}

	layout := outliner.ComputeOutlinerRowLayout(outliner.Rows, 80)
	childRow, found := rowForTaskID(outliner, child.ID)
	if !found {
		t.Fatal("the child task has no row")
	}
	segments := outliner.buildTaskRowSegments(childRow, 80, layout, cursorPaint{})
	if width := lipgloss.Width(segments.projectField); width != layout.ProjectWidth+1 {
		t.Fatalf("the silent child reserved %d columns, want the column's %d", width, layout.ProjectWidth+1)
	}
	if strings.TrimSpace(ansi.Strip(segments.projectField)) != "" {
		t.Fatalf("the silent child wrote into the column it only reserves: %q", ansi.Strip(segments.projectField))
	}
}

// TestProjectLabelGrowsWithTheRowAndIsReservedAtEveryWidth is the label's width
// rules at once. A long project name is cut to the row's own ceiling rather
// than taking the row; that ceiling rises with the row, because a wide terminal
// is where a truncated slug is least excusable and where the title can most
// afford the columns; and the label survives every width the indicator ladder
// is already shedding segments at, because a row whose project is unknown is a
// row whose title may mean two things.
func TestProjectLabelGrowsWithTheRowAndIsReservedAtEveryWidth(t *testing.T) {
	long := primaryRowsProject("a-very-long-project-slug-indeed", primaryRowsTask("long-task", models.StateBacklog))
	outliner, _ := primaryRowsOutliner(long, newRowGroupFixture().beta)
	outliner.SelectOutlinerMode(chrome.ModeAll)

	row, found := rowForTaskID(outliner, "long-task")
	if !found {
		t.Fatal("the long-slug task has no row")
	}

	for _, width := range []int{24, 40, 60, 80, 120, 400} {
		label := ansi.Strip(outliner.taskProjectLabel(row, width))
		if got := lipgloss.Width(label); got != projectLabelBudget(width) {
			t.Fatalf("label %q at %d columns is %d wide, want the budget %d", label, width, got, projectLabelBudget(width))
		}
		if !strings.HasPrefix(label, "[a-very-long") || !strings.HasSuffix(label, "…]") {
			t.Fatalf("label %q at %d columns dropped the head of the slug rather than its tail", label, width)
		}
	}

	if got := projectLabelBudget(chrome.MinTerminalWidth); got != projectLabelWidth {
		t.Fatalf("the budget at the app's own floor is %d, want %d", got, projectLabelWidth)
	}
	if got := projectLabelBudget(projectLabelGrowthFloorWidth); got != projectLabelWidth {
		t.Fatalf("an ordinary %d-column row spends %d on the label, want the floor %d",
			projectLabelGrowthFloorWidth, got, projectLabelWidth)
	}
	if projectLabelBudget(120) <= projectLabelBudget(chrome.MinTerminalWidth) {
		t.Fatal("a wide row buys the label no columns at all")
	}
	if got := projectLabelBudget(4000); got != projectLabelCeilingWidth {
		t.Fatalf("the budget is unbounded at %d, want the ceiling %d", got, projectLabelCeilingWidth)
	}

	for _, width := range []int{80, 60, 40, 24} {
		line := labelledRow(t, outliner, "long-task", width)
		if !strings.Contains(line, "[a-very-long") {
			t.Fatalf("row at %d columns dropped the project label: %q", width, line)
		}
	}
}
