package app_test

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/sglavoie/docket/internal/app"
	"github.com/sglavoie/docket/internal/models"
)

// checkInNow is the instant every service in this file runs at, and checkInToday
// the calendar day it falls on. A check-in is a calendar date, so every schedule
// below is built from the day rather than the instant — the model's own gate
// refuses one carrying a time.
var (
	checkInNow   = time.Date(2026, 7, 10, 12, 0, 0, 0, time.UTC)
	checkInToday = time.Date(2026, 7, 10, 0, 0, 0, 0, time.UTC)
)

// checkInSchedule builds a schedule dayOffset days from checkInToday.
func checkInSchedule(dayOffset, repeatDays int) models.CheckInSchedule {
	next := checkInToday.AddDate(0, 0, dayOffset)
	return models.CheckInSchedule{Next: &next, RepeatDays: repeatDays}
}

// setCheckIn runs the operation through the one application dispatcher, so every
// test exercises the path a presentation takes rather than the bulk primitive
// underneath it.
func setCheckIn(svc *app.Service, schedule models.CheckInSchedule, targets ...app.TaskLocator) (app.TaskOperationResult, error) {
	return svc.ExecuteTaskOperation(app.TaskOperationRequest{
		Operation: app.TaskOperationSetCheckIn,
		Targets:   targets,
		CheckIn:   schedule,
	})
}

func newCheckInService(repo *fakeRepo) *app.Service {
	return newTestServiceWithClock(repo, fixedClock{t: checkInNow})
}

// TestSetCheckInWritesOneScheduleAcrossTheSelection is the operation's ordinary
// case: one value, every target, one transaction, one undo entry naming what was
// committed rather than the operation that produced it.
func TestSetCheckInWritesOneScheduleAcrossTheSelection(t *testing.T) {
	first := newTask("t1", 1, "chase the vendor")
	second := newTask("t2", 2, "chase the other vendor")
	repo := newFakeRepo(newProject("proj-a", "Alpha", first, second))
	svc := newCheckInService(repo)

	result, err := setCheckIn(svc, checkInSchedule(3, 7), repo.loc("t1"), repo.loc("t2"))
	if err != nil {
		t.Fatalf("SetCheckIn: %v", err)
	}
	if len(result.Tasks) != 2 || len(result.TouchedProjectIDs) != 1 || result.TransactionID == "" {
		t.Fatalf("result = %+v, want two tasks in one project under one transaction", result)
	}
	want := checkInToday.AddDate(0, 0, 3)
	for _, task := range []*models.Task{first, second} {
		if task.NextCheckIn == nil || !task.NextCheckIn.Equal(want) || task.CheckInRepeatDays != 7 {
			t.Errorf("%s check-in = %v every %d days, want %v every 7",
				task.ID, task.NextCheckIn, task.CheckInRepeatDays, want)
		}
		if !task.UpdatedAt.Equal(checkInNow) {
			t.Errorf("%s UpdatedAt = %v, want the operation clock %v", task.ID, task.UpdatedAt, checkInNow)
		}
	}
	committed, ok := repo.lastCommit()
	if !ok || committed.UndoAction != "Set Check-In" {
		t.Fatalf("undo action = %q, want Set Check-In", committed.UndoAction)
	}
	if committed.UndoContext != "2026-07-13 every 7 days" {
		t.Errorf("undo context = %q, want the committed value", committed.UndoContext)
	}
	// Scheduling is not planning: nothing here writes Today or Next, so the
	// transaction carries no planning artifact at all.
	if len(committed.Artifacts) != 0 {
		t.Errorf("artifacts = %+v, want a schedule to touch no planning", committed.Artifacts)
	}
}

// TestSetCheckInIsNotAnAcknowledgement keeps the two acts apart. Deciding when to
// look at something again says nothing about having looked at it now, so the
// review flag, the review history, the lifecycle state, and its clock all survive
// a schedule being written over them.
func TestSetCheckInIsNotAnAcknowledgement(t *testing.T) {
	task := newTask("t1", 1, "chase the vendor")
	task.Review = true
	task.ApplyWorkflowState(models.StateWaiting)
	stateChanged := checkInNow.AddDate(0, 0, -21)
	task.StateChangedAt = stateChanged
	reviewed := checkInNow.AddDate(0, 0, -5)
	task.LastReviewedAt = &reviewed
	repo := newFakeRepo(newProject("proj-a", "Alpha", task))

	if _, err := setCheckIn(newCheckInService(repo), checkInSchedule(2, 0), repo.loc("t1")); err != nil {
		t.Fatalf("SetCheckIn: %v", err)
	}
	if !task.Review {
		t.Error("scheduling cleared the review flag")
	}
	if task.LastReviewedAt == nil || !task.LastReviewedAt.Equal(reviewed) {
		t.Errorf("LastReviewedAt = %v, want the untouched %v", task.LastReviewedAt, reviewed)
	}
	if !task.IsInState(models.StateWaiting) || !task.StateChangedAt.Equal(stateChanged) {
		t.Errorf("state = %s at %v, want the untouched Waiting at %v",
			task.State(), task.StateChangedAt, stateChanged)
	}
}

