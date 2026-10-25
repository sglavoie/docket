package app_test

import (
	"errors"
	"testing"
	"time"

	"github.com/sglavoie/docket/internal/app"
	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/storage"
)

// TestStepDueDateCommitsMixedDestinationsAsOneTransaction is the case a loop
// over SetDueDate cannot express: three targets in two projects landing on a
// date, on today, and on no deadline at all, with the Today planning half and
// the strict read-set guards riding the same transaction.
func TestStepDueDateCommitsMixedDestinationsAsOneTransaction(t *testing.T) {
	ordinary := taskWithDue("ordinary", 1, "Ordinary", dueAt(time.UTC, 2026, time.July, 20, 7, 5))
	ordinary.Priority = 2
	ordinary.Tags = []string{"kitchen"}
	ordinary.Context = "home"
	ordinary.Pinned = true
	ordinary.Checklist = []models.ChecklistItem{{Text: "first", Done: true}}
	createdAt := ordinary.CreatedAt
	cleared := taskWithDue("cleared", 2, "Cleared", dueAt(time.UTC, 2026, time.July, 9, 0, 0))
	alpha := newProject("alpha", "Alpha", ordinary, cleared)

	arriving := taskWithDue("arriving", 1, "Arriving", nil)
	arriving.ApplyWorkflowState(models.StateWaiting)
	deferred := time.Date(2026, time.July, 30, 0, 0, 0, 0, time.UTC)
	arriving.DeferredUntil = &deferred
	beta := newProject("beta", "Beta", arriving)

	env := newDueStepEnv(referenceNoon, alpha, beta)
	env.planning.AddToNext(arriving.ID, beta.ID)

	result, err := env.step(app.DueStepForward, nil,
		env.repo.loc(ordinary.ID), env.repo.loc(cleared.ID), env.repo.loc(arriving.ID))
	if err != nil {
		t.Fatalf("step: %v", err)
	}

	t.Run("every target lands where the sequence says", func(t *testing.T) {
		if got := dueString(ordinary.DueDate); got != "2026-07-21 07:05" {
			t.Fatalf("ordinary deadline = %s, want one dated day with its clock", got)
		}
		if cleared.DueDate != nil {
			t.Fatalf("cleared deadline = %s, want unset", dueString(cleared.DueDate))
		}
		if got := dueString(arriving.DueDate); got != "2026-07-10 00:00" {
			t.Fatalf("arriving deadline = %s, want today", got)
		}
	})

	t.Run("only the target landing on today takes the planning half", func(t *testing.T) {
		if arriving.State() != models.StateBacklog || arriving.DeferredUntil != nil {
			t.Fatalf("arriving state/defer = %s/%v, want backlog with no deferral", arriving.State(), arriving.DeferredUntil)
		}
		today := env.planning.GetToday()
		if len(today) != 1 || today[0].TaskID != arriving.ID || len(env.planning.GetNext()) != 0 {
			t.Fatalf("planning = Today %+v Next %+v", today, env.planning.GetNext())
		}
	})

	t.Run("both projects commit as one transaction and one ledger entry", func(t *testing.T) {
		assertOneMixedStepTransaction(t, env)
	})

	t.Run("the result reports every target as committed evidence", func(t *testing.T) {
		assertMixedStepEvidence(t, result)
	})

	t.Run("fields unrelated to the step are untouched", func(t *testing.T) {
		assertUnrelatedFieldsHeld(t, ordinary, createdAt)
	})
}

// assertOneMixedStepTransaction holds the mixed selection to one repository
// transaction over both projects, carrying the one planning artifact and one
// shared ledger entry that make it a single undoable act.
func assertOneMixedStepTransaction(t *testing.T, env dueStepEnv) {
	t.Helper()
	if len(env.repo.commits) != 1 || len(env.repo.commits[0]) != 2 {
		t.Fatalf("commits = %+v, want one transaction over two projects", env.repo.commits)
	}
	artifacts := 0
	for _, changeset := range env.repo.commits[0] {
		artifacts += len(changeset.Artifacts)
		if changeset.UndoAction != "Step Due Date" || changeset.UndoContext != "+1 day" {
			t.Fatalf("ledger entry = %q/%q, want one shared undo entry", changeset.UndoAction, changeset.UndoContext)
		}
	}
	if artifacts != 1 {
		t.Fatalf("planning artifacts in the transaction = %d, want one", artifacts)
	}
}

