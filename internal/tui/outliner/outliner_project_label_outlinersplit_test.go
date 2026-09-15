package outliner

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/chrome"
)

// labelledRow renders one task row plain, at width, from the projection the
// model is already in.
func labelledRow(t *testing.T, m *OutlinerModel, taskID string, width int) string {
	t.Helper()
	row, found := rowForTaskID(m, taskID)
	if !found {
		t.Fatalf("task %q has no row", taskID)
	}
	return ansi.Strip(m.RenderOutlinerTaskRow(row, false, width, 0, nil))
}

// TestCrossProjectRowNamesItsOwnProject is the whole of what replaced the
// heading: each row of a project group carries the project, so a lane holding
// two projects is readable without a row spent on either one.
func TestCrossProjectRowNamesItsOwnProject(t *testing.T) {
	fixture := newRowGroupFixture()
	outliner, _ := primaryRowsOutliner(fixture.alpha, fixture.beta)
	outliner.SelectOutlinerMode(chrome.ModeAll)

	for taskID, want := range map[string]string{
		fixture.alphaBacklog.ID: "[alpha]",
		fixture.alphaWaiting.ID: "[alpha]",
		fixture.betaBacklog.ID:  "[beta]",
	} {
		line := labelledRow(t, outliner, taskID, 80)
		if !strings.Contains(line, want) {
			t.Fatalf("row for %q = %q, want the label %q", taskID, line, want)
		}
		if index := strings.Index(line, want); index > strings.Index(line, taskID) {
			t.Fatalf("row for %q put the label after the title: %q", taskID, line)
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
}

// TestNestedRowsInheritTheProjectFromTheirConnectors keeps the label to the rows
// that answer a question the structure has not: a child seated under its own
// parent is silent, and the parent above it still names the project.
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
}

// TestProjectLabelIsBoundedAndReservedAtEveryWidth is the label's two width
// rules at once. A long project name is cut to projectLabelWidth rather than
// taking the row, and a narrow row keeps the label the indicator ladder is
// already shedding segments for, because a row whose project is unknown is a row
// whose title may mean two things.
func TestProjectLabelIsBoundedAndReservedAtEveryWidth(t *testing.T) {
	long := primaryRowsProject("a-very-long-project-slug-indeed", primaryRowsTask("long-task", models.StateBacklog))
	outliner, _ := primaryRowsOutliner(long, newRowGroupFixture().beta)
	outliner.SelectOutlinerMode(chrome.ModeAll)

	row, found := rowForTaskID(outliner, "long-task")
	if !found {
		t.Fatal("the long-slug task has no row")
	}
	label := outliner.taskProjectLabel(row)
	if width := lipgloss.Width(label); width != projectLabelWidth+1 {
		t.Fatalf("label %q is %d columns, want %d plus its separating space", label, width, projectLabelWidth)
	}
	if !strings.HasPrefix(ansi.Strip(label), "[a-very-long…]") {
		t.Fatalf("label %q dropped the head of the slug rather than its tail", ansi.Strip(label))
	}

	for _, width := range []int{80, 60, 40, 24} {
		line := labelledRow(t, outliner, "long-task", width)
		if !strings.Contains(line, "[a-very-long…]") {
			t.Fatalf("row at %d columns dropped the project label: %q", width, line)
		}
	}
}