// TestSetCheckInRefusesTheWholeBatchForOneTerminalTarget is the all-or-nothing
// rule where it matters most: a selection that half-landed would leave the caller
// no way to tell which half did.
func TestSetCheckInRefusesTheWholeBatchForOneTerminalTarget(t *testing.T) {
	open := newTask("t1", 1, "chase the vendor")
	done := newTask("t2", 2, "shipped it")
	done.ApplyWorkflowState(models.StateDone)
	repo := newFakeRepo(newProject("proj-a", "Alpha", open, done))
	svc := newCheckInService(repo)
	before := len(repo.commits)

	_, err := setCheckIn(svc, checkInSchedule(3, 0), repo.loc("t1"), repo.loc("t2"))
	if !errors.Is(err, app.ErrInvalidInput) {
		t.Fatalf("SetCheckIn = %v, want ErrInvalidInput", err)
	}
	// Named, not merely refused: the caller has to learn which target it was.
	if got := err.Error(); !strings.Contains(got, "#2") {
		t.Errorf("error = %q, want the refusing target named", got)
	}
	if !open.CheckIn().IsEmpty() {
		t.Errorf("innocent target = %v, want nothing written by a refused batch", open.CheckIn())
	}
	if len(repo.commits) != before {
		t.Errorf("commits = %d, want the %d it started with", len(repo.commits), before)
	}
}

// TestSetCheckInClearsAndAcceptsATerminalMember is the clear's own shape. The
// empty schedule is the clear rather than a second operation, and a terminal
// member cannot refuse it: a finished task holds no schedule, so clearing one is
// a no-op, and refusing would mean a selection containing one finished task could
// never be cleared at all.
func TestSetCheckInClearsAndAcceptsATerminalMember(t *testing.T) {
	scheduled := newTask("t1", 1, "chase the vendor")
	if err := scheduled.SetCheckIn(checkInSchedule(4, 14), checkInNow.AddDate(0, 0, -1)); err != nil {
		t.Fatalf("seed check-in: %v", err)
	}
	done := newTask("t2", 2, "shipped it")
	done.ApplyWorkflowState(models.StateDone)
	repo := newFakeRepo(newProject("proj-a", "Alpha", scheduled, done))

	if _, err := setCheckIn(
		newCheckInService(repo), models.CheckInSchedule{}, repo.loc("t1"), repo.loc("t2"),
	); err != nil {
		t.Fatalf("clear check-in: %v", err)
	}
	if !scheduled.CheckIn().IsEmpty() {
		t.Errorf("check-in = %v, want it cleared whole", scheduled.CheckIn())
	}
	committed, ok := repo.lastCommit()
	if !ok || committed.UndoAction != "Clear Check-In" {
		t.Fatalf("undo action = %q, want Clear Check-In", committed.UndoAction)
	}
}

// TestSetCheckInRefusesAnUnmintableScheduleBeforeResolving keeps the value gate
// the model's, and keeps it ahead of everything else: a schedule no surface may
// write is refused before a target is resolved, so the refusal names the value
// rather than a task.
func TestSetCheckInRefusesAnUnmintableScheduleBeforeResolving(t *testing.T) {
	timed := checkInNow
	for _, testCase := range []struct {
		name     string
		schedule models.CheckInSchedule
	}{
		{"repeat with no date", models.CheckInSchedule{RepeatDays: 7}},
		{"date carrying a time", models.CheckInSchedule{Next: &timed}},
		{"repeat past the bound", checkInSchedule(1, models.MaxCheckInRepeatDays+1)},
		{"negative repeat", checkInSchedule(1, -1)},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			task := newTask("t1", 1, "chase the vendor")
			repo := newFakeRepo(newProject("proj-a", "Alpha", task))

			_, err := setCheckIn(newCheckInService(repo), testCase.schedule, repo.loc("t1"))
			if !errors.Is(err, app.ErrInvalidInput) {
				t.Fatalf("SetCheckIn = %v, want ErrInvalidInput", err)
			}
			if !task.CheckIn().IsEmpty() || len(repo.commits) != 0 {
				t.Errorf("refused schedule still wrote %v across %d commits",
					task.CheckIn(), len(repo.commits))
			}
			if repo.snapshots != 0 {
				t.Errorf("snapshots = %d, want the value refused before any target is resolved",
					repo.snapshots)
			}
		})
	}
}