// assertMixedStepEvidence checks the result carries one project-qualified
// outcome per target, including the cleared one whose after side is absent.
func assertMixedStepEvidence(t *testing.T, result app.TaskOperationResult) {
	t.Helper()
	if len(result.DueSteps) != 3 || len(result.TouchedProjectIDs) != 2 {
		t.Fatalf("outcomes/projects = %d/%d, want three over two", len(result.DueSteps), len(result.TouchedProjectIDs))
	}
	for _, outcome := range result.DueSteps {
		if outcome.Target.ProjectID == "" || outcome.Target.TaskID == "" {
			t.Fatalf("outcome %+v is not project-qualified", outcome)
		}
	}
	if dueString(result.DueSteps[1].Before) != "2026-07-09 00:00" || result.DueSteps[1].After != nil {
		t.Fatalf("cleared outcome = %+v, want a reported before and no after", result.DueSteps[1])
	}
}

// assertUnrelatedFieldsHeld checks a stepped task keeps everything the deadline
// effects have no business touching, so the operation is a deadline move rather
// than a rewrite of the task.
func assertUnrelatedFieldsHeld(t *testing.T, task *models.Task, createdAt time.Time) {
	t.Helper()
	if task.Priority != 2 || task.Context != "home" || !task.Pinned || task.CreatedAt != createdAt {
		t.Fatalf("unrelated fields on %s changed", task.Title)
	}
	if len(task.Tags) != 1 || task.Tags[0] != "kitchen" ||
		len(task.Checklist) != 1 || !task.Checklist[0].Done {
		t.Fatalf("unrelated collections on %s changed: tags=%v checklist=%+v", task.Title, task.Tags, task.Checklist)
	}
	if task.State() != models.StateBacklog {
		t.Fatalf("state = %s, want a step onto an ordinary date to leave it alone", task.State())
	}
}

// TestStepDueDateGuardsEveryReadProject proves the read set is guarded rather
// than merged: every project the calculation consulted carries a strict
// revision precondition on the transaction, naming the revision it was read at.
func TestStepDueDateGuardsEveryReadProject(t *testing.T) {
	first := taskWithDue("first", 1, "First", dueAt(time.UTC, 2026, time.July, 20, 0, 0))
	second := taskWithDue("second", 1, "Second", dueAt(time.UTC, 2026, time.July, 21, 0, 0))
	alpha := newProject("alpha", "Alpha", first)
	beta := newProject("beta", "Beta", second)
	env := newDueStepEnv(referenceNoon, alpha, beta)
	loaded := map[string]string{alpha.ID: env.repo.revisions[alpha.ID], beta.ID: env.repo.revisions[beta.ID]}

	if _, err := env.step(app.DueStepForward, nil, env.repo.loc(first.ID), env.repo.loc(second.ID)); err != nil {
		t.Fatalf("step: %v", err)
	}
	guards := map[string]string{}
	for _, changeset := range env.repo.commits[0] {
		for _, precondition := range changeset.ProjectPreconditions {
			guards[precondition.ProjectID] = precondition.ExpectedRevision
		}
	}
	if len(guards) != 2 {
		t.Fatalf("guards = %+v, want one per read project", guards)
	}
	for id, revision := range loaded {
		if guards[id] != revision {
			t.Fatalf("guard for %s = %q, want the loaded revision %q", id, guards[id], revision)
		}
	}
}

