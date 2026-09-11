package app_test

import (
	"errors"
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

// referenceNoon is the operation clock every sequence test anchors to, so
// "today" is 2026-07-10 and "yesterday" is 2026-07-09 throughout.
var referenceNoon = time.Date(2026, time.July, 10, 12, 0, 0, 0, time.UTC)

// dueStepEnv is the whole fixture a deadline step needs: projects behind a fake
// repository, a fixed operation clock that fixes the reference day, and the
// in-memory planning state the Today half of a step is recorded in.
type dueStepEnv struct {
	repo     *fakeRepo
	svc      *app.Service
	planning *config.PlanningState
}

func newDueStepEnv(now time.Time, projects ...*models.Project) dueStepEnv {
	repo := newFakeRepo(projects...)
	planning := config.NewPlanningState()
	svc := newTestServiceWithClock(repo, fixedClock{t: now}).WithPlanningArtifacts(memoryPlanningBuilder(planning))
	return dueStepEnv{repo: repo, svc: svc, planning: planning}
}

func (e dueStepEnv) step(direction app.DueStepDirection, clocks map[app.TaskLocator]app.DueStepClock, targets ...app.TaskLocator) (app.TaskOperationResult, error) {
	return e.svc.ExecuteTaskOperation(app.TaskOperationRequest{
		Operation: app.TaskOperationStepDueDate,
		Targets:   targets,
		DueStep:   app.DueStepRequest{Direction: direction, RememberedClocks: clocks},
	})
}

// dueString renders a deadline the way these tests compare them: the calendar
// day and the wall clock together, with "unset" for no deadline at all, so a
// preserved clock and a cleared one are visible in the failure message.
func dueString(due *time.Time) string {
	if due == nil {
		return "unset"
	}
	return due.Format("2006-01-02 15:04")
}

// taskWithDue builds a leaf task whose deadline is the given local components,
// or no deadline when day is zero.
func taskWithDue(id string, shortID int, title string, due *time.Time) *models.Task {
	task := newTask(id, shortID, title)
	task.DueDate = due
	return task
}

func dueAt(loc *time.Location, year int, month time.Month, day, hour, minute int) *time.Time {
	value := time.Date(year, month, day, hour, minute, 0, 0, loc)
	return &value
}

func TestStepDueDateWalksTheDeadlineSequenceInBothDirections(t *testing.T) {
	cases := []struct {
		name      string
		start     *time.Time
		direction app.DueStepDirection
		want      string
	}{
		{"unset steps right onto today", nil, app.DueStepForward, "2026-07-10 00:00"},
		{"unset steps left onto yesterday", nil, app.DueStepBackward, "2026-07-09 00:00"},
		{"yesterday steps right onto unset", dueAt(time.UTC, 2026, time.July, 9, 0, 0), app.DueStepForward, "unset"},
		{"today steps left onto unset", dueAt(time.UTC, 2026, time.July, 10, 0, 0), app.DueStepBackward, "unset"},
		{"today steps right onto tomorrow", dueAt(time.UTC, 2026, time.July, 10, 0, 0), app.DueStepForward, "2026-07-11 00:00"},
		{"tomorrow steps left onto today", dueAt(time.UTC, 2026, time.July, 11, 0, 0), app.DueStepBackward, "2026-07-10 00:00"},
		{"yesterday steps left one further day", dueAt(time.UTC, 2026, time.July, 9, 0, 0), app.DueStepBackward, "2026-07-08 00:00"},
		{"an earlier date steps right toward yesterday", dueAt(time.UTC, 2026, time.July, 8, 0, 0), app.DueStepForward, "2026-07-09 00:00"},
		{"a later date steps right one further day", dueAt(time.UTC, 2026, time.July, 12, 0, 0), app.DueStepForward, "2026-07-13 00:00"},
		{"a weekend day is an ordinary day", dueAt(time.UTC, 2026, time.July, 11, 0, 0), app.DueStepForward, "2026-07-12 00:00"},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			task := taskWithDue("stepped", 1, "Stepped", testCase.start)
			project := newProject("project", "Project", task)
			env := newDueStepEnv(referenceNoon, project)

			result, err := env.step(testCase.direction, nil, env.repo.loc(task.ID))
			if err != nil {
				t.Fatalf("step: %v", err)
			}
			if got := dueString(task.DueDate); got != testCase.want {
				t.Fatalf("committed deadline = %s, want %s", got, testCase.want)
			}
			if len(result.DueSteps) != 1 {
				t.Fatalf("outcomes = %+v, want exactly one", result.DueSteps)
			}
			outcome := result.DueSteps[0]
			if got := dueString(outcome.Before); got != dueString(testCase.start) {
				t.Fatalf("reported before = %s, want %s", got, dueString(testCase.start))
			}
			if got := dueString(outcome.After); got != testCase.want {
				t.Fatalf("reported after = %s, want %s", got, testCase.want)
			}
			if outcome.Target != env.repo.loc(task.ID) {
				t.Fatalf("reported target = %+v, want %+v", outcome.Target, env.repo.loc(task.ID))
			}
			if result.TransactionID == "" || len(env.repo.commits) != 1 {
				t.Fatalf("transaction = %q over %d commits, want one identified transaction", result.TransactionID, len(env.repo.commits))
			}
		})
	}
}

