package app_test

import (
	"errors"
	"testing"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

func TestCommittedStateEffectsDistinguishEntryNoopAndSchedule(t *testing.T) {
	first, same, scheduled := newTask("first", 1, "first"), newTask("same", 2, "same"), newTask("scheduled", 3, "scheduled")
	same.ApplyWorkflowState(models.StateDoing)
	scheduled.NextCheckIn = checkInSchedule(3, 0).Next
	repo := newFakeRepo(newProject("p", "Project", first, same, scheduled))
	svc := newCheckInService(repo)
	result, err := svc.BulkTransition([]app.TaskLocator{repo.loc(first.ID), repo.loc(scheduled.ID)}, models.StateDoing)
	if err != nil {
		t.Fatal(err)
	}
	for i, want := range []bool{true, false} {
		if got := result.Tasks[i].StateEffect.OffersCheckIn(); got != want {
			t.Fatalf("effect %d = %+v, offer=%v want %v", i, result.Tasks[i].StateEffect, got, want)
		}
	}
	repo.commitErr = errors.New("refused")
	failed, err := svc.BulkTransition([]app.TaskLocator{repo.loc(first.ID)}, models.StateWaiting)
	if err == nil || len(failed.Tasks) != 0 {
		t.Fatalf("failed commit leaked effects: %+v %v", failed, err)
	}
}

func TestCommittedStateEffectsCreationAndLineMove(t *testing.T) {
	repo := newFakeRepo(newProject("p", "Project"), newProject("q", "Other"))
	svc := newCheckInService(repo)
	created, err := svc.CaptureMany([]app.CaptureRequest{
		{ProjectID: "p", Title: "one", Destination: app.CaptureDestination(models.StateDoing)},
		{ProjectID: "q", Title: "two", Destination: app.CaptureDestination(models.StateWaiting), CheckIn: checkInSchedule(3, 0)},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !created[0].StateEffect.OffersCheckIn() || created[0].StateEffect.Before != "" || created[1].StateEffect.OffersCheckIn() {
		t.Fatalf("created effects: %+v %+v", created[0], created[1])
	}
	source := app.TaskLocator{ProjectID: "p", TaskID: created[0].TaskID}
	moved, err := svc.EditTaskLine(app.TaskLineEditRequest{Source: source, Title: "one", DestinationProjectID: "q"})
	if err != nil {
		t.Fatal(err)
	}
	if moved.StateEffect.OffersCheckIn() {
		t.Fatalf("ownership alone became entry: %+v", moved)
	}
	waiting := app.CaptureDestination(models.StateWaiting)
	edited, err := svc.EditTaskLine(app.TaskLineEditRequest{Source: moved.Task, Title: "one", Destination: &waiting})
	if err != nil {
		t.Fatal(err)
	}
	if !edited.StateEffect.OffersCheckIn() || edited.StateEffect.Before != models.StateDoing {
		t.Fatalf("line effect: %+v", edited)
	}
	noop, err := svc.EditTaskLine(app.TaskLineEditRequest{Source: moved.Task, Title: "one", Destination: &waiting})
	if err != nil || noop.StateEffect.OffersCheckIn() {
		t.Fatalf("no-op effect: %+v %v", noop, err)
	}
}

func TestCheckInOfferSaveRefusesStaleEligibilityAndGuardsReadSet(t *testing.T) {
	for _, stale := range []string{"scheduled", "left-active", "missing", "commit-race", "eligible"} {
		t.Run(stale, func(t *testing.T) {
			task := newTask("one", 1, "one")
			task.ApplyWorkflowState(models.StateDoing)
			if stale == "scheduled" {
				task.NextCheckIn = checkInSchedule(8, 0).Next
			}
			if stale == "left-active" {
				task.ApplyWorkflowState(models.StateBacklog)
			}
			repo := newFakeRepo(newProject("p", "Project", task))
			svc := newCheckInService(repo)
			target := repo.loc(task.ID)
			if stale == "missing" {
				target.TaskID = "missing"
			}
			if stale == "commit-race" {
				repo.preCommit = func() { repo.revisions["p"] = "concurrent-revision" }
			}
			result, err := svc.ExecuteTaskOperation(app.TaskOperationRequest{Operation: app.TaskOperationSetCheckIn, Targets: []app.TaskLocator{target}, CheckIn: checkInSchedule(2, 7), CheckInOffer: true})
			if stale != "eligible" {
				if err == nil || result.TransactionID != "" || len(repo.commits) != 0 {
					t.Fatalf("stale offer wrote: %+v %v", result, err)
				}
				if stale == "commit-race" && !errors.Is(err, storage.ErrStaleRevision) {
					t.Fatalf("race error=%v", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			commit, _ := repo.lastCommit()
			if len(commit.ProjectPreconditions) != 1 {
				t.Fatalf("no strict guard: %+v", commit.ProjectPreconditions)
			}
		})
	}
}

func TestCommittedStateEffectsOutlineFormAndBulkMove(t *testing.T) {
	task := newTask("one", 1, "one")
	repo := newFakeRepo(newProject("p", "Project", task), newProject("q", "Other"))
	svc := newCheckInService(repo)
	outline, err := svc.ApplyOutlinePlan(app.OutlinePlanRequest{ProjectID: "p", ExpectedRevision: repo.revisions["p"], ScopeTaskIDs: []string{task.ID}, Lines: []app.OutlinePlanLine{
		{SourceLine: 1, TaskID: task.ID, Title: task.Title, Destination: "doing"},
		{SourceLine: 2, Title: "new line", Destination: "waiting"},
	}})
	if err != nil {
		t.Fatal(err)
	}
	if len(outline.Created) != 1 || len(outline.Updated) != 1 || !outline.Created[0].StateEffect.OffersCheckIn() || !outline.Updated[0].StateEffect.OffersCheckIn() {
		t.Fatalf("outline=%+v", outline)
	}
	form, err := svc.AddTaskWithChildren(app.AddTaskRequest{ProjectID: "p", Title: "form", Destination: app.CaptureDestination(models.StateDoing)})
	if err != nil || !form.Task.StateEffect.OffersCheckIn() {
		t.Fatalf("form=%+v %v", form, err)
	}
	moved, err := svc.BulkMoveToProject([]app.TaskLocator{repo.loc(task.ID)}, "q")
	if err != nil {
		t.Fatal(err)
	}
	if moved.Tasks[0].StateEffect.OffersCheckIn() {
		t.Fatal("bulk ownership move invented a creation effect")
	}
}