func TestStepDueDateExpiresOnAStaleReadProject(t *testing.T) {
	first := taskWithDue("first", 1, "First", dueAt(time.UTC, 2026, time.July, 20, 0, 0))
	second := taskWithDue("second", 1, "Second", dueAt(time.UTC, 2026, time.July, 21, 0, 0))
	alpha := newProject("alpha", "Alpha", first)
	beta := newProject("beta", "Beta", second)
	env := newDueStepEnv(referenceNoon, alpha, beta)
	env.repo.preCommit = func() { env.repo.revisions[beta.ID] = "moved-by-another-machine" }

	_, err := env.step(app.DueStepForward, nil, env.repo.loc(first.ID), env.repo.loc(second.ID))
	if !errors.Is(err, storage.ErrStaleRevision) {
		t.Fatalf("step error = %v, want ErrStaleRevision", err)
	}
	var conflict *storage.RevisionConflictError
	if !errors.As(err, &conflict) || conflict.ProjectID != beta.ID {
		t.Fatalf("conflict = %#v, want beta named", conflict)
	}
	if got := dueString(first.DueDate); got != "2026-07-20 00:00" {
		t.Fatalf("alpha deadline = %s, want it rolled back", got)
	}
	if got := dueString(second.DueDate); got != "2026-07-21 00:00" {
		t.Fatalf("beta deadline = %s, want it rolled back", got)
	}
}

// TestStepDueDateRejectsATerminalTaskLandingOnToday holds the step to the
// existing Today refusal, and only where Today is actually entered: a terminal
// task whose destination is any other date keeps stepping.
func TestStepDueDateRejectsATerminalTaskLandingOnToday(t *testing.T) {
	t.Run("landing on today rejects the whole selection", func(t *testing.T) {
		finished := taskWithDue("finished", 1, "Finished", nil)
		finished.ApplyWorkflowState(models.StateDone)
		open := taskWithDue("open", 2, "Open", dueAt(time.UTC, 2026, time.July, 20, 0, 0))
		env := newDueStepEnv(referenceNoon, newProject("project", "Project", finished, open))

		_, err := env.step(app.DueStepForward, nil, env.repo.loc(finished.ID), env.repo.loc(open.ID))
		if !errors.Is(err, app.ErrInvalidInput) {
			t.Fatalf("step error = %v, want ErrInvalidInput", err)
		}
		if finished.DueDate != nil || dueString(open.DueDate) != "2026-07-20 00:00" {
			t.Fatalf("deadlines = %s / %s, want both unchanged", dueString(finished.DueDate), dueString(open.DueDate))
		}
		if len(env.repo.commits) != 0 || len(env.planning.GetToday()) != 0 {
			t.Fatalf("commits/Today = %d/%+v, want nothing written", len(env.repo.commits), env.planning.GetToday())
		}
	})

	t.Run("any other destination still steps", func(t *testing.T) {
		finished := taskWithDue("finished", 1, "Finished", dueAt(time.UTC, 2026, time.July, 20, 0, 0))
		finished.ApplyWorkflowState(models.StateDone)
		env := newDueStepEnv(referenceNoon, newProject("project", "Project", finished))

		if _, err := env.step(app.DueStepForward, nil, env.repo.loc(finished.ID)); err != nil {
			t.Fatalf("step: %v", err)
		}
		if got := dueString(finished.DueDate); got != "2026-07-21 00:00" {
			t.Fatalf("deadline = %s, want an ordinary dated step", got)
		}
		if finished.State() != models.StateDone {
			t.Fatalf("state = %s, want the terminal state preserved", finished.State())
		}
	})
}

func TestStepDueDateRollsBackAFailedPlanningArtifact(t *testing.T) {
	arriving := taskWithDue("arriving", 1, "Arriving", nil)
	arriving.ApplyWorkflowState(models.StateWaiting)
	companion := taskWithDue("companion", 2, "Companion", dueAt(time.UTC, 2026, time.July, 20, 0, 0))
	project := newProject("project", "Project", arriving, companion)
	repo := newFakeRepo(project)
	builderErr := errors.New("planning store unavailable")
	svc := newTestServiceWithClock(repo, fixedClock{t: referenceNoon}).WithPlanningArtifacts(
		func(app.PlanningReconcileRequest) ([]storage.ArtifactChange, error) { return nil, builderErr },
	)

	_, err := svc.ExecuteTaskOperation(app.TaskOperationRequest{
		Operation: app.TaskOperationStepDueDate,
		Targets:   []app.TaskLocator{repo.loc(arriving.ID), repo.loc(companion.ID)},
		DueStep:   app.DueStepRequest{Direction: app.DueStepForward},
	})
	if !errors.Is(err, builderErr) {
		t.Fatalf("step error = %v, want the builder failure", err)
	}
	if arriving.DueDate != nil || arriving.State() != models.StateWaiting {
		t.Fatalf("arriving = %s in %s, want it rolled back", dueString(arriving.DueDate), arriving.State())
	}
	if got := dueString(companion.DueDate); got != "2026-07-20 00:00" {
		t.Fatalf("companion deadline = %s, want it rolled back", got)
	}
	if len(repo.commits) != 0 {
		t.Fatalf("commits = %d, want none", len(repo.commits))
	}
}

