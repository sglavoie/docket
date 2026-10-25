package app_test

import (
	"errors"
	"testing"
	"time"

	"github.com/sglavoie/docket/internal/app"
	"github.com/sglavoie/docket/internal/models"
)

func operationTarget(projectID, taskID string) app.TaskLocator {
	return app.TaskLocator{ProjectID: projectID, TaskID: taskID}
}

func TestTaskOperationsExposeNarrowContentAndClipboardPaths(t *testing.T) {
	task := newTask("task-1", 1, "Before")
	task.Description = "Existing body"
	project := newProject("project-1", "Project", task)
	repo := newFakeRepo(project)
	svc := newTestService(repo)
	target := operationTarget(project.ID, task.ID)

	titleResult, err := svc.ExecuteTaskOperation(app.TaskOperationRequest{
		Operation: app.TaskOperationEditTitle,
		Targets:   []app.TaskLocator{target},
		Text:      "After",
	})
	if err != nil {
		t.Fatalf("edit title: %v", err)
	}
	if task.Title != "After" || task.Description != "Existing body" {
		t.Fatalf("title-only edit changed the wrong fields: %+v", task)
	}
	if len(titleResult.Tasks) != 1 || len(titleResult.TouchedProjectIDs) != 1 {
		t.Fatalf("title result = %+v", titleResult)
	}

	descriptionResult, err := svc.ExecuteTaskOperation(app.TaskOperationRequest{
		Operation: app.TaskOperationEditDescription,
		Targets:   []app.TaskLocator{target},
		Text:      "New body",
	})
	if err != nil {
		t.Fatalf("edit description: %v", err)
	}
	if task.Title != "After" || task.Description != "New body" {
		t.Fatalf("description-only edit changed the wrong fields: %+v", task)
	}
	if len(descriptionResult.Tasks) != 1 {
		t.Fatalf("description result = %+v", descriptionResult)
	}

	commitsBeforeCopy := len(repo.commits)
	for _, testCase := range []struct {
		operation app.TaskOperation
		want      string
	}{
		{app.TaskOperationCopyTitle, "After"},
		{app.TaskOperationCopyDescription, "New body"},
		{app.TaskOperationCopyTitleDescription, "After\n\nNew body"},
	} {
		result, copyErr := svc.ExecuteTaskOperation(app.TaskOperationRequest{
			Operation: testCase.operation,
			Targets:   []app.TaskLocator{target},
		})
		if copyErr != nil {
			t.Fatalf("%s: %v", testCase.operation, copyErr)
		}
		if result.Clipboard == nil || result.Clipboard.Text != testCase.want {
			t.Errorf("%s projection = %+v, want %q", testCase.operation, result.Clipboard, testCase.want)
		}
	}
	if len(repo.commits) != commitsBeforeCopy {
		t.Fatalf("clipboard projection committed: before=%d after=%d", commitsBeforeCopy, len(repo.commits))
	}
}

func TestTaskOperationCreateChildUsesReadyStateAndParentIdentity(t *testing.T) {
	parent := newTask("parent", 1, "Parent")
	project := newProject("project-1", "Project", parent)
	repo := newFakeRepo(project)
	svc := newTestService(repo)

	result, err := svc.ExecuteTaskOperation(app.TaskOperationRequest{
		Operation: app.TaskOperationCreateChild,
		Targets:   []app.TaskLocator{operationTarget(project.ID, parent.ID)},
		Text:      "Child",
	})
	if err != nil {
		t.Fatalf("create child: %v", err)
	}
	if len(result.Tasks) != 1 {
		t.Fatalf("result tasks = %+v", result.Tasks)
	}
	child := project.GetTaskByID(result.Tasks[0].TaskID)
	if child == nil || child.ParentID == nil || *child.ParentID != parent.ID {
		t.Fatalf("created child = %+v", child)
	}
	if child.State() != models.StateBacklog {
		t.Fatalf("child state = %s, want ready", child.State())
	}
}

func TestTaskOperationsUseOneAtomicPathForSingleAndBulkMetadata(t *testing.T) {
	first := newTask("first", 1, "First")
	second := newTask("second", 1, "Second")
	projectA := newProject("project-a", "Alpha", first)
	projectB := newProject("project-b", "Beta", second)
	repo := newFakeRepo(projectA, projectB)
	svc := newTestService(repo)

	targets := []app.TaskLocator{
		operationTarget(projectA.ID, first.ID),
		operationTarget(projectB.ID, second.ID),
	}
	result, err := svc.ExecuteTaskOperation(app.TaskOperationRequest{
		Operation: app.TaskOperationSetPriority,
		Targets:   targets,
		Priority:  1,
	})
	if err != nil {
		t.Fatalf("bulk priority: %v", err)
	}
	if first.Priority != 1 || second.Priority != 1 {
		t.Fatalf("priorities = %d/%d, want 1/1", first.Priority, second.Priority)
	}
	if len(result.Tasks) != 2 || len(result.TouchedProjectIDs) != 2 || len(repo.commits) != 1 {
		t.Fatalf("bulk operation result/commit = %+v commits=%d", result, len(repo.commits))
	}

	pinned, err := svc.ExecuteTaskOperation(app.TaskOperationRequest{
		Operation: app.TaskOperationSetPinned,
		Targets:   targets[:1],
		Pinned:    true,
	})
	if err != nil {
		t.Fatalf("single pin through common path: %v", err)
	}
	if len(pinned.Tasks) != 1 || !first.Pinned || first.PinnedAt == nil {
		t.Fatalf("single pin result/task = %+v %+v", pinned, first)
	}
	wantTime := time.Date(2026, 7, 10, 12, 0, 0, 0, time.UTC)
	if !first.PinnedAt.Equal(wantTime) {
		t.Fatalf("PinnedAt = %v, want injected clock %v", first.PinnedAt, wantTime)
	}
}

