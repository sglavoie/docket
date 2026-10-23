package app_test

import (
	"errors"
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
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

// TestStepDueDateRefusesADateTheZoneSkipsEntirely is the second half of that
// refusal, and the reason the calendar arithmetic is normalized in UTC: a zone
// can lose a whole local date rather than one hour of it, so normalizing the
// adjacent day in the task's own zone would hand the validation the surviving
// neighbor and nothing could tell the requested destination never existed.
// Pacific/Apia jumped from 2011-12-29 to 2011-12-31, so both steps into
// 2011-12-30 must refuse the whole selection instead of landing on 2011-12-31.
func TestStepDueDateRefusesADateTheZoneSkipsEntirely(t *testing.T) {
	zone, err := time.LoadLocation("Pacific/Apia")
	if err != nil {
		t.Fatalf("LoadLocation: %v", err)
	}
	cases := []struct {
		name      string
		from      int
		direction app.DueStepDirection
	}{
		{"forward off the day before", 29, app.DueStepForward},
		{"backward off the day after", 31, app.DueStepBackward},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			skipped := taskWithDue("skipped", 1, "Skipped", dueAt(zone, 2011, time.December, testCase.from, 14, 30))
			untouched := taskWithDue("untouched", 2, "Untouched", dueAt(zone, 2012, time.January, 20, 9, 0))
			before := dueString(skipped.DueDate)
			env := newDueStepEnv(
				time.Date(2012, time.January, 10, 12, 0, 0, 0, zone),
				newProject("project", "Project", skipped, untouched),
			)

			_, stepErr := env.step(testCase.direction, nil, env.repo.loc(skipped.ID), env.repo.loc(untouched.ID))
			if !errors.Is(stepErr, app.ErrInvalidInput) {
				t.Fatalf("step error = %v, want ErrInvalidInput; landed %s", stepErr, dueString(skipped.DueDate))
			}
			if got := dueString(skipped.DueDate); got != before {
				t.Fatalf("refused task deadline = %s, want it unchanged at %s", got, before)
			}
			if got := dueString(untouched.DueDate); got != "2012-01-20 09:00" {
				t.Fatalf("companion deadline = %s, want the whole selection refused", got)
			}
			if len(env.repo.commits) != 0 {
				t.Fatalf("commits = %d, want none", len(env.repo.commits))
			}
		})
	}

	// A date-only deadline has no clock to skip, so it is the case that proves the
	// refusal reads the date rather than the wall time.
	t.Run("a date-only deadline is refused on the date alone", func(t *testing.T) {
		skipped := taskWithDue("dateonly", 1, "Date only", dueAt(zone, 2011, time.December, 29, 0, 0))
		env := newDueStepEnv(
			time.Date(2012, time.January, 10, 12, 0, 0, 0, zone),
			newProject("project", "Project", skipped),
		)

		_, stepErr := env.step(app.DueStepForward, nil, env.repo.loc(skipped.ID))
		if !errors.Is(stepErr, app.ErrInvalidInput) {
			t.Fatalf("step error = %v, want ErrInvalidInput; landed %s", stepErr, dueString(skipped.DueDate))
		}
		if got := dueString(skipped.DueDate); got != "2011-12-29 00:00" {
			t.Fatalf("refused task deadline = %s, want it unchanged", got)
		}
	})

	// Every other day in that zone still steps, so the refusal is the missing date
	// and not the zone.
	t.Run("an ordinary day in that zone still steps", func(t *testing.T) {
		ordinary := taskWithDue("ordinary", 1, "Ordinary", dueAt(zone, 2011, time.December, 31, 14, 30))
		env := newDueStepEnv(
			time.Date(2012, time.January, 10, 12, 0, 0, 0, zone),
			newProject("project", "Project", ordinary),
		)

		if _, err := env.step(app.DueStepForward, nil, env.repo.loc(ordinary.ID)); err != nil {
			t.Fatalf("step: %v", err)
		}
		if got := dueString(ordinary.DueDate); got != "2012-01-01 14:30" {
			t.Fatalf("deadline = %s, want an ordinary dated step across the year boundary", got)
		}
	})
}

// TestStepDueDateLandsOnTheStartOfADateWithNoMidnight is the date-only half of
// the zone checks: America/Havana skips 00:00 on 2026-03-08, but the date
// itself exists, so a date-only step resolves the start of that date — 01:00 —
// rather than refusing a midnight nobody asked for by clock.
func TestStepDueDateLandsOnTheStartOfADateWithNoMidnight(t *testing.T) {
	zone, err := time.LoadLocation("America/Havana")
	if err != nil {
		t.Fatalf("LoadLocation: %v", err)
	}
	task := taskWithDue("dateonly", 1, "Date only", dueAt(zone, 2026, time.March, 7, 0, 0))
	env := newDueStepEnv(time.Date(2026, time.March, 1, 12, 0, 0, 0, zone), newProject("project", "Project", task))

	if _, err := env.step(app.DueStepForward, nil, env.repo.loc(task.ID)); err != nil {
		t.Fatalf("step: %v", err)
	}
	if got := dueString(task.DueDate); got != "2026-03-08 01:00" {
		t.Fatalf("deadline = %s, want the start of 2026-03-08", got)
	}
	if task.DueHasTime() {
		t.Fatal("the stepped deadline gained a clock")
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
