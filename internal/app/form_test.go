package app_test

import (
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/models"
)

// TestAddTaskWithChildrenCreatesParentAndSubtasks proves the task-form create use
// case commits a parent plus its subtasks as ONE ChangeSet, parents the subtasks,
// and inherits the parent's priority, state, and tags. A blank-title subtask is
// skipped.
func TestAddTaskWithChildrenCreatesParentAndSubtasks(t *testing.T) {
	project := newProject("proj-1", "Proj")
	repo := newFakeRepo(project)
	svc := newTestService(repo)

	res, err := svc.AddTaskWithChildren(app.AddTaskRequest{
		ProjectID: project.ID,
		Title:     "Parent Epic",
		Type:      models.TaskTypeEpic,
		Priority:  2,
		Tags:      []string{"backend"},
		State:     models.StateNext,
		Subtasks: []app.AddSubtaskRequest{
			{Title: "child one", Type: models.TaskTypeTask},
			{Title: "  ", Type: models.TaskTypeTask},
			{Title: "child two", Type: models.TaskTypeBug},
		},
	})
	if err != nil {
		t.Fatalf("AddTaskWithChildren: %v", err)
	}
	if len(res.Subtasks) != 2 {
		t.Fatalf("subtask results = %d, want 2 (blank skipped)", len(res.Subtasks))
	}
	if _, ok := repo.lastCommit(); !ok {
		t.Fatal("expected exactly one changeset committed")
	}

	parent := project.GetTaskByID(res.Task.TaskID)
	if parent == nil || parent.Type != models.TaskTypeEpic {
		t.Fatalf("parent not committed as epic: %v", parent)
	}
	for _, cr := range res.Subtasks {
		child := project.GetTaskByID(cr.TaskID)
		if child == nil {
			t.Fatalf("subtask %s missing", cr.TaskID)
		}
		if child.ParentID == nil || *child.ParentID != parent.ID {
			t.Errorf("subtask %q not parented to the epic", child.Title)
		}
		if child.Priority != 2 {
			t.Errorf("subtask priority = %d, want inherited 2", child.Priority)
		}
		if !child.HasTag("backend") {
			t.Errorf("subtask tags = %v, want inherited backend", child.Tags)
		}
		if got := child.State(); got != models.StateNext {
			t.Errorf("subtask section = %q, want Next", got)
		}
	}
}

// TestAddTaskWithChildrenAppliesFormFields proves the create use case applies the
// task form's scheduling, reference, flag, and checklist metadata.
func TestAddTaskWithChildrenAppliesFormFields(t *testing.T) {
	project := newProject("proj-1", "Proj")
	repo := newFakeRepo(project)
	svc := newTestService(repo)

	due := time.Date(2026, 8, 1, 15, 0, 0, 0, time.UTC)
	start := time.Date(2026, 7, 20, 0, 0, 0, 0, time.UTC)
	res, err := svc.AddTaskWithChildren(app.AddTaskRequest{
		ProjectID:     project.ID,
		Title:         "Full task",
		Priority:      3,
		State:         models.StateNext,
		DueDate:       &due,
		HasTime:       true,
		StartDate:     &start,
		EstimatedDays: 2.5,
		URL:           "https://example.com",
		Source:        "meeting",
		Pinned:        true,
		ChecklistItems: []models.ChecklistItem{
			{Text: "done step", Done: true},
			{Text: "  ", Done: false},
			{Text: "open step", Done: false},
		},
	})
	if err != nil {
		t.Fatalf("AddTaskWithChildren: %v", err)
	}
	task := project.GetTaskByID(res.Task.TaskID)
	if task == nil {
		t.Fatal("task not committed")
	}
	if task.DueDate == nil || !task.HasTime {
		t.Errorf("due/hasTime not applied: %v %v", task.DueDate, task.HasTime)
	}
	if task.StartDate == nil {
		t.Error("start date not applied")
	}
	if task.EstimatedDays != 2.5 {
		t.Errorf("estimated days = %v, want 2.5", task.EstimatedDays)
	}
	if task.URL != "https://example.com" || task.Source != "meeting" {
		t.Errorf("url/source not applied: %q %q", task.URL, task.Source)
	}
	if !task.Pinned || task.PinnedAt == nil {
		t.Error("pinned/pinnedAt not applied")
	}
	if len(task.Checklist) != 2 {
		t.Fatalf("checklist = %d, want 2 (blank filtered)", len(task.Checklist))
	}
	if !task.Checklist[0].Done {
		t.Error("first checklist item should preserve Done=true")
	}
}

