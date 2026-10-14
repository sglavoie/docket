package tui

import (
	"fmt"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/taskref"
)

// anchoredSheetTaskCount is more Inbox tasks than any frame these tests draw
// holds, so the cursor can sit near either end of the window.
const anchoredSheetTaskCount = 30

// anchoredSheetTestDriver draws an Outliner of one project holding
// anchoredSheetTaskCount Inbox tasks, so the cursor can be put on a row near either end of a frame of the
// given height.
func anchoredSheetTestDriver(t *testing.T, width, height int) (*UITestDriver, *models.Project) {
	t.Helper()
	app := newUITestApp()
	tasks := make([]*models.Task, 0, anchoredSheetTaskCount)
	for i := range anchoredSheetTaskCount {
		tasks = append(tasks, &models.Task{
			ID:            fmt.Sprintf("task-%02d", i),
			Title:         fmt.Sprintf("Anchor task %02d", i),
			WorkflowState: models.StateInbox,
		})
	}
	project := &models.Project{ID: "project", Name: "Project", Tasks: tasks}
	project.BuildTaskTree()
	setTestAppProjects(app, project)
	app.crossOutliner.Population.Projects = []*models.Project{project}
	driver := NewUITestDriver(app, width, height)
	driver.Render()
	return driver, project
}

// seatAnchoredSheetCursor puts the cursor on the named task's row and draws the
// frame once, so the Outliner has recorded the window the sheet reads.
func seatAnchoredSheetCursor(t *testing.T, driver *UITestDriver, title string) {
	t.Helper()
	out := driver.App().crossOutliner
	for i, row := range out.Rows {
		if row.Task != nil && row.Task.Title == title {
			out.Cursor = i
			driver.Render()
			return
		}
	}
	t.Fatalf("no row for %q", title)
}

func openAnchoredSheet(t *testing.T, driver *UITestDriver) {
	t.Helper()
	app := driver.App()
	if err := app.openTaskActionSheet(app.operationCommandContext()); err != nil {
		t.Fatalf("open task action sheet: %v", err)
	}
}

// sheetBox reports the frame rows the sheet's border spans, and the row the
// given title is drawn on outside the box.
func sheetBox(t *testing.T, frame, title string) (top, bottom, titleRow int) {
	t.Helper()
	top, bottom, titleRow = -1, -1, -1
	for i, line := range strings.Split(ansi.Strip(frame), "\n") {
		switch {
		case strings.Contains(line, "╭"):
			top = i
		case strings.Contains(line, "╰"):
			bottom = i
		case titleRow < 0 && strings.Contains(line, title) && !strings.Contains(line, "Task Actions"):
			titleRow = i
		}
	}
	if top < 0 || bottom < top {
		t.Fatalf("no sheet box in frame:\n%s", ansi.Strip(frame))
	}
	return top, bottom, titleRow
}

func TestTaskActionSheetHangsUnderACursorRowNearTheTop(t *testing.T) {
	driver, _ := anchoredSheetTestDriver(t, 100, 40)
	seatAnchoredSheetCursor(t, driver, "Anchor task 01")
	row, ok := driver.App().crossOutliner.CursorScreenRow()
	if !ok {
		t.Fatal("the Outliner named no cursor row after a render")
	}
	openAnchoredSheet(t, driver)

	frame := driver.Render()
	top, _, titleRow := sheetBox(t, frame, "Anchor task 01")
	if titleRow != row {
		t.Fatalf("cursor row drawn at %d, want %d, uncovered:\n%s", titleRow, row, ansi.Strip(frame))
	}
	if top <= row || top > row+2 {
		t.Fatalf("sheet border at row %d, want directly under cursor row %d:\n%s", top, row, ansi.Strip(frame))
	}
}

func TestTaskActionSheetRisesAboveACursorRowNearTheBottom(t *testing.T) {
	driver, _ := anchoredSheetTestDriver(t, 100, 40)
	seatAnchoredSheetCursor(t, driver, "Anchor task 29")
	row, ok := driver.App().crossOutliner.CursorScreenRow()
	if !ok {
		t.Fatal("the Outliner named no cursor row after a render")
	}
	openAnchoredSheet(t, driver)

	frame := driver.Render()
	_, bottom, titleRow := sheetBox(t, frame, "Anchor task 29")
	if titleRow != row {
		t.Fatalf("cursor row drawn at %d, want %d, uncovered:\n%s", titleRow, row, ansi.Strip(frame))
	}
	if bottom >= row || bottom < row-2 {
		t.Fatalf("sheet border ends at row %d, want directly above cursor row %d:\n%s", bottom, row, ansi.Strip(frame))
	}
	if !strings.Contains(ansi.Strip(frame), "Active Projects") {
		t.Fatalf("the sheet covered the title row:\n%s", ansi.Strip(frame))
	}
}

func TestTaskActionSheetStaysWholeInsideAShortFrame(t *testing.T) {
	driver, _ := anchoredSheetTestDriver(t, 80, 20)
	seatAnchoredSheetCursor(t, driver, "Anchor task 04")
	openAnchoredSheet(t, driver)

	frame := driver.Render()
	top, bottom, _ := sheetBox(t, frame, "Anchor task 04")
	lines := strings.Split(ansi.Strip(frame), "\n")
	if top < 1 || !strings.Contains(lines[0], "Active Projects") {
		t.Fatalf("the sheet covered the title row:\n%s", ansi.Strip(frame))
	}
	box := strings.Join(lines[top:bottom+1], "\n")
	for _, want := range []string{"SCHEDULE", "Set Check-In", "DANGER", "Delete"} {
		if !strings.Contains(box, want) {
			t.Fatalf("short-frame sheet lost %q:\n%s", want, ansi.Strip(frame))
		}
	}
}

func TestBulkTaskActionSheetAnchorsToTheCursorRow(t *testing.T) {
	driver, project := anchoredSheetTestDriver(t, 100, 40)
	seatAnchoredSheetCursor(t, driver, "Anchor task 02")
	out := driver.App().crossOutliner
	out.MultiSelectMode = true
	out.ReplaceSelection([]taskref.TaskIdentity{
		taskref.For(project.Tasks[1], project),
		taskref.For(project.Tasks[2], project),
	})
	// The selection adds a context row, so the cursor row is read off the frame
	// that draws it.
	driver.Render()
	row, ok := out.CursorScreenRow()
	if !ok {
		t.Fatal("the Outliner named no cursor row after a render")
	}
	openAnchoredSheet(t, driver)

	frame := driver.Render()
	if !strings.Contains(ansi.Strip(frame), "2 tasks") {
		t.Fatalf("bulk sheet did not open:\n%s", ansi.Strip(frame))
	}
	top, _, _ := sheetBox(t, frame, "Anchor task 02")
	if top <= row || top > row+2 {
		t.Fatalf("bulk sheet border at row %d, want directly under cursor row %d:\n%s", top, row, ansi.Strip(frame))
	}
}

func TestTaskActionSheetFallsBackToBottomWithoutAnOutlinerRow(t *testing.T) {
	driver, _ := anchoredSheetTestDriver(t, 100, 40)
	seatAnchoredSheetCursor(t, driver, "Anchor task 01")
	openAnchoredSheet(t, driver)
	// The project sheet mounted in its place is not the one the row raised.
	app := driver.App()
	app.operations.RowSheet = nil

	frame := driver.Render()
	_, bottom, _ := sheetBox(t, frame, "Anchor task 01")
	if bottom < 30 {
		t.Fatalf("sheet with no anchor ends at row %d, want bottom of frame:\n%s", bottom, ansi.Strip(frame))
	}
}
