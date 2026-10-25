package app_test

import (
	"errors"
	"testing"

	"github.com/sglavoie/docket/internal/app"
	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/storage"
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

// TestExplicitCheckInSaveTakesNoEligibilityGuard pins the whole of what a
// check-in save judges now that scheduling is only ever explicit: the locator
// must resolve and the target must not be terminal. A task that was scheduled
// while the editor was open, or that left active work in the meantime, is
// exactly the task a user reaching for the editor means to correct, so neither
// refuses; nor does a concurrent write to the project, since a strict read-set
// guard existed only to keep an automatic save from replacing a schedule the
// user had just made by hand. A concurrent write to the project is still
// refused, but by the project's own ExpectedRevision contract rather than by a
// guard of this operation's, which is the difference the assertion pins.
func TestExplicitCheckInSaveTakesNoEligibilityGuard(t *testing.T) {
	for _, stale := range []string{"scheduled", "left-active", "missing", "terminal", "commit-race", "eligible"} {
		t.Run(stale, func(t *testing.T) {
			task := newTask("one", 1, "one")
			task.ApplyWorkflowState(models.StateDoing)
			switch stale {
			case "scheduled":
				task.NextCheckIn = checkInSchedule(8, 0).Next
			case "left-active":
				task.ApplyWorkflowState(models.StateBacklog)
			case "terminal":
				task.ApplyWorkflowState(models.StateDone)
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
			result, err := svc.ExecuteTaskOperation(app.TaskOperationRequest{
				Operation: app.TaskOperationSetCheckIn,
				Targets:   []app.TaskLocator{target},
				CheckIn:   checkInSchedule(2, 7),
			})
			switch stale {
			case "missing", "terminal", "commit-race":
				if err == nil || result.TransactionID != "" {
					t.Fatalf("%s target wrote: %+v %v", stale, result, err)
				}
				if raced := errors.Is(err, storage.ErrStaleRevision); raced != (stale == "commit-race") {
					t.Fatalf("%s refused for the wrong reason: %v", stale, err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			commit, _ := repo.lastCommit()
			if len(commit.ProjectPreconditions) != 0 {
				t.Fatalf("explicit save attached a strict guard: %+v", commit.ProjectPreconditions)
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