func TestStepDueDatePreservesAndRestoresClocks(t *testing.T) {
	t.Run("a dated step carries its wall clock", func(t *testing.T) {
		task := taskWithDue("timed", 1, "Timed", dueAt(time.UTC, 2026, time.July, 12, 9, 30))
		env := newDueStepEnv(referenceNoon, newProject("project", "Project", task))

		if _, err := env.step(app.DueStepForward, nil, env.repo.loc(task.ID)); err != nil {
			t.Fatalf("step: %v", err)
		}
		if got := dueString(task.DueDate); got != "2026-07-13 09:30" {
			t.Fatalf("deadline = %s, want the clock preserved on 2026-07-13", got)
		}
	})

	t.Run("clearing reports the clock the caller must remember", func(t *testing.T) {
		task := taskWithDue("cleared", 1, "Cleared", dueAt(time.UTC, 2026, time.July, 10, 16, 45))
		env := newDueStepEnv(referenceNoon, newProject("project", "Project", task))

		result, err := env.step(app.DueStepBackward, nil, env.repo.loc(task.ID))
		if err != nil {
			t.Fatalf("step: %v", err)
		}
		if task.DueDate != nil {
			t.Fatalf("deadline = %s, want unset", dueString(task.DueDate))
		}
		if got := dueString(result.DueSteps[0].Before); got != "2026-07-10 16:45" {
			t.Fatalf("reported before = %s, want the cleared clock", got)
		}
	})

	t.Run("a remembered clock is restored on the way out of unset", func(t *testing.T) {
		task := taskWithDue("restored", 1, "Restored", nil)
		env := newDueStepEnv(referenceNoon, newProject("project", "Project", task))
		locator := env.repo.loc(task.ID)

		if _, err := env.step(app.DueStepBackward, map[app.TaskLocator]app.DueStepClock{
			locator: {Hour: 16, Minute: 45},
		}, locator); err != nil {
			t.Fatalf("step: %v", err)
		}
		if got := dueString(task.DueDate); got != "2026-07-09 16:45" {
			t.Fatalf("deadline = %s, want yesterday at the remembered clock", got)
		}
	})

	t.Run("no remembered clock leaves unset for a date-only deadline", func(t *testing.T) {
		task := taskWithDue("dateonly", 1, "Date only", nil)
		env := newDueStepEnv(referenceNoon, newProject("project", "Project", task))
		other := app.TaskLocator{ProjectID: "project", TaskID: "somebody-else"}

		if _, err := env.step(app.DueStepBackward, map[app.TaskLocator]app.DueStepClock{
			other: {Hour: 16, Minute: 45},
		}, env.repo.loc(task.ID)); err != nil {
			t.Fatalf("step: %v", err)
		}
		if got := dueString(task.DueDate); got != "2026-07-09 00:00" {
			t.Fatalf("deadline = %s, want a date-only yesterday", got)
		}
		if task.DueHasTime() {
			t.Fatalf("deadline %s carries a clock, want date-only", dueString(task.DueDate))
		}
	})
}

func TestStepDueDateCrossesCalendarBoundaries(t *testing.T) {
	cases := []struct {
		name      string
		start     *time.Time
		direction app.DueStepDirection
		want      string
	}{
		{"month end", dueAt(time.UTC, 2026, time.January, 31, 0, 0), app.DueStepForward, "2026-02-01 00:00"},
		{"month start", dueAt(time.UTC, 2026, time.March, 1, 0, 0), app.DueStepBackward, "2026-02-28 00:00"},
		{"year end", dueAt(time.UTC, 2026, time.December, 31, 8, 15), app.DueStepForward, "2027-01-01 08:15"},
		{"into a leap day", dueAt(time.UTC, 2028, time.February, 28, 0, 0), app.DueStepForward, "2028-02-29 00:00"},
		{"out of a leap day", dueAt(time.UTC, 2028, time.February, 29, 0, 0), app.DueStepForward, "2028-03-01 00:00"},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			task := taskWithDue("boundary", 1, "Boundary", testCase.start)
			env := newDueStepEnv(referenceNoon, newProject("project", "Project", task))

			if _, err := env.step(testCase.direction, nil, env.repo.loc(task.ID)); err != nil {
				t.Fatalf("step: %v", err)
			}
			if got := dueString(task.DueDate); got != testCase.want {
				t.Fatalf("deadline = %s, want %s", got, testCase.want)
			}
		})
	}
}