// TestAddTaskWithChildrenInsertBeforeDimmed proves the create positioning inserts
// a new root task before the first dimmed root task in the same display section.
func TestAddTaskWithChildrenInsertBeforeDimmed(t *testing.T) {
	dimmed := newTask("dimmed-1", 1, "Dimmed")
	dimmed.ApplyWorkflowState(models.StateNext)
	dimmed.Dimmed = true
	project := newProject("proj-1", "Proj", dimmed)
	repo := newFakeRepo(project)
	svc := newTestService(repo)

	res, err := svc.AddTaskWithChildren(app.AddTaskRequest{
		ProjectID:          project.ID,
		Title:              "New",
		State:              models.StateNext,
		InsertBeforeDimmed: true,
	})
	if err != nil {
		t.Fatalf("AddTaskWithChildren: %v", err)
	}
	newIdx, dimmedIdx := -1, -1
	for i, task := range project.Tasks {
		switch task.ID {
		case res.Task.TaskID:
			newIdx = i
		case dimmed.ID:
			dimmedIdx = i
		}
	}
	if newIdx < 0 || dimmedIdx < 0 || newIdx >= dimmedIdx {
		t.Fatalf("new (%d) should precede dimmed (%d)", newIdx, dimmedIdx)
	}
}

// TestEditFormReplacesFullFieldSet proves a form edit replaces the whole editable
// field set, including switching type and clearing the previous type's attrs.
func TestEditFormReplacesFullFieldSet(t *testing.T) {
	task := newTask("t-1", 1, "Before")
	task.Type = models.TaskTypeBug
	task.BugAttrs = &models.BugAttrs{Severity: models.BugSeverityMajor}
	project := newProject("proj-1", "Proj", task)
	repo := newFakeRepo(project)
	svc := newTestService(repo)
	state := models.StateNext

	res, err := svc.Edit(app.EditRequest{
		TaskID: task.ID,
		Form: &app.FormEdit{
			Title:    "After",
			Type:     models.TaskTypeTask,
			Priority: 1,
			Tags:     []string{"x"},
			TaskAttrs: &models.TaskAttrs{
				Effort: models.EffortSmall,
			},
		},
		WorkflowState: &state,
	})
	if err != nil {
		t.Fatalf("Edit: %v", err)
	}
	got := project.GetTaskByID(res.TaskID)
	if got.Title != "After" || got.Type != models.TaskTypeTask || got.Priority != 1 {
		t.Fatalf("edit not applied: %+v", got)
	}
	if got.BugAttrs != nil {
		t.Error("stale BugAttrs should be cleared on type change")
	}
	if got.TaskAttrs == nil || got.TaskAttrs.Effort != models.EffortSmall {
		t.Errorf("new TaskAttrs not applied: %v", got.TaskAttrs)
	}
	if !got.HasTag("x") {
		t.Errorf("tags = %v, want x", got.Tags)
	}
}

// TestEditFormRejectsInvalidTitle proves a form edit validates the title before
// committing.
func TestEditFormRejectsInvalidTitle(t *testing.T) {
	task := newTask("t-1", 1, "Before")
	project := newProject("proj-1", "Proj", task)
	repo := newFakeRepo(project)
	svc := newTestService(repo)

	_, err := svc.Edit(app.EditRequest{
		TaskID: task.ID,
		Form:   &app.FormEdit{Title: "   ", Type: models.TaskTypeTask, Priority: 3},
	})
	if err == nil {
		t.Fatal("expected an error for a blank title")
	}
	if len(repo.commits) != 0 {
		t.Fatal("a rejected edit must not commit")
	}
}

func TestEditFormCanonicalWorkflowStateOverride(t *testing.T) {
	t.Run("preserves in progress through empty active section", func(t *testing.T) {
		task := newTask("t-1", 1, "Before")
		task.ApplyWorkflowState(models.StateInProgress)
		project := newProject("proj-1", "Proj", task)
		svc := newTestService(newFakeRepo(project))
		state := models.StateInProgress

		_, err := svc.Edit(app.EditRequest{
			TaskID: task.ID,
			Form: &app.FormEdit{
				Title: "After", Type: models.TaskTypeTask, Priority: 3, Tags: []string{},
			},
			WorkflowState: &state,
		})
		if err != nil {
			t.Fatalf("Edit: %v", err)
		}
		if got := task.State(); got != models.StateInProgress {
			t.Fatalf("state = %s, want in_progress", got)
		}
	})

	t.Run("applies lifecycle side effects", func(t *testing.T) {
		task := newTask("t-1", 1, "Before")
		project := newProject("proj-1", "Proj", task)
		svc := newTestService(newFakeRepo(project))
		state := models.StateDone

		_, err := svc.Edit(app.EditRequest{
			TaskID: task.ID,
			Form: &app.FormEdit{
				Title: "Done", Type: models.TaskTypeTask, Priority: 3, Tags: []string{},
			},
			WorkflowState: &state,
		})
		if err != nil {
			t.Fatalf("Edit: %v", err)
		}
		if task.State() != models.StateDone || task.CompletedAt == nil {
			t.Fatalf("terminal state did not apply lifecycle metadata: %+v", task)
		}
	})
}