// TestSetCheckInRefusesAStaleTargetWhole is the other half of the target set
// being judged before any of it is written: a locator naming a task the workspace
// no longer holds refuses the batch rather than quietly scheduling its neighbors.
func TestSetCheckInRefusesAStaleTargetWhole(t *testing.T) {
	present := newTask("t1", 1, "chase the vendor")
	repo := newFakeRepo(newProject("proj-a", "Alpha", present))
	svc := newCheckInService(repo)

	_, err := setCheckIn(svc, checkInSchedule(3, 0),
		repo.loc("t1"), app.TaskLocator{ProjectID: "proj-a", TaskID: "gone"})
	if !errors.Is(err, app.ErrTaskNotFound) {
		t.Fatalf("SetCheckIn = %v, want ErrTaskNotFound", err)
	}
	if !present.CheckIn().IsEmpty() || len(repo.commits) != 0 {
		t.Errorf("stale target still wrote %v across %d commits", present.CheckIn(), len(repo.commits))
	}
}

// TestSetCheckInReadsTheClockOnceForTheBatch pins the reading rather than the
// value. A schedule written across midnight must not touch half its targets on
// one day and half on the next, so an advancing clock has to leave every member
// carrying the same stamp.
func TestSetCheckInReadsTheClockOnceForTheBatch(t *testing.T) {
	first := newTask("t1", 1, "first")
	second := newTask("t2", 2, "second")
	third := newTask("t3", 3, "third")
	repo := newFakeRepo(newProject("proj-a", "Alpha", first, second, third))
	svc := newTestServiceWithClock(repo, &advancingClock{t: checkInNow, step: time.Hour})

	if _, err := setCheckIn(svc, checkInSchedule(5, 0),
		repo.loc("t1"), repo.loc("t2"), repo.loc("t3")); err != nil {
		t.Fatalf("SetCheckIn: %v", err)
	}
	if !first.UpdatedAt.Equal(second.UpdatedAt) || !second.UpdatedAt.Equal(third.UpdatedAt) {
		t.Fatalf("UpdatedAt = %v / %v / %v, want one reading of the clock",
			first.UpdatedAt, second.UpdatedAt, third.UpdatedAt)
	}
}

// TestSetCheckInCommitsEveryTouchedProjectAsOneTransaction is the cross-project
// case: a selection spanning projects is still one act, so it is one transaction
// and one undo entry rather than one per project.
func TestSetCheckInCommitsEveryTouchedProjectAsOneTransaction(t *testing.T) {
	alphaTask := newTask("t1", 1, "alpha work")
	betaTask := newTask("t2", 1, "beta work")
	repo := newFakeRepo(
		newProject("proj-a", "Alpha", alphaTask),
		newProject("proj-b", "Beta", betaTask))
	svc := newCheckInService(repo)

	result, err := setCheckIn(svc, checkInSchedule(2, 0), repo.loc("t1"), repo.loc("t2"))
	if err != nil {
		t.Fatalf("SetCheckIn: %v", err)
	}
	if len(result.TouchedProjectIDs) != 2 {
		t.Fatalf("touched projects = %v, want both", result.TouchedProjectIDs)
	}
	if len(repo.commits) != 1 || len(repo.commits[0]) != 2 {
		t.Fatalf("commits = %d call(s) of %d changeset(s), want one call of two",
			len(repo.commits), len(repo.commits[0]))
	}
	if alphaTask.CheckIn().IsEmpty() || betaTask.CheckIn().IsEmpty() {
		t.Error("a cross-project batch left a member unscheduled")
	}
}

// TestSetCheckInRollsBackAFailedCommit keeps a refusal at the storage boundary
// from leaving the in-memory workspace ahead of the durable one: the snapshot the
// mutation ran against is restored, so the next read sees what disk holds.
func TestSetCheckInRollsBackAFailedCommit(t *testing.T) {
	scheduled := newTask("t1", 1, "chase the vendor")
	if err := scheduled.SetCheckIn(checkInSchedule(1, 3), checkInNow.AddDate(0, 0, -1)); err != nil {
		t.Fatalf("seed check-in: %v", err)
	}
	before := scheduled.CheckIn()
	repo := newFakeRepo(newProject("proj-a", "Alpha", scheduled))
	repo.commitErr = errors.New("disk is on fire")

	if _, err := setCheckIn(newCheckInService(repo), checkInSchedule(9, 21), repo.loc("t1")); err == nil {
		t.Fatal("a failed commit reported success")
	}
	// The rollback restores from the document the snapshot was cloned into, which
	// re-reads the date in the local zone, so the restored schedule is compared as
	// the calendar day it is rather than as an instant.
	restored := repo.projects[0].Tasks[0].CheckIn()
	if restored.RepeatDays != before.RepeatDays || restored.Next == nil ||
		restored.Next.Format(time.DateOnly) != before.Next.Format(time.DateOnly) {
		t.Fatalf("check-in = %v after a failed commit, want the restored %v", restored, before)
	}
}
