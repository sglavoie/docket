package app_test

import (
	"errors"
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/textgrammar"
)

// This file holds the check-in's preservation contract: the schedule and the
// review history are the task's, not any one editor's, so every mutation that
// never mentions them has to leave them exactly as it found them. The paths are
// enumerated rather than sampled, because each one writes the task through
// different code and a preservation bug shows up in one path at a time.

// checkInReviewedAt is the review history every fixture below carries. It is a
// separate instant from the schedule so an assertion cannot pass by accident
// when one value is written over the other.
var checkInReviewedAt = checkInNow.AddDate(0, 0, -9)

// scheduledTask is open work carrying a complete check-in and a review history:
// both halves of what an unrelated edit must preserve and a terminal transition
// must treat differently.
func scheduledTask(t *testing.T, id string, shortID int, title string) *models.Task {
	t.Helper()
	task := newTask(id, shortID, title)
	seedTaskCheckIn(t, task)
	return task
}

// seedTaskCheckIn writes the fixture schedule and history onto an existing task,
// which is what a descendant carried into a move needs.
func seedTaskCheckIn(t *testing.T, task *models.Task) {
	t.Helper()
	if err := task.SetCheckIn(checkInSchedule(4, 7), checkInNow.AddDate(0, 0, -1)); err != nil {
		t.Fatalf("seed check-in: %v", err)
	}
	reviewed := checkInReviewedAt
	task.LastReviewedAt = &reviewed
}

// persistedTask renders the project the repo last committed through the live
// canonical writer and reads the task back out of the parsed bytes, so every
// assertion below is about what durable storage holds rather than about the
// in-memory pointer a mutation happened to keep.
func persistedTask(t *testing.T, repo *fakeRepo, projectID, taskID string) *models.Task {
	t.Helper()
	document, ok := repo.documents[projectID]
	if !ok {
		t.Fatalf("no committed document for project %q", projectID)
	}
	content, err := document.WriteCurrent()
	if err != nil {
		t.Fatalf("WriteCurrent: %v", err)
	}
	parsed, err := storage.ParseProjectDocument(content)
	if err != nil {
		t.Fatalf("ParseProjectDocument: %v", err)
	}
	for _, task := range parsed.Tasks {
		if task != nil && task.ID == taskID {
			return task
		}
	}
	t.Fatalf("project %q no longer holds task %q", projectID, taskID)
	return nil
}

// requireScheduleIntact is the preservation assertion itself, spelled once so
// every path below states the same expectation. The date is compared as the
// calendar day it is, because reading a task back out of its document re-parses
// that day in the local zone.
func requireScheduleIntact(t *testing.T, task *models.Task, where string) {
	t.Helper()
	want := checkInToday.AddDate(0, 0, 4)
	if task.NextCheckIn == nil || task.NextCheckIn.Format(time.DateOnly) != want.Format(time.DateOnly) {
		t.Errorf("%s: NextCheckIn = %v, want the preserved %v", where, task.NextCheckIn, want)
	}
	if task.CheckInRepeatDays != 7 {
		t.Errorf("%s: repeat = %d, want the preserved 7", where, task.CheckInRepeatDays)
	}
	if task.LastReviewedAt == nil || !task.LastReviewedAt.Equal(checkInReviewedAt) {
		t.Errorf("%s: LastReviewedAt = %v, want the preserved %v",
			where, task.LastReviewedAt, checkInReviewedAt)
	}
}

// TestEditPreservesAnUnmentionedCheckIn is the omission rule on the edit request
// itself: a nil patch is "say nothing", not "clear", which is what keeps a title
// fix and an external-editor body round trip from retiring a schedule.
func TestEditPreservesAnUnmentionedCheckIn(t *testing.T) {
	task := scheduledTask(t, "t1", 1, "chase the vendor")
	repo := newFakeRepo(newProject("proj-a", "Alpha", task))
	title := "chase the vendor again"
	body := "The importer cannot land without it."

	if _, err := newCheckInService(repo).Edit(app.EditRequest{
		Locator:     repo.loc("t1"),
		Title:       &title,
		Description: &body,
	}); err != nil {
		t.Fatalf("Edit: %v", err)
	}
	requireScheduleIntact(t, persistedTask(t, repo, "proj-a", "t1"), "content edit")
}

