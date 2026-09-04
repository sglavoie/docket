package tui

import (
	"strings"
	"testing"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/outliner"
	"github.com/sglavoie/tt/internal/tui/taskref"
)

func rangeSelectionTestApp(t *testing.T) (*AppModel, *models.Project, []*models.Task) {
	t.Helper()
	app := newUITestApp()
	tasks := []*models.Task{
		{ID: "first", Title: "First", WorkflowState: models.StateBacklog},
		{ID: "second", Title: "Second", WorkflowState: models.StateBacklog},
		{ID: "third", Title: "Third", WorkflowState: models.StateBacklog},
	}
	project := &models.Project{ID: "project", Name: "Project", Tasks: tasks}
	project.BuildTaskTree()
	setTestAppProjects(app, project)
	app.crossOutliner.Population.Projects = []*models.Project{project}
	app.crossOutliner.Rows = []outliner.OutlinerRow{
		{Task: tasks[0], ProjectID: project.ID},
		{IsSectionHeader: true, SectionName: "header"},
		{Task: tasks[1], ProjectID: project.ID},
		{IsProjectHeader: true, ProjectID: project.ID},
		{Task: tasks[2], ProjectID: project.ID},
	}
	app.crossOutliner.Cursor = 0
	return app, project, tasks
}

func sendRangeSelectionKey(t *testing.T, app *AppModel, key string) {
	t.Helper()
	msg, err := parseUITestKey(key)
	if err != nil {
		t.Fatalf("parse key %q: %v", key, err)
	}
	handled, _ := app.handleGlobalKeyPress(msg)
	if !handled {
		t.Fatalf("key %q was not handled", key)
	}
}

func TestVimRangeSelectionSkipsHeadersAndFreezes(t *testing.T) {
	app, project, tasks := rangeSelectionTestApp(t)
	out := app.crossOutliner

	sendRangeSelectionKey(t, app, "V")
	if !out.IsRangeSelectionActive() || out.SelectedCount() != 1 {
		t.Fatalf("fresh range = active %v count %d, want active/1", out.IsRangeSelectionActive(), out.SelectedCount())
	}

	sendRangeSelectionKey(t, app, "j") // section header
	if out.Cursor != 1 || out.SelectedCount() != 1 {
		t.Fatalf("range on header = cursor %d count %d, want 1/1", out.Cursor, out.SelectedCount())
	}
	sendRangeSelectionKey(t, app, "j") // second task
	for _, task := range tasks[:2] {
		if !out.IsTaskIdentitySelected(taskref.For(task, project)) {
			t.Fatalf("range omitted task %s", task.ID)
		}
	}
	if out.SelectedCount() != 2 {
		t.Fatalf("inclusive range count = %d, want 2", out.SelectedCount())
	}

	sendRangeSelectionKey(t, app, "V")
	if out.IsRangeSelectionActive() || out.SelectedCount() != 2 {
		t.Fatalf("V freeze = active %v count %d, want frozen/2", out.IsRangeSelectionActive(), out.SelectedCount())
	}
	sendRangeSelectionKey(t, app, "j")
	if out.SelectedCount() != 2 {
		t.Fatalf("movement reshaped frozen range to %d tasks", out.SelectedCount())
	}

	// A new V starts a fresh range and replaces the prior selection.
	out.Cursor = 2
	sendRangeSelectionKey(t, app, "V")
	if !out.IsRangeSelectionActive() || out.SelectedCount() != 1 ||
		!out.IsTaskIdentitySelected(taskref.For(tasks[1], project)) {
		t.Fatalf("new range did not replace selection at second task: active=%v selected=%v", out.IsRangeSelectionActive(), out.SelectedTaskIdentities())
	}
	sendRangeSelectionKey(t, app, "esc")
	if out.IsRangeSelectionActive() || !out.MultiSelectMode || out.SelectedCount() != 1 {
		t.Fatalf("Esc freeze = range %v multi %v count %d", out.IsRangeSelectionActive(), out.MultiSelectMode, out.SelectedCount())
	}
}

func TestCtrlAReplacesSelectionWithVisibleTaskRows(t *testing.T) {
	app, project, tasks := rangeSelectionTestApp(t)
	out := app.crossOutliner
	hidden := taskref.TaskIdentity{ProjectID: "hidden-project", TaskID: "hidden-task"}
	out.MultiSelectMode = true
	out.SelectTaskIdentity(hidden)

	sendRangeSelectionKey(t, app, "ctrl+a")
	if out.IsRangeSelectionActive() || out.SelectedCount() != 3 {
		t.Fatalf("Ctrl+A = range %v count %d, want frozen/3", out.IsRangeSelectionActive(), out.SelectedCount())
	}
	if out.IsTaskIdentitySelected(hidden) {
		t.Fatal("Ctrl+A added to the old selection instead of replacing it")
	}
	for _, task := range tasks {
		if !out.IsTaskIdentitySelected(taskref.For(task, project)) {
			t.Fatalf("Ctrl+A omitted visible task %s", task.ID)
		}
	}
}

