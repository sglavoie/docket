package tui

import (
	"strings"
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

// TestCrossProjectBulkStatusSkipsInvalidTransitions verifies that a Done task
// included in a bulk "start" (InProgress) operation is not forcibly un-completed.
// Prior to the fix, ApplyTransition was called inside MutateTask without a
// CanTransitionTo guard, so a Done task would have its CompletedAt cleared and
// its status reverted to InProgress.
func TestCrossProjectBulkStatusSkipsInvalidTransitions(t *testing.T) {
	projDir := t.TempDir()

	doneTask, _ := models.NewTask("done-task-001", 1, models.TaskTypeTask, "Already done")
	completedAt := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)
	doneTask.Status = models.StatusDone
	doneTask.CompletedAt = &completedAt
	doneTask.UpdatedAt = completedAt

	todoTask, _ := models.NewTask("todo-task-001", 2, models.TaskTypeTask, "Not started yet")
	todoTask.Status = models.StatusTodo

	proj := &models.Project{ID: "test-proj", Name: "Test", Path: projDir}
	proj.Tasks = []*models.Task{doneTask, todoTask}
	proj.BuildTaskTree()

	if err := storage.SaveProject(proj); err != nil {
		t.Fatalf("save project: %v", err)
	}
	if err := storage.LoadProject(proj); err != nil {
		t.Fatalf("load project: %v", err)
	}

	doneRef := proj.GetTaskByID("done-task-001")
	todoRef := proj.GetTaskByID("todo-task-001")
	if doneRef == nil || todoRef == nil {
		t.Fatal("could not find tasks after reload")
	}

	m := &OutlinerModel{
		PrimaryViewBundle: PrimaryViewBundle{
			projectService: NewProjectService(),
			bulkOp:         NewBulkOperationComponent(),
			quickEdit:      NewQuickEditComponent(),
		},
		collapsedSections: make(map[string]bool),
		expandedNodes:     make(map[string]bool),
		MultiSelectState:  MultiSelectState{selectedTasks: make(map[string]bool)},
		activePerspective: 1,
		loadedProjects:    []*models.Project{proj},
	}

	// Simulate user selecting both tasks and issuing "start" (InProgress)
	m.bulkOp.SetOperation(models.StatusInProgress, []*models.Task{doneRef, todoRef}, 0, nil)
	resultModel, _ := m.crossExecutePendingBulkStatus()
	m = resultModel.(*OutlinerModel)

	// Load the saved state from disk to see what actually got persisted
	if err := storage.LoadProject(proj); err != nil {
		t.Fatalf("reload after bulk: %v", err)
	}

	doneFinal := proj.GetTaskByID("done-task-001")
	todoFinal := proj.GetTaskByID("todo-task-001")
	if doneFinal == nil || todoFinal == nil {
		t.Fatal("could not find tasks after bulk operation")
	}

	if doneFinal.Status != models.StatusDone {
		t.Errorf("done task should remain Done after bulk start, got %s", doneFinal.Status)
	}
	if doneFinal.CompletedAt == nil {
		t.Errorf("done task CompletedAt should be preserved, got nil")
	}
	if todoFinal.Status != models.StatusInProgress {
		t.Errorf("todo task should be InProgress after bulk start, got %s", todoFinal.Status)
	}

	_ = m
}

// newParentIncompleteChildProject builds a persisted project containing an
// incomplete Story parent with a single incomplete child plus a standalone leaf
// task, then reloads it from disk. Returns the reloaded project and the parent /
// standalone IDs.
func newParentIncompleteChildProject(t *testing.T, projDir string) (*models.Project, string, string) {
	t.Helper()

	parent, _ := models.NewTask("bulk-parent-001", 1, models.TaskTypeStory, "Parent Story")
	parent.Status = models.StatusInProgress
	child, _ := models.NewTask("bulk-child-001", 2, models.TaskTypeTask, "Incomplete child")
	child.ParentID = &parent.ID
	child.Status = models.StatusTodo
	standalone, _ := models.NewTask("bulk-standalone-001", 3, models.TaskTypeTask, "Standalone leaf")
	standalone.Status = models.StatusTodo

	proj := &models.Project{ID: "skip-proj", Name: "Skip", Path: projDir, NextShortID: 4}
	proj.Tasks = []*models.Task{parent, child, standalone}
	proj.BuildTaskTree()
	if err := storage.SaveProject(proj); err != nil {
		t.Fatalf("save project: %v", err)
	}
	if err := storage.LoadProject(proj); err != nil {
		t.Fatalf("load project: %v", err)
	}
	return proj, parent.ID, standalone.ID
}