// TestEditWritesAndClearsAMentionedCheckIn is the other half: a named patch is
// the whole schedule, and the empty one is the clear. Neither touches the review
// history, because replacing what is owed says nothing about what happened.
func TestEditWritesAndClearsAMentionedCheckIn(t *testing.T) {
	task := scheduledTask(t, "t1", 1, "chase the vendor")
	repo := newFakeRepo(newProject("proj-a", "Alpha", task))
	svc := newCheckInService(repo)

	replacement := checkInSchedule(10, 30)
	if _, err := svc.Edit(app.EditRequest{Locator: repo.loc("t1"), CheckIn: &replacement}); err != nil {
		t.Fatalf("Edit with a schedule: %v", err)
	}
	stored := persistedTask(t, repo, "proj-a", "t1")
	want := checkInToday.AddDate(0, 0, 10)
	if stored.NextCheckIn == nil ||
		stored.NextCheckIn.Format(time.DateOnly) != want.Format(time.DateOnly) ||
		stored.CheckInRepeatDays != 30 {
		t.Fatalf("check-in = %v every %d days, want %v every 30",
			stored.NextCheckIn, stored.CheckInRepeatDays, want)
	}

	cleared := models.CheckInSchedule{}
	if _, err := svc.Edit(app.EditRequest{Locator: repo.loc("t1"), CheckIn: &cleared}); err != nil {
		t.Fatalf("Edit with an empty schedule: %v", err)
	}
	stored = persistedTask(t, repo, "proj-a", "t1")
	if !stored.CheckIn().IsEmpty() {
		t.Errorf("check-in = %v, want the empty patch to have cleared it", stored.CheckIn())
	}
	if stored.LastReviewedAt == nil || !stored.LastReviewedAt.Equal(checkInReviewedAt) {
		t.Errorf("LastReviewedAt = %v, want it to survive both writes", stored.LastReviewedAt)
	}
}

// TestEditRefusesAScheduleOnTerminalWork keeps the edit surface on the same
// terms as the bulk operation, so a finished task cannot acquire a schedule the
// lifecycle primitive would have dropped and the canonical writer would refuse.
func TestEditRefusesAScheduleOnTerminalWork(t *testing.T) {
	task := newTask("t1", 1, "shipped it")
	task.ApplyWorkflowState(models.StateDone)
	repo := newFakeRepo(newProject("proj-a", "Alpha", task))

	schedule := checkInSchedule(3, 0)
	_, err := newCheckInService(repo).Edit(app.EditRequest{Locator: repo.loc("t1"), CheckIn: &schedule})
	if !errors.Is(err, app.ErrInvalidInput) {
		t.Fatalf("Edit = %v, want ErrInvalidInput", err)
	}
	if !task.CheckIn().IsEmpty() || len(repo.commits) != 0 {
		t.Errorf("refused edit wrote %v across %d commits", task.CheckIn(), len(repo.commits))
	}
}

// TestTaskLineEditPreservesTheCheckIn walks the inline `e` path, which rewrites
// the task through a completely separate patch applier from Edit's.
func TestTaskLineEditPreservesTheCheckIn(t *testing.T) {
	task := scheduledTask(t, "t1", 1, "chase the vendor")
	repo := newFakeRepo(newProject("proj-a", "Alpha", task))

	result, err := newCheckInService(repo).EditTaskLine(app.TaskLineEditRequest{
		Source:    repo.loc("t1"),
		Title:     "chase the vendor twice",
		TagsToAdd: []string{"importer"},
	})
	if err != nil {
		t.Fatalf("EditTaskLine: %v", err)
	}
	if !result.Changed {
		t.Fatal("the fixture edit changed nothing; preservation proves nothing")
	}
	requireScheduleIntact(t, persistedTask(t, repo, "proj-a", "t1"), "line edit")
}

