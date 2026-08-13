package app_test

import (
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/app"
)

func TestTaskLineEditPinsOnInjectedClock(t *testing.T) {
	wantNow := time.Date(2026, 7, 10, 12, 0, 0, 0, time.UTC)
	task := newTask("task", 1, "Ship release")
	project := newProject("project", "Project", task)
	repo := newFakeRepo(project)

	pinned := true
	result, err := newTestService(repo).EditTaskLine(app.TaskLineEditRequest{
		Source: app.TaskLocator{ProjectID: project.ID, TaskID: task.ID},
		Title:  "Ship release",
		Pinned: &pinned,
	})
	if err != nil {
		t.Fatalf("EditTaskLine: %v", err)
	}
	if !task.Pinned || task.PinnedAt == nil || *task.PinnedAt != wantNow {
		t.Fatalf("Pinned/PinnedAt = %v/%v, want true and the injected clock", task.Pinned, task.PinnedAt)
	}
	if !result.Changed || result.TransactionID == "" {
		t.Fatalf("result = %+v, want a committed change", result)
	}
}

func TestTaskLineEditUnpins(t *testing.T) {
	task := newTask("task", 1, "Ship release")
	pinnedAt := time.Date(2026, 6, 1, 9, 0, 0, 0, time.UTC)
	task.Pinned = true
	task.PinnedAt = &pinnedAt
	project := newProject("project", "Project", task)
	repo := newFakeRepo(project)

	pinned := false
	result, err := newTestService(repo).EditTaskLine(app.TaskLineEditRequest{
		Source: app.TaskLocator{ProjectID: project.ID, TaskID: task.ID},
		Title:  "Ship release",
		Pinned: &pinned,
	})
	if err != nil {
		t.Fatalf("EditTaskLine: %v", err)
	}
	if task.Pinned || task.PinnedAt != nil {
		t.Fatalf("Pinned/PinnedAt = %v/%v, want false/nil", task.Pinned, task.PinnedAt)
	}
	if !result.Changed || result.TransactionID == "" {
		t.Fatalf("result = %+v, want a committed change", result)
	}
}

func TestTaskLineEditRepeatedPinDoesNotTransact(t *testing.T) {
	task := newTask("task", 1, "Ship release")
	pinnedAt := time.Date(2026, 6, 1, 9, 0, 0, 0, time.UTC)
	updatedAt := time.Date(2026, 6, 2, 10, 0, 0, 0, time.UTC)
	task.Pinned = true
	task.PinnedAt = &pinnedAt
	task.UpdatedAt = updatedAt
	project := newProject("project", "Project", task)
	repo := newFakeRepo(project)

	pinned := true
	result, err := newTestService(repo).EditTaskLine(app.TaskLineEditRequest{
		Source: app.TaskLocator{ProjectID: project.ID, TaskID: task.ID},
		Title:  "Ship release",
		Pinned: &pinned,
	})
	if err != nil {
		t.Fatalf("EditTaskLine: %v", err)
	}
	if result.Changed || result.TransactionID != "" || len(result.AffectedProjectIDs) != 0 {
		t.Fatalf("no-op result = %+v", result)
	}
	if len(repo.commits) != 0 {
		t.Fatalf("no-op committed %d transaction(s)", len(repo.commits))
	}
	if task.PinnedAt == nil || *task.PinnedAt != pinnedAt {
		t.Fatalf("PinnedAt = %v, want the manual pinned order preserved at %v", task.PinnedAt, pinnedAt)
	}
	if task.UpdatedAt != updatedAt {
		t.Fatalf("UpdatedAt = %v, want the untouched %v", task.UpdatedAt, updatedAt)
	}
}

func TestTaskLineEditPinsInDestinationProject(t *testing.T) {
	wantNow := time.Date(2026, 7, 10, 12, 0, 0, 0, time.UTC)
	task := newTask("task", 1, "Ship release")
	source := newProject("source", "Source", task)
	destination := newProject("destination", "Destination")
	repo := newFakeRepo(source, destination)

	pinned := true
	result, err := newTestService(repo).EditTaskLine(app.TaskLineEditRequest{
		Source:               app.TaskLocator{ProjectID: source.ID, TaskID: task.ID},
		Title:                "Ship release",
		Pinned:               &pinned,
		DestinationProjectID: destination.ID,
		Parent:               app.TaskLineParentPatch{Mode: app.TaskLineParentRoot},
	})
	if err != nil {
		t.Fatalf("EditTaskLine: %v", err)
	}
	moved := destination.GetTaskByID(task.ID)
	if moved == nil {
		t.Fatal("edited task did not land in the destination project")
	}
	if !moved.Pinned || moved.PinnedAt == nil || *moved.PinnedAt != wantNow {
		t.Fatalf("Pinned/PinnedAt = %v/%v, want true and the injected clock", moved.Pinned, moved.PinnedAt)
	}
	if result.Task.ProjectID != destination.ID || result.TransactionID == "" {
		t.Fatalf("result = %+v, want a committed cross-project edit", result)
	}
}