func TestOperationContextClassifiesHiddenSelectionAndBulkSheetNamesIt(t *testing.T) {
	app, project, tasks := rangeSelectionTestApp(t)
	out := app.crossOutliner
	out.Rows = out.Rows[:1]
	out.ReplaceSelection([]taskref.TaskIdentity{
		taskref.For(tasks[0], project),
		taskref.For(tasks[1], project),
	})

	context := app.operationCommandContext()
	if context.Selection.Count() != 2 || context.Selection.VisibleCount != 1 || context.Selection.HiddenCount != 1 {
		t.Fatalf("selection classification = total %d visible %d hidden %d", context.Selection.Count(), context.Selection.VisibleCount, context.Selection.HiddenCount)
	}
	if err := app.openTaskActionSheet(context); err != nil {
		t.Fatalf("open bulk action sheet: %v", err)
	}
	if app.contextActionSheet == nil || !strings.Contains(app.contextActionSheet.Config.Subtitle, "2 tasks · 1 visible · 1 hidden") {
		t.Fatalf("bulk subtitle = %q, want total/visible/hidden counts", app.contextActionSheet.Config.Subtitle)
	}

	// Delete confirmation must be calculated from the complete selection, not
	// just the one visible row.
	app.handleTaskDeleteRequested(TaskDeleteRequestedMsg{Refs: []TaskProjectRef{
		{Task: tasks[0], Project: project},
		{Task: tasks[1], Project: project},
	}})
	if got := app.taskMutation.deleteConfirm.Prompt(); got != "Delete 2 tasks?" {
		t.Fatalf("hidden-target delete prompt = %q, want complete two-target count", got)
	}
}

func TestEscAfterFrozenSelectionClearsOnSecondPress(t *testing.T) {
	app, _, _ := rangeSelectionTestApp(t)
	sendRangeSelectionKey(t, app, "V")
	sendRangeSelectionKey(t, app, "esc")
	sendRangeSelectionKey(t, app, "esc")
	if app.crossOutliner.MultiSelectMode || app.crossOutliner.SelectedCount() != 0 {
		t.Fatal("second Esc did not clear the frozen selection")
	}
}

func TestToggleSelectionBuildsNonContiguousSetWithoutARange(t *testing.T) {
	app, project, tasks := rangeSelectionTestApp(t)
	out := app.crossOutliner

	sendRangeSelectionKey(t, app, "v")
	for range 4 {
		sendRangeSelectionKey(t, app, "j")
	}
	sendRangeSelectionKey(t, app, "v")
	if out.IsRangeSelectionActive() {
		t.Fatal("v activated a range instead of toggling one row")
	}
	if out.SelectedCount() != 2 ||
		!out.IsTaskIdentitySelected(taskref.For(tasks[0], project)) ||
		!out.IsTaskIdentitySelected(taskref.For(tasks[2], project)) {
		t.Fatalf("toggled selection = %v, want the first and third tasks only", out.SelectedTaskIdentities())
	}
	if out.IsTaskIdentitySelected(taskref.For(tasks[1], project)) {
		t.Fatal("cursor movement between two toggles selected the task it passed over")
	}

	sendRangeSelectionKey(t, app, "v")
	if out.SelectedCount() != 1 || out.IsTaskIdentitySelected(taskref.For(tasks[2], project)) {
		t.Fatalf("second v on the same row = %v, want it deselected", out.SelectedTaskIdentities())
	}
	if !out.MultiSelectMode {
		t.Fatal("deselecting a row left multi-select mode; only Esc may leave it")
	}
}

func TestToggleDuringActiveRangeFreezesThenDropsTheCursorRow(t *testing.T) {
	app, project, tasks := rangeSelectionTestApp(t)
	out := app.crossOutliner

	sendRangeSelectionKey(t, app, "V")
	sendRangeSelectionKey(t, app, "j")
	sendRangeSelectionKey(t, app, "j")
	if !out.IsRangeSelectionActive() || out.SelectedCount() != 2 {
		t.Fatalf("range before toggle = active %v count %d, want active/2", out.IsRangeSelectionActive(), out.SelectedCount())
	}

	sendRangeSelectionKey(t, app, "v")
	if out.IsRangeSelectionActive() {
		t.Fatal("v did not freeze the live range")
	}
	if out.SelectedCount() != 1 || !out.IsTaskIdentitySelected(taskref.For(tasks[0], project)) {
		t.Fatalf("v over the range end = %v, want only the anchor task", out.SelectedTaskIdentities())
	}

	sendRangeSelectionKey(t, app, "j")
	if out.SelectedCount() != 1 {
		t.Fatalf("movement reshaped the frozen selection to %d tasks", out.SelectedCount())
	}
}