func TestTaskOperationsCoverDateDeferAndTagValues(t *testing.T) {
	task := newTask("task", 1, "Plan")
	project := newProject("project", "Project", task)
	repo := newFakeRepo(project)
	svc := newTestService(repo)
	target := []app.TaskLocator{operationTarget(project.ID, task.ID)}
	date := time.Date(2026, 7, 12, 18, 30, 0, 0, time.UTC)

	if _, err := svc.ExecuteTaskOperation(app.TaskOperationRequest{
		Operation: app.TaskOperationSetDueDate,
		Targets:   target,
		Date:      &date,
	}); err != nil {
		t.Fatalf("set due date: %v", err)
	}
	if task.DueDate == nil || task.DueDate.Year() != 2026 || task.DueDate.Month() != time.July || task.DueDate.Day() != 12 {
		t.Fatalf("due date = %v, want 2026-07-12", task.DueDate)
	}
	if _, err := svc.ExecuteTaskOperation(app.TaskOperationRequest{
		Operation: app.TaskOperationSetDueDate,
		Targets:   target,
	}); err != nil {
		t.Fatalf("clear due date: %v", err)
	}
	if task.DueDate != nil {
		t.Fatalf("cleared due date = %v, want nil", task.DueDate)
	}

	if _, err := svc.ExecuteTaskOperation(app.TaskOperationRequest{
		Operation: app.TaskOperationSetDefer,
		Targets:   target,
		Date:      &date,
	}); err != nil {
		t.Fatalf("set deferral: %v", err)
	}
	if task.DeferredUntil == nil || task.DeferredUntil.Day() != 12 {
		t.Fatalf("deferral = %v, want 2026-07-12", task.DeferredUntil)
	}

	if _, err := svc.ExecuteTaskOperation(app.TaskOperationRequest{
		Operation: app.TaskOperationSetTags,
		Targets:   target,
		Tags:      []string{"keep", "remove"},
	}); err != nil {
		t.Fatalf("replace tags: %v", err)
	}
	if _, err := svc.ExecuteTaskOperation(app.TaskOperationRequest{
		Operation:    app.TaskOperationSetTags,
		Targets:      target,
		TagPatch:     true,
		TagsToAdd:    []string{"add"},
		TagsToRemove: []string{"remove"},
	}); err != nil {
		t.Fatalf("patch tags: %v", err)
	}
	if got := task.Tags; len(got) != 2 || got[0] != "keep" || got[1] != "add" {
		t.Fatalf("patched tags = %v, want [keep add]", got)
	}
}

func TestTaskOperationStateUsesOneAtomicTransition(t *testing.T) {
	task := newTask("task", 1, "Ship")
	project := newProject("project", "Project", task)
	repo := newFakeRepo(project)
	svc := newTestService(repo)
	result, err := svc.ExecuteTaskOperation(app.TaskOperationRequest{
		Operation: app.TaskOperationSetState,
		Targets:   []app.TaskLocator{operationTarget(project.ID, task.ID)},
		State:     models.StateDone,
	})
	if err != nil {
		t.Fatalf("set state: %v", err)
	}
	if task.State() != models.StateDone || len(result.Tasks) != 1 {
		t.Fatalf("state operation result: state=%s result=%+v", task.State(), result)
	}
	if len(repo.commits) != 1 {
		t.Fatalf("state and cascade committed %d times, want one", len(repo.commits))
	}
}

func TestTaskOperationsRejectInvalidCardinalityAndRollBackFailedMutations(t *testing.T) {
	task := newTask("task", 1, "Before")
	task.Priority = models.PriorityLow
	project := newProject("project", "Project", task)
	repo := newFakeRepo(project)
	svc := newTestService(repo)
	target := operationTarget(project.ID, task.ID)

	_, err := svc.ExecuteTaskOperation(app.TaskOperationRequest{
		Operation: app.TaskOperationEditTitle,
		Targets:   []app.TaskLocator{target, target},
		Text:      "After",
	})
	if !errors.Is(err, app.ErrInvalidInput) {
		t.Fatalf("edit cardinality error = %v, want ErrInvalidInput", err)
	}

	repo.commitErr = errors.New("commit unavailable")
	_, err = svc.ExecuteTaskOperation(app.TaskOperationRequest{
		Operation: app.TaskOperationSetPriority,
		Targets:   []app.TaskLocator{target},
		Priority:  1,
	})
	if err == nil {
		t.Fatal("failed metadata commit returned nil error")
	}
	if task.Priority != models.PriorityLow {
		t.Fatalf("failed metadata commit left priority %d, want %d", task.Priority, models.PriorityLow)
	}

	_, err = svc.ExecuteTaskOperation(app.TaskOperationRequest{
		Operation: app.TaskOperationEditTitle,
		Targets:   []app.TaskLocator{target},
		Text:      "After",
	})
	if err == nil {
		t.Fatal("failed title commit returned nil error")
	}
	if task.Title != "Before" {
		t.Fatalf("failed title commit left title %q, want Before", task.Title)
	}
}