// TestTaskLineEditAcrossProjectsPreservesTheCheckIn is the same patch applier
// over a project boundary, which removes the task from one document and writes
// it into another. The schedule is the task's, so changing owners changes
// nothing about it.
func TestTaskLineEditAcrossProjectsPreservesTheCheckIn(t *testing.T) {
	task := scheduledTask(t, "t1", 1, "chase the vendor")
	repo := newFakeRepo(
		newProject("proj-a", "Alpha", task),
		newProject("proj-b", "Beta"))

	result, err := newCheckInService(repo).EditTaskLine(app.TaskLineEditRequest{
		Source:               repo.loc("t1"),
		Title:                "chase the vendor",
		DestinationProjectID: "proj-b",
	})
	if err != nil {
		t.Fatalf("EditTaskLine across projects: %v", err)
	}
	if result.Task.ProjectID != "proj-b" {
		t.Fatalf("moved task lives in %q, want proj-b", result.Task.ProjectID)
	}
	requireScheduleIntact(t, persistedTask(t, repo, "proj-b", "t1"), "cross-project line edit")
}

// TestBulkMovePreservesTheCheckIn is the bulk half of the same boundary, which
// reaches storage through the batch transaction rather than the single edit, and
// carries a descendant the selection never named.
func TestBulkMovePreservesTheCheckIn(t *testing.T) {
	root := scheduledTask(t, "t1", 1, "chase the vendor")
	child := childOf("t2", 2, "chase the sub-vendor", root)
	seedTaskCheckIn(t, child)
	repo := newFakeRepo(
		newProject("proj-a", "Alpha", root, child),
		newProject("proj-b", "Beta"))

	if _, err := newCheckInService(repo).BulkMoveToProject([]app.TaskLocator{repo.loc("t1")}, "proj-b"); err != nil {
		t.Fatalf("BulkMoveToProject: %v", err)
	}
	requireScheduleIntact(t, persistedTask(t, repo, "proj-b", "t1"), "bulk move")
	requireScheduleIntact(t, persistedTask(t, repo, "proj-b", "t2"), "bulk move of an unnamed child")
}

// TestReparentPreservesTheCheckIn covers the hierarchy path, which rewrites the
// parent edge and the project's task order without going near task content.
func TestReparentPreservesTheCheckIn(t *testing.T) {
	parent := newTask("t0", 1, "the umbrella")
	task := scheduledTask(t, "t1", 2, "chase the vendor")
	repo := newFakeRepo(newProject("proj-a", "Alpha", parent, task))
	svc := newCheckInService(repo)

	if _, err := svc.Reparent(repo.loc("t1"), &parent.ID); err != nil {
		t.Fatalf("Reparent: %v", err)
	}
	attached := persistedTask(t, repo, "proj-a", "t1")
	if attached.ParentID == nil || *attached.ParentID != parent.ID {
		t.Fatalf("ParentID = %v, want the new parent", attached.ParentID)
	}
	requireScheduleIntact(t, attached, "reparent")

	if _, err := svc.Reparent(repo.loc("t1"), nil); err != nil {
		t.Fatalf("Reparent back to a root: %v", err)
	}
	requireScheduleIntact(t, persistedTask(t, repo, "proj-a", "t1"), "detach back to a root")
}

// TestOutlinePlanPreservesTheCheckIn covers the declarative buffer, which is the
// one surface that restates a task's whole line: an absent field there is a
// clear rather than an omission, and the buffer holds no check-in syntax at all,
// which makes it the path most likely to erase one.
func TestOutlinePlanPreservesTheCheckIn(t *testing.T) {
	task := scheduledTask(t, "t1", 1, "chase the vendor")
	project := newProject("proj-a", "Alpha", task)
	repo := newFakeRepo(project)

	result, err := newCheckInService(repo).ApplyOutlinePlan(app.OutlinePlanRequest{
		ProjectID:        project.ID,
		ExpectedRevision: repo.revisions[project.ID],
		ScopeTaskIDs:     []string{task.ID},
		Lines: []app.OutlinePlanLine{
			{SourceLine: 1, TaskID: task.ID, Depth: 0, Title: "chase the vendor properly", Priority: 2},
		},
	})
	if err != nil {
		t.Fatalf("ApplyOutlinePlan: %v", err)
	}
	if len(result.Updated) != 1 {
		t.Fatalf("updated = %+v, want the one rewritten task", result.Updated)
	}
	requireScheduleIntact(t, persistedTask(t, repo, "proj-a", "t1"), "outline plan")
}