// TestStepDueDateRefusesAClockDaylightSavingSkips is the reason the calculation
// reconstructs an instant and reads its components back instead of adding 24
// hours: time.Date normalizes 02:30 on a spring-forward day to 03:30 rather
// than failing, and silently moving a deadline an hour is exactly what a step
// must not do. The refusal covers the whole selection.
func TestStepDueDateRefusesAClockDaylightSavingSkips(t *testing.T) {
	zone, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Fatalf("LoadLocation: %v", err)
	}
	skipped := taskWithDue("skipped", 1, "Skipped", dueAt(zone, 2026, time.March, 7, 2, 30))
	untouched := taskWithDue("untouched", 2, "Untouched", dueAt(zone, 2026, time.March, 20, 9, 0))
	project := newProject("project", "Project", skipped, untouched)
	env := newDueStepEnv(time.Date(2026, time.March, 7, 12, 0, 0, 0, zone), project)

	_, stepErr := env.step(app.DueStepForward, nil, env.repo.loc(skipped.ID), env.repo.loc(untouched.ID))
	if !errors.Is(stepErr, app.ErrInvalidInput) {
		t.Fatalf("step error = %v, want ErrInvalidInput", stepErr)
	}
	if got := dueString(skipped.DueDate); got != "2026-03-07 02:30" {
		t.Fatalf("refused task deadline = %s, want it unchanged", got)
	}
	if got := dueString(untouched.DueDate); got != "2026-03-20 09:00" {
		t.Fatalf("companion deadline = %s, want the whole selection refused", got)
	}
	if len(env.repo.commits) != 0 {
		t.Fatalf("commits = %d, want none", len(env.repo.commits))
	}
}

func TestStepDueDateRejectsMalformedRequests(t *testing.T) {
	task := taskWithDue("target", 1, "Target", nil)
	project := newProject("project", "Project", task)

	cases := []struct {
		name    string
		request app.TaskOperationRequest
	}{
		{"no targets", app.TaskOperationRequest{
			Operation: app.TaskOperationStepDueDate,
			DueStep:   app.DueStepRequest{Direction: app.DueStepForward},
		}},
		{"an unqualified target", app.TaskOperationRequest{
			Operation: app.TaskOperationStepDueDate,
			Targets:   []app.TaskLocator{{TaskID: "target"}},
			DueStep:   app.DueStepRequest{Direction: app.DueStepForward},
		}},
		{"an unknown target", app.TaskOperationRequest{
			Operation: app.TaskOperationStepDueDate,
			Targets:   []app.TaskLocator{{ProjectID: "project", TaskID: "nobody"}},
			DueStep:   app.DueStepRequest{Direction: app.DueStepForward},
		}},
		{"no direction", app.TaskOperationRequest{
			Operation: app.TaskOperationStepDueDate,
			Targets:   []app.TaskLocator{{ProjectID: "project", TaskID: "target"}},
		}},
		{"a two-day stride", app.TaskOperationRequest{
			Operation: app.TaskOperationStepDueDate,
			Targets:   []app.TaskLocator{{ProjectID: "project", TaskID: "target"}},
			DueStep:   app.DueStepRequest{Direction: 2},
		}},
		{"an impossible remembered clock", app.TaskOperationRequest{
			Operation: app.TaskOperationStepDueDate,
			Targets:   []app.TaskLocator{{ProjectID: "project", TaskID: "target"}},
			DueStep: app.DueStepRequest{
				Direction:        app.DueStepForward,
				RememberedClocks: map[app.TaskLocator]app.DueStepClock{{ProjectID: "project", TaskID: "target"}: {Hour: 24}},
			},
		}},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			task.DueDate = nil
			env := newDueStepEnv(referenceNoon, project)

			if _, err := env.svc.ExecuteTaskOperation(testCase.request); err == nil {
				t.Fatalf("step accepted %s", testCase.name)
			}
			if task.DueDate != nil || len(env.repo.commits) != 0 {
				t.Fatalf("deadline = %s over %d commits, want nothing written", dueString(task.DueDate), len(env.repo.commits))
			}
		})
	}
}

// TestStepDueDateStepsADuplicatedTargetOnce holds the operation to the same
// deduplication every bulk path gives: a row named twice is one target, so it
// moves one day rather than two and is reported once.
func TestStepDueDateStepsADuplicatedTargetOnce(t *testing.T) {
	task := taskWithDue("twice", 1, "Twice", dueAt(time.UTC, 2026, time.July, 20, 0, 0))
	env := newDueStepEnv(referenceNoon, newProject("project", "Project", task))
	locator := env.repo.loc(task.ID)

	result, err := env.step(app.DueStepForward, nil, locator, locator)
	if err != nil {
		t.Fatalf("step: %v", err)
	}
	if got := dueString(task.DueDate); got != "2026-07-21 00:00" {
		t.Fatalf("deadline = %s, want exactly one day of movement", got)
	}
	if len(result.DueSteps) != 1 || len(result.Tasks) != 1 {
		t.Fatalf("outcomes/tasks = %d/%d, want one of each", len(result.DueSteps), len(result.Tasks))
	}
}

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
