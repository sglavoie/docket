package tui

import (
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/testutil"
)

// savedCoordinatorProject creates a temp-dir project with one todo task
// (priority 5), saves it to disk, and returns the project plus the task ID.
// Callers wire a coordinator over it via newTestSecondaryCoordinators to
// exercise the coordinator-backed single-task mutations these primary views were
// migrated onto.
func savedCoordinatorProject(t *testing.T) (*models.Project, string) {
	t.Helper()
	dir, cleanup := testutil.TempProjectDir(t)
	t.Cleanup(cleanup)
	task, err := models.NewTask("task-1", 1, models.TaskTypeTask, "Original")
	if err != nil {
		t.Fatalf("NewTask: %v", err)
	}
	task.Priority = 5
	project := &models.Project{
		ID:          "proj-1",
		Name:        "Project",
		Path:        dir,
		NextShortID: 2,
		Tasks:       []*models.Task{task},
	}
	project.BuildTaskTree()
	if err := storage.SaveProject(project); err != nil {
		t.Fatalf("SaveProject: %v", err)
	}
	return project, task.ID
}

// reloadCoordinatorTask re-reads the project from disk and returns the named
// task, so a test observes exactly what the coordinator committed rather than an
// in-memory optimistic copy.
func reloadCoordinatorTask(t *testing.T, project *models.Project, taskID string) *models.Task {
	t.Helper()
	fresh := &models.Project{ID: project.ID, Name: project.Name, Path: project.Path}
	if err := storage.LoadProject(fresh); err != nil {
		t.Fatalf("LoadProject: %v", err)
	}
	task := fresh.GetTaskByID(taskID)
	if task == nil {
		t.Fatalf("task %s not found after reload", taskID)
	}
	return task
}

// TestPriorityViewStartSuggestedTaskCommitsThroughCoordinator verifies the
// priority view's "start suggested task" action commits the in_progress
// transition through the single-task coordinator.
func TestPriorityViewStartSuggestedTaskCommitsThroughCoordinator(t *testing.T) {
	project, taskID := savedCoordinatorProject(t)
	m := NewPriorityViewModel(project, newTestConfig(), nil)
	m.SetTaskItemCoordinator(newTestSecondaryCoordinators(project).TaskItem)
	m.suggestion = &SuggestedTask{Task: &PriorityTask{Task: project.GetTaskByID(taskID)}}

	if _, cmd := m.startSuggestedTask(); cmd == nil {
		t.Fatal("expected a transaction command from startSuggestedTask")
	}
	if got := reloadCoordinatorTask(t, project, taskID).State(); got != models.StateInProgress {
		t.Fatalf("state = %q, want in_progress", got)
	}
}

// TestFocusBoardEditTitleCommitsThroughCoordinator verifies the focus board's
// inline title edit commits through the single-task coordinator.
func TestFocusBoardEditTitleCommitsThroughCoordinator(t *testing.T) {
	project, taskID := savedCoordinatorProject(t)
	m := newTestFocusBoard()
	m.SetTaskItemCoordinator(newTestSecondaryCoordinators(project).TaskItem)
	row := boardRow{Task: project.GetTaskByID(taskID), Project: project, Source: project.Name}
	m.filtered = []boardRow{row}
	m.editMode = true
	m.editIdx = 0
	m.editInput.SetValue("Renamed")
	m.editInput.Focus()

	if _, cmd := m.handleEditInput(syntheticSpecialKey(tea.KeyEnter)); cmd == nil {
		t.Fatal("expected a transaction command from handleEditInput")
	}
	if got := reloadCoordinatorTask(t, project, taskID).Title; got != "Renamed" {
		t.Fatalf("title = %q, want Renamed", got)
	}
	if m.editMode {
		t.Error("editMode should be cleared after committing the edit")
	}
}

// TestKanbanPriorityMoveCommitsThroughCoordinator verifies the kanban single-card
// priority change commits through the single-task coordinator.
func TestKanbanPriorityMoveCommitsThroughCoordinator(t *testing.T) {
	project, taskID := savedCoordinatorProject(t)
	m := NewKanbanModel(project, nil, nil, NewProjectService())
	m.SetTaskItemCoordinator(newTestSecondaryCoordinators(project).TaskItem)

	if _, cmd := m.applyKanbanPriorityChange(project, project.GetTaskByID(taskID), 1); cmd == nil {
		t.Fatal("expected a transaction command from applyKanbanPriorityChange")
	}
	if got := reloadCoordinatorTask(t, project, taskID).Priority; got != 1 {
		t.Fatalf("priority = %d, want 1", got)
	}
}

// TestDailyReviewMutationCommitsAndTracksProgress verifies a daily-review
// single-task mutation commits through the coordinator and advances review
// progress, and that a rejected mutation surfaces an error without advancing it.
func TestDailyReviewMutationCommitsAndTracksProgress(t *testing.T) {
	project, taskID := savedCoordinatorProject(t)
	cfg := newTestConfig()
	setTestProjects(cfg, []*models.Project{project})
	m := NewDailyReviewModel(testRuntimeAt("", cfg), nil)
	m.SetTaskItemCoordinator(newTestSecondaryCoordinators(project).TaskItem)

	before := m.reviewedCount
	if cmd := m.applyReviewMutation(m.taskItem.SetPriority(taskID, 1)); cmd == nil {
		t.Fatal("expected a transaction command from applyReviewMutation")
	}
	if m.reviewedCount != before+1 {
		t.Fatalf("reviewedCount = %d, want %d", m.reviewedCount, before+1)
	}
	if got := reloadCoordinatorTask(t, project, taskID).Priority; got != 1 {
		t.Fatalf("priority = %d, want 1", got)
	}

	// Failure path: a mutation targeting a missing task is rejected before commit,
	// so it surfaces an error and leaves review progress untouched.
	if cmd := m.applyReviewMutation(m.taskItem.SetPriority("nonexistent", 2)); cmd != nil {
		t.Fatal("expected nil command on a rejected mutation")
	}
	if !m.HasError() {
		t.Error("expected an error to be surfaced on a rejected mutation")
	}
	if m.reviewedCount != before+1 {
		t.Fatalf("reviewedCount advanced on a rejected mutation: got %d", m.reviewedCount)
	}
}

// TestReviewStatusBlockReason covers the daily-review status preflight that keeps
// an invalid transition a status message (not an error toast) and off the review
// counter.
func TestReviewStatusBlockReason(t *testing.T) {
	parent, err := models.NewTask("parent", 1, models.TaskTypeStory, "Parent")
	if err != nil {
		t.Fatalf("NewTask: %v", err)
	}
	child, err := models.NewTask("child", 2, models.TaskTypeTask, "Child")
	if err != nil {
		t.Fatalf("NewTask: %v", err)
	}
	child.ParentID = &parent.ID
	project := &models.Project{Tasks: []*models.Task{parent, child}}
	project.BuildTaskTree()

	if reason := reviewStateBlockReason(project, parent, models.StateDone); reason == "" {
		t.Fatal("expected completing a parent with an incomplete child to be blocked")
	}
	if reason := reviewStateBlockReason(project, child, models.StateInProgress); reason != "" {
		t.Fatalf("expected a valid transition, got block reason %q", reason)
	}
}