// TestTerminalTransitionsClearTheScheduleThroughEveryPath is the other side of
// preservation. Finishing work is the one thing that retires a schedule, and it
// has to retire it identically however the task got there, because the canonical
// writer refuses a terminal task carrying one and the merge audit records a
// conflict when a merge composes one. The review history is not part of that:
// what is dropped is what is owed, not what happened.
func TestTerminalTransitionsClearTheScheduleThroughEveryPath(t *testing.T) {
	for _, testCase := range []struct {
		name   string
		finish func(*testing.T, *app.Service, *fakeRepo, *models.Project, *models.Task)
	}{
		{"transition", func(t *testing.T, svc *app.Service, repo *fakeRepo, _ *models.Project, _ *models.Task) {
			if _, err := svc.Transition(app.StateRequest{
				Locator: repo.loc("t1"), Target: models.StateDone,
			}); err != nil {
				t.Fatalf("Transition: %v", err)
			}
		}},
		{"bulk transition", func(t *testing.T, svc *app.Service, repo *fakeRepo, _ *models.Project, _ *models.Task) {
			if _, err := svc.BulkTransition([]app.TaskLocator{repo.loc("t1")}, models.StateCanceled); err != nil {
				t.Fatalf("BulkTransition: %v", err)
			}
		}},
		{"line edit destination", func(t *testing.T, svc *app.Service, repo *fakeRepo, _ *models.Project, task *models.Task) {
			done := textgrammar.DestinationDone
			if _, err := svc.EditTaskLine(app.TaskLineEditRequest{
				Source: repo.loc("t1"), Title: task.Title, Destination: &done,
			}); err != nil {
				t.Fatalf("EditTaskLine: %v", err)
			}
		}},
		{"outline destination", func(t *testing.T, svc *app.Service, repo *fakeRepo, project *models.Project, task *models.Task) {
			if _, err := svc.ApplyOutlinePlan(app.OutlinePlanRequest{
				ProjectID:        project.ID,
				ExpectedRevision: repo.revisions[project.ID],
				ScopeTaskIDs:     []string{task.ID},
				Lines: []app.OutlinePlanLine{
					{SourceLine: 1, TaskID: task.ID, Depth: 0, Title: task.Title,
						Destination: textgrammar.DestinationDone},
				},
			}); err != nil {
				t.Fatalf("ApplyOutlinePlan: %v", err)
			}
		}},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			task := scheduledTask(t, "t1", 1, "chase the vendor")
			project := newProject("proj-a", "Alpha", task)
			repo := newFakeRepo(project)
			svc := newCheckInService(repo)

			testCase.finish(t, svc, repo, project, task)

			stored := persistedTask(t, repo, "proj-a", "t1")
			if !stored.IsTerminalState() {
				t.Fatalf("state = %s, want terminal", stored.State())
			}
			if !stored.CheckIn().IsEmpty() {
				t.Errorf("check-in = %v, want terminal work to owe nothing", stored.CheckIn())
			}
			if stored.LastReviewedAt == nil || !stored.LastReviewedAt.Equal(checkInReviewedAt) {
				t.Errorf("LastReviewedAt = %v, want the preserved %v",
					stored.LastReviewedAt, checkInReviewedAt)
			}
		})
	}
}

// TestReopeningInheritsNoCheckIn is that rule's consequence: the schedule was
// gone the moment the task finished, and reopening mints nothing, so a reopened
// task is unscheduled rather than quietly owing a date nobody set.
func TestReopeningInheritsNoCheckIn(t *testing.T) {
	task := scheduledTask(t, "t1", 1, "chase the vendor")
	repo := newFakeRepo(newProject("proj-a", "Alpha", task))
	svc := newCheckInService(repo)

	if _, err := svc.Transition(app.StateRequest{Locator: repo.loc("t1"), Target: models.StateDone}); err != nil {
		t.Fatalf("complete: %v", err)
	}
	if _, err := svc.Transition(app.StateRequest{
		Locator: repo.loc("t1"), Target: models.ReopenTargetState,
	}); err != nil {
		t.Fatalf("reopen: %v", err)
	}
	stored := persistedTask(t, repo, "proj-a", "t1")
	if !stored.CheckIn().IsEmpty() {
		t.Fatalf("check-in = %v, want a reopened task to inherit nothing", stored.CheckIn())
	}
	if stored.LastReviewedAt == nil {
		t.Error("reopening dropped the review history completion had preserved")
	}
}