// newSkipOutliner returns an OutlinerModel in cross-project mode over the given
// projects with the given config.
func newSkipOutliner(cfg *config.Config, projects []*models.Project) *OutlinerModel {
	return &OutlinerModel{
		PrimaryViewBundle: NewPrimaryViewBundle(cfg, nil),
		collapsedSections: make(map[string]bool),
		expandedNodes:     make(map[string]bool),
		MultiSelectState:  MultiSelectState{selectedTasks: make(map[string]bool)},
		activePerspective: 1,
		loadedProjects:    projects,
	}
}

// assertBulkSkipsParent runs a cross-project bulk transition of [parent,
// standalone] to the given completion status and asserts the parent is skipped
// (never persisted in a completion state while a child is incomplete) while the
// standalone leaf is applied, and that the status message reports the skip.
func assertBulkSkipsParent(t *testing.T, status models.TaskStatus) {
	t.Helper()

	proj, parentID, standaloneID := newParentIncompleteChildProject(t, t.TempDir())

	m := newSkipOutliner(config.NewConfig(), []*models.Project{proj})
	parentRef := proj.GetTaskByID(parentID)
	standaloneRef := proj.GetTaskByID(standaloneID)
	if parentRef == nil || standaloneRef == nil {
		t.Fatal("could not find tasks after reload")
	}

	m.bulkOp.SetOperation(status, []*models.Task{parentRef, standaloneRef}, 0, nil)
	resultModel, _ := m.crossExecutePendingBulkStatus()
	m = resultModel.(*OutlinerModel)

	if !strings.Contains(m.statusMessage, "1 skipped") || !strings.Contains(m.statusMessage, "incomplete children") {
		t.Errorf("status message = %q, want it to report 1 skipped for incomplete children", m.statusMessage)
	}

	if err := storage.LoadProject(proj); err != nil {
		t.Fatalf("reload after bulk: %v", err)
	}
	parentFinal := proj.GetTaskByID(parentID)
	if parentFinal.Status.IsComplete() {
		t.Errorf("parent should be skipped (not %s) while a child is incomplete, got %s", status, parentFinal.Status)
	}
	if got := proj.GetTaskByID(standaloneID).Status; got != status {
		t.Errorf("standalone leaf should be %s, got %s", status, got)
	}
}

// TestCrossProjectBulkCompleteSkipsParentWithIncompleteChild verifies a
// cross-project bulk-complete never writes a done parent with an incomplete
// child; the parent is skipped and the skip is reported.
func TestCrossProjectBulkCompleteSkipsParentWithIncompleteChild(t *testing.T) {
	assertBulkSkipsParent(t, models.StatusDone)
}

// TestCrossProjectBulkCancelSkipsParentWithIncompleteChild verifies a
// cross-project bulk-cancel likewise skips a parent with an incomplete child.
func TestCrossProjectBulkCancelSkipsParentWithIncompleteChild(t *testing.T) {
	assertBulkSkipsParent(t, models.StatusCanceled)
}

