package tui

import (
	"slices"
	"testing"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/operation"
	"github.com/sglavoie/tt/internal/tui/taskref"
)

func TestGoParentTaskChordMovesTheCursorToTheActualParent(t *testing.T) {
	root := primaryRowsTask("root", models.StateBacklog)
	child := primaryRowsTask("child", models.StateBacklog)
	child.ParentID = &root.ID
	grandchild := primaryRowsTask("grandchild", models.StateBacklog)
	grandchild.ParentID = &child.ID
	project := primaryRowsProject("project", root, child, grandchild)
	app := newUITestApp()
	setTestAppProjects(app, project)
	out := app.crossOutliner
	out.Population.Projects = []*models.Project{project}
	out.Population.Invalidate()
	out.CrossProjectRebuildRows()
	identity := func(id string) taskref.TaskIdentity {
		return taskref.TaskIdentity{ProjectID: project.ID, TaskID: id}
	}
	if !out.MoveCursorToIdentity(identity(grandchild.ID)) {
		t.Fatal("grandchild row is unavailable")
	}
	ctx := app.operationCommandContext()
	binding, _, resolution := app.operations.Registry.InputBindingForSequence(
		operation.InputStrategySimplified, []string{"g", "P"}, ctx,
	)
	if resolution != operation.ChordTerminal || binding.OperationID != operation.OperationCoreGotoParentTask {
		t.Fatalf("gP resolved %s (%v), want Go to Parent Task", binding.OperationID, resolution)
	}
	if !slices.ContainsFunc(app.operations.Registry.HelpItemsForInputStrategy(operation.InputStrategySimplified, ctx), func(item operation.HelpItem) bool {
		return item.Key == "gP" && item.Description == "Go to Parent Task"
	}) {
		t.Fatal("generated help omits gP Go to Parent Task")
	}
	result := executeDirectPath(t, app, ctx, []string{"g", "P"})
	if result.Err != nil || result.Status != "" {
		t.Fatalf("gP = status %q err %v", result.Status, result.Err)
	}
	if got := out.CurrentTaskIdentity(); got != identity(child.ID) {
		t.Fatalf("cursor = %+v, want immediate parent", got)
	}
	if !out.GoToParentTaskAtCursor() || out.CurrentTaskIdentity() != identity(root.ID) {
		t.Fatal("second parent jump did not reach the root")
	}
	if out.GoToParentTaskAtCursor() {
		t.Fatal("root task has no parent to jump to")
	}
}

func TestGoParentTaskLeavesOnlyTheFocusFrameHidingItsParent(t *testing.T) {
	root := primaryRowsTask("root", models.StateBacklog)
	child := primaryRowsTask("child", models.StateBacklog)
	child.ParentID = &root.ID
	grandchild := primaryRowsTask("grandchild", models.StateBacklog)
	grandchild.ParentID = &child.ID
	project := primaryRowsProject("project", root, child, grandchild)
	out := scopeTestOutliner(project)
	identity := func(id string) taskref.TaskIdentity {
		return taskref.TaskIdentity{ProjectID: project.ID, TaskID: id}
	}
	for _, id := range []string{root.ID, child.ID} {
		if !out.MoveCursorToIdentity(identity(id)) || !out.FocusSubtreeAtCursor() {
			t.Fatalf("could not focus %s", id)
		}
	}
	if !out.MoveCursorToIdentity(identity(grandchild.ID)) || !out.GoToParentTaskAtCursor() {
		t.Fatal("gP could not reach its rowless Focus root")
	}
	if got := out.CurrentTaskIdentity(); got != identity(child.ID) {
		t.Fatalf("cursor = %+v, want child", got)
	}
	if len(out.Focus.CrossStack) != 1 || out.Focus.CrossStack[0].TaskID != root.ID {
		t.Fatalf("focus stack = %+v, want only root", out.Focus.CrossStack)
	}
	if !out.GoToParentTaskAtCursor() || out.CurrentTaskIdentity() != identity(root.ID) || len(out.Focus.CrossStack) != 0 {
		t.Fatal("second jump did not leave Focus and land on root")
	}
}

func TestGoParentTaskFromChecklistItemLandsOnOwner(t *testing.T) {
	owner := primaryRowsTask("owner", models.StateBacklog)
	owner.Checklist = []models.ChecklistItem{{Text: "item"}}
	project := primaryRowsProject("project", owner)
	app := newUITestApp()
	setTestAppProjects(app, project)
	out := app.crossOutliner
	out.Population.Projects = []*models.Project{project}
	out.Population.Invalidate()
	out.CrossProjectRebuildRows()
	identity := taskref.TaskIdentity{ProjectID: project.ID, TaskID: owner.ID}
	out.Folds.SetChecklist(identity, true)
	out.RebuildRows()
	if !out.MoveCursorToChecklistItem(identity, 0) {
		t.Fatal("checklist item row is unavailable")
	}
	result := executeDirectPath(t, app, app.operationCommandContext(), []string{"g", "P"})
	if result.Err != nil || result.Status != "" {
		t.Fatalf("gP from checklist item = status %q err %v", result.Status, result.Err)
	}
	if !out.Rows[out.Cursor].IsTaskRow() || out.CurrentTaskIdentity() != identity {
		t.Fatal("gP did not jump from checklist item to its owner")
	}
}

func TestGoParentTaskExitsRankedSearchOntoParent(t *testing.T) {
	root := primaryRowsTask("root", models.StateBacklog)
	child := primaryRowsTask("child", models.StateBacklog)
	child.ParentID = &root.ID
	project := primaryRowsProject("project", root, child)
	out := scopeTestOutliner(project)
	searchRankedList(out, child.Title)
	if !out.MoveCursorToIdentity(taskref.TaskIdentity{ProjectID: project.ID, TaskID: child.ID}) {
		t.Fatal("child is absent from search results")
	}
	if !out.GoToParentTaskAtCursor() || out.ShowsRankedList() {
		t.Fatal("gP did not leave search")
	}
	if got := out.CurrentTaskIdentity(); got != (taskref.TaskIdentity{ProjectID: project.ID, TaskID: root.ID}) {
		t.Fatalf("cursor = %+v, want parent", got)
	}
}

func TestGoParentTaskRevealsAFilteredParent(t *testing.T) {
	root := primaryRowsTask("root", models.StateBacklog)
	child := primaryRowsTask("child", models.StateBacklog)
	child.ParentID = &root.ID
	child.Tags = []string{"only-child"}
	project := primaryRowsProject("project", root, child)
	out := scopeTestOutliner(project)
	out.Filter.Tags = map[string]bool{"only-child": true}
	adoptFilterCarriers(out)
	out.RebuildRows()
	identity := taskref.TaskIdentity{ProjectID: project.ID, TaskID: root.ID}
	if _, found := out.RowIndexForTask(identity); found {
		t.Fatal("parent unexpectedly visible before jump")
	}
	if !out.MoveCursorToIdentity(taskref.TaskIdentity{ProjectID: project.ID, TaskID: child.ID}) || !out.GoToParentTaskAtCursor() {
		t.Fatal("gP did not reveal filtered parent")
	}
	if out.CurrentTaskIdentity() != identity || !visibleOutlinerTaskIDs(out)[root.ID] {
		t.Fatalf("cursor = %+v, want revealed parent", out.CurrentTaskIdentity())
	}
	if out.Scope.ProjectID != "" {
		t.Fatalf("parent jump changed project scope to %q", out.Scope.ProjectID)
	}
}