// TestCaptureCarriesAScheduleAndRefusesATerminalOne is creation's half of the
// contract: a new task may start scheduled, and the destination it settles on
// decides whether it may, on the same terms every other path uses.
func TestCaptureCarriesAScheduleAndRefusesATerminalOne(t *testing.T) {
	repo := newFakeRepo(newProject("proj-a", "Alpha"))
	svc := newCheckInService(repo)

	result, err := svc.Capture(app.CaptureRequest{
		ProjectID: "proj-a",
		Title:     "chase the vendor",
		CheckIn:   checkInSchedule(2, 14),
	})
	if err != nil {
		t.Fatalf("Capture: %v", err)
	}
	created := persistedTask(t, repo, "proj-a", result.TaskID)
	want := checkInToday.AddDate(0, 0, 2)
	if created.NextCheckIn == nil ||
		created.NextCheckIn.Format(time.DateOnly) != want.Format(time.DateOnly) ||
		created.CheckInRepeatDays != 14 {
		t.Errorf("check-in = %v every %d days, want %v every 14",
			created.NextCheckIn, created.CheckInRepeatDays, want)
	}
	// Scheduling a new task is not reviewing it, so creation mints no history.
	if created.LastReviewedAt != nil {
		t.Errorf("LastReviewedAt = %v, want nothing minted at creation", created.LastReviewedAt)
	}

	commits := len(repo.commits)
	_, err = svc.Capture(app.CaptureRequest{
		ProjectID:   "proj-a",
		Title:       "already finished",
		Destination: textgrammar.DestinationDone,
		CheckIn:     checkInSchedule(2, 0),
	})
	if !errors.Is(err, app.ErrInvalidInput) {
		t.Fatalf("terminal capture = %v, want ErrInvalidInput", err)
	}
	if len(repo.commits) != commits {
		t.Errorf("commits = %d, want the %d a refused capture left", len(repo.commits), commits)
	}
}

// TestRepeatDraftCarriesNoScheduleOrHistory is repetition's own boundary,
// checked against a source that carries both: a repeat creates fresh work, and
// fresh work owes nothing and has never been reviewed. TaskRepeatDraft's field
// set is enumerated elsewhere, which is what makes the absence structural; this
// is the behavioral half, over a source that has something to leak.
func TestRepeatDraftCarriesNoScheduleOrHistory(t *testing.T) {
	source := scheduledTask(t, "t1", 1, "ship the quarterly report")
	repo := newFakeRepo(newProject("proj-a", "Alpha", source))
	svc := newCheckInService(repo)

	draft, err := svc.PrepareTaskRepeatDraft(repo.loc("t1"))
	if err != nil {
		t.Fatalf("PrepareTaskRepeatDraft: %v", err)
	}
	result, err := svc.Capture(app.CaptureRequest{
		ProjectID:   "proj-a",
		Title:       draft.Title,
		Description: draft.Description,
		Priority:    draft.Priority,
		Tags:        draft.Tags,
		Context:     draft.Context,
		Destination: draft.Destination,
	})
	if err != nil {
		t.Fatalf("Capture the draft: %v", err)
	}

	repeated := persistedTask(t, repo, "proj-a", result.TaskID)
	if !repeated.CheckIn().IsEmpty() {
		t.Errorf("repeated check-in = %v, want fresh work to owe nothing", repeated.CheckIn())
	}
	if repeated.LastReviewedAt != nil {
		t.Errorf("repeated LastReviewedAt = %v, want no inherited history", repeated.LastReviewedAt)
	}
	// And the source it was repeated from keeps its own.
	requireScheduleIntact(t, persistedTask(t, repo, "proj-a", "t1"), "repeat source")
}