// TestCrossProjectBulkReconcileFullFileUndoRestoresReopenedParent verifies the
// reconcile net + full-file undo contract: when a cross-project bulk op touches
// a project that already holds an illegal done-parent/incomplete-child state,
// ReconcileParentCompletion reopens the parent before save, and a grouped undo
// restores the reopened parent (covered by the nil-TaskIDs full-project
// snapshot) along with the mutated leaves in every affected project.
func TestCrossProjectBulkReconcileFullFileUndoRestoresReopenedParent(t *testing.T) {
	// Alpha carries a pre-existing illegal state: done parent + incomplete child.
	alphaParent, _ := models.NewTask("alpha-parent-001", 1, models.TaskTypeStory, "Alpha parent")
	alphaParent.Complete()
	alphaChild, _ := models.NewTask("alpha-child-001", 2, models.TaskTypeTask, "Alpha incomplete child")
	alphaChild.ParentID = &alphaParent.ID
	alphaChild.Status = models.StatusTodo
	alphaLeaf, _ := models.NewTask("alpha-leaf-001", 3, models.TaskTypeTask, "Alpha standalone")
	alphaLeaf.Status = models.StatusTodo

	alpha := &models.Project{ID: "alpha-proj", Name: "Alpha", Path: t.TempDir(), NextShortID: 4}
	alpha.Tasks = []*models.Task{alphaParent, alphaChild, alphaLeaf}
	alpha.BuildTaskTree()
	if err := storage.SaveProject(alpha); err != nil {
		t.Fatalf("save alpha: %v", err)
	}

	betaLeaf, _ := models.NewTask("beta-leaf-001", 1, models.TaskTypeTask, "Beta standalone")
	betaLeaf.Status = models.StatusTodo
	beta := &models.Project{ID: "beta-proj", Name: "Beta", Path: t.TempDir(), NextShortID: 2}
	beta.Tasks = []*models.Task{betaLeaf}
	beta.BuildTaskTree()
	if err := storage.SaveProject(beta); err != nil {
		t.Fatalf("save beta: %v", err)
	}

	if err := storage.LoadProject(alpha); err != nil {
		t.Fatalf("load alpha: %v", err)
	}
	if err := storage.LoadProject(beta); err != nil {
		t.Fatalf("load beta: %v", err)
	}

	cfg := config.NewConfig()
	cfg.UndoHistoryDepth = intPtr(5)
	m := newSkipOutliner(cfg, []*models.Project{alpha, beta})

	alphaLeafRef := alpha.GetTaskByID("alpha-leaf-001")
	betaLeafRef := beta.GetTaskByID("beta-leaf-001")
	if alphaLeafRef == nil || betaLeafRef == nil {
		t.Fatal("could not find leaves after reload")
	}
	m.bulkOp.SetOperation(models.StatusDone, []*models.Task{alphaLeafRef, betaLeafRef}, 0, nil)
	resultModel, _ := m.crossExecutePendingBulkStatus()
	m = resultModel.(*OutlinerModel)

	if err := storage.LoadProject(alpha); err != nil {
		t.Fatalf("reload alpha after bulk: %v", err)
	}
	// Reconcile reopened the illegal done parent before save.
	if got := alpha.GetTaskByID("alpha-parent-001").Status; got != models.StatusInProgress {
		t.Errorf("alpha parent after reconcile = %s, want %s", got, models.StatusInProgress)
	}
	if got := alpha.GetTaskByID("alpha-leaf-001").Status; got != models.StatusDone {
		t.Errorf("alpha leaf = %s, want done", got)
	}

	// Grouped undo restores both projects; the reopened parent must come back
	// done via the full-file snapshot even though it was never in the bulk set.
	undoResult, _, ok := m.crossHandleUndo("u")
	if !ok {
		t.Fatal("crossHandleUndo returned ok=false")
	}
	m = undoResult.(*OutlinerModel)
	if err := storage.LoadProject(alpha); err != nil {
		t.Fatalf("reload alpha after undo: %v", err)
	}
	if err := storage.LoadProject(beta); err != nil {
		t.Fatalf("reload beta after undo: %v", err)
	}
	if got := alpha.GetTaskByID("alpha-parent-001").Status; got != models.StatusDone {
		t.Errorf("alpha parent after undo = %s, want %s (full-file snapshot must restore the reopened parent)", got, models.StatusDone)
	}
	if got := alpha.GetTaskByID("alpha-leaf-001").Status; got != models.StatusTodo {
		t.Errorf("alpha leaf after undo = %s, want todo", got)
	}
	if got := beta.GetTaskByID("beta-leaf-001").Status; got != models.StatusTodo {
		t.Errorf("beta leaf after undo = %s, want todo", got)
	}
}