func TestStepDueDateRollsBackAFailedCommit(t *testing.T) {
	arriving := taskWithDue("arriving", 1, "Arriving", nil)
	arriving.ApplyWorkflowState(models.StateWaiting)
	moved := taskWithDue("moved", 2, "Moved", dueAt(time.UTC, 2026, time.July, 20, 11, 0))
	env := newDueStepEnv(referenceNoon, newProject("project", "Project", arriving, moved))
	env.repo.commitErr = errors.New("disk is gone")

	_, err := env.step(app.DueStepForward, nil, env.repo.loc(arriving.ID), env.repo.loc(moved.ID))
	if !errors.Is(err, env.repo.commitErr) {
		t.Fatalf("step error = %v, want the commit failure", err)
	}
	if arriving.DueDate != nil || arriving.State() != models.StateWaiting || arriving.DeferredUntil != nil {
		t.Fatalf("arriving = %s in %s, want it rolled back", dueString(arriving.DueDate), arriving.State())
	}
	if got := dueString(moved.DueDate); got != "2026-07-20 11:00" {
		t.Fatalf("moved deadline = %s, want it rolled back", got)
	}
}

// TestStepDueDateHoldsEveryTargetAtTheCommitBarrier proves the operation is
// observable at the one seam a cross-machine race is measured from: every
// resolved target is named, with the revision its project's write is guarded by,
// while the transaction is still unwritten.
func TestStepDueDateHoldsEveryTargetAtTheCommitBarrier(t *testing.T) {
	first := taskWithDue("first", 1, "First", dueAt(time.UTC, 2026, time.July, 20, 0, 0))
	second := taskWithDue("second", 1, "Second", dueAt(time.UTC, 2026, time.July, 21, 0, 0))
	alpha := newProject("alpha", "Alpha", first)
	beta := newProject("beta", "Beta", second)
	env := newDueStepEnv(referenceNoon, alpha, beta)
	loaded := map[string]string{alpha.ID: env.repo.revisions[alpha.ID], beta.ID: env.repo.revisions[beta.ID]}

	type held struct{ action, project, task, revision string }
	var holds []held
	commitsWhenHeld := -1
	app.CommitBarrier = func(action, projectID, taskID, expectedRevision string) {
		holds = append(holds, held{action, projectID, taskID, expectedRevision})
		commitsWhenHeld = len(env.repo.commits)
	}
	t.Cleanup(func() { app.CommitBarrier = nil })

	if _, err := env.step(app.DueStepForward, nil, env.repo.loc(first.ID), env.repo.loc(second.ID)); err != nil {
		t.Fatalf("step: %v", err)
	}
	if commitsWhenHeld != 0 {
		t.Fatalf("commits observed at the barrier = %d, want the transaction still unwritten", commitsWhenHeld)
	}
	want := []held{
		{"Step Due Date", alpha.ID, first.ID, loaded[alpha.ID]},
		{"Step Due Date", beta.ID, second.ID, loaded[beta.ID]},
	}
	if len(holds) != len(want) {
		t.Fatalf("barrier holds = %+v, want one per target", holds)
	}
	for i := range want {
		if holds[i] != want[i] {
			t.Fatalf("barrier hold %d = %+v, want %+v", i, holds[i], want[i])
		}
	}
}
