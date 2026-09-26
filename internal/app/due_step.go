package app

import (
	"fmt"
	"time"

	"github.com/sglavoie/tt/internal/storage"
)

// DueStepDirection is the signed direction of one deadline step. The two values
// are the whole vocabulary: a step walks exactly one place along the deadline
// sequence — earlier dates, yesterday, unset, today, tomorrow, later dates — so
// a request naming any other magnitude is refused by name rather than clamped
// into one of these.
type DueStepDirection int

const (
	// DueStepBackward moves one place left along that sequence.
	DueStepBackward DueStepDirection = -1
	// DueStepForward moves one place right along it.
	DueStepForward DueStepDirection = 1
)

const (
	// dueDateStorageLayout and dueTimeStorageLayout are the canonical spellings
	// the durable task document stores a deadline in. A calculated deadline is
	// held to both before anything is mutated, so the operation refuses a value
	// the canonical writer would refuse rather than discovering it at render time.
	dueDateStorageLayout = "2006-01-02"
	dueTimeStorageLayout = "15:04"
)

// DueStepClock is one wall clock remembered for a task whose deadline was
// cleared. Stepping back out of unset restores it, which is what keeps a timed
// deadline timed across a round trip through unset; without one the destination
// is date-only, and date-only is local midnight.
type DueStepClock struct {
	Hour   int
	Minute int
}

// DueStepRequest carries the deadline-stepping intent of
// TaskOperationStepDueDate. RememberedClocks is keyed by the same
// project-qualified locator the targets are named with, so a presentation hands
// back exactly what it remembered for the rows it is stepping and nothing else;
// a target with no entry steps to a date-only deadline.
type DueStepRequest struct {
	Direction        DueStepDirection
	RememberedClocks map[TaskLocator]DueStepClock
}

// DueStepOutcome reports one target's committed deadline move. Before and After
// are the deadlines the transaction actually replaced and wrote, so a
// presentation updates its remembered clock from what landed rather than by
// re-reading a row the commit may have relocated. A nil pointer is an absent
// deadline on that side of the step.
type DueStepOutcome struct {
	Target TaskLocator
	Before *time.Time
	After  *time.Time
}

// calendarDay is a local calendar date with no instant attached. Deadline
// stepping is calendar arithmetic — a day is one day whether or not the zone
// spent 23 or 25 hours in it — so the sequence is walked over these and an
// instant is reconstructed only once, at the end, where it can be validated.
type calendarDay struct {
	year  int
	month time.Month
	day   int
	loc   *time.Location
}

func dayOf(t time.Time) calendarDay {
	year, month, day := t.Date()
	return calendarDay{year: year, month: month, day: day, loc: t.Location()}
}

// shift moves a calendar day by whole days through time.Date's own calendar
// normalization, which is what carries a month, year, or leap-day boundary
// correctly without ever adding a 24-hour duration to an instant. It normalizes
// in UTC and keeps c.loc untouched, so the answer is the date that was asked for
// and nothing else: a zone can skip a whole local date — Pacific/Apia lost
// 2011-12-30 outright, every hour of it — and normalizing there would quietly
// substitute the surviving neighbor, leaving nothing downstream able to tell that
// the requested destination never existed. Rebuilding an instant in c.loc is
// materializeDueDate's job, and that is the one place a skipped date or a skipped
// clock is refused.
func (c calendarDay) shift(days int) calendarDay {
	year, month, day := time.Date(c.year, c.month, c.day+days, 0, 0, 0, 0, time.UTC).Date()
	return calendarDay{year: year, month: month, day: day, loc: c.loc}
}

// sameDate compares two calendar days by date alone, which is what "this
// deadline falls on today" means regardless of the zone each side was read in.
func (c calendarDay) sameDate(other calendarDay) bool {
	return c.year == other.year && c.month == other.month && c.day == other.day
}

// dueStepInput is one target's whole contribution to the pure calculation:
// where its deadline stands, whether that deadline carries a wall clock —
// models.Task.DueHasTime is the one authority for that, never a second reading
// of the instant here — and the clock remembered from when it was last cleared.
type dueStepInput struct {
	current    *time.Time
	timed      bool
	remembered *DueStepClock
}

// stepDueDate is the deadline sequence in one pure function. Unset sits between
// yesterday and today, so leaving unset lands on the neighbor the direction
// names and arriving at unset happens from exactly one side: forward off
// yesterday, backward off today. Every other step moves one calendar day and
// preserves the clock it started with. A nil result is a cleared deadline.
func stepDueDate(in dueStepInput, direction DueStepDirection, reference calendarDay) (*time.Time, error) {
	switch direction {
	case DueStepForward, DueStepBackward:
	default:
		return nil, invalidInput("due", fmt.Sprintf(
			"a deadline step is one day forward or backward, not %d", int(direction)))
	}
	if in.current == nil {
		destination := reference
		if direction == DueStepBackward {
			destination = reference.shift(-1)
		}
		return materializeDueDate(destination, in.remembered)
	}
	from := dayOf(*in.current)
	if (direction == DueStepForward && from.sameDate(reference.shift(-1))) ||
		(direction == DueStepBackward && from.sameDate(reference)) {
		return nil, nil
	}
	var clock *DueStepClock
	if in.timed {
		clock = &DueStepClock{Hour: in.current.Hour(), Minute: in.current.Minute()}
	}
	return materializeDueDate(from.shift(int(direction)), clock)
}

// materializeDueDate reconstructs one local instant from a calendar day and an
// optional wall clock, and refuses everything the reconstruction cannot express.
// time.Date normalizes rather than fails, so the only way to learn that a zone
// skipped the requested clock — a spring-forward hour — or skipped the requested
// date outright is to read the components back off the result and compare them
// with what was asked for. Both comparisons are load-bearing, and both depend on
// the caller naming the date it meant rather than one the zone already rewrote.
// The storage round trip is checked here too, before any task is touched.
func materializeDueDate(day calendarDay, clock *DueStepClock) (*time.Time, error) {
	hour, minute := 0, 0
	if clock != nil {
		hour, minute = clock.Hour, clock.Minute
	}
	due := time.Date(day.year, day.month, day.day, hour, minute, 0, 0, day.loc)
	if !dayOf(due).sameDate(day) || due.Hour() != hour || due.Minute() != minute {
		return nil, invalidInput("due", fmt.Sprintf(
			"%04d-%02d-%02d %02d:%02d does not exist in %s",
			day.year, int(day.month), day.day, hour, minute, day.loc))
	}
	if err := validateStorableDueDate(due, clock != nil); err != nil {
		return nil, err
	}
	return &due, nil
}

// validateStorableDueDate holds a calculated deadline to the canonical spellings
// the durable document stores, so a date the schema's fixed-width layout cannot
// round-trip is refused before the mutation instead of after the render.
func validateStorableDueDate(due time.Time, timed bool) error {
	date := due.Format(dueDateStorageLayout)
	if parsed, err := time.Parse(dueDateStorageLayout, date); err != nil || parsed.Format(dueDateStorageLayout) != date {
		return invalidInput("due", fmt.Sprintf("date %q is not a canonical YYYY-MM-DD value", date))
	}
	if !timed {
		return nil
	}
	clock := due.Format(dueTimeStorageLayout)
	if parsed, err := time.Parse(dueTimeStorageLayout, clock); err != nil || parsed.Format(dueTimeStorageLayout) != clock {
		return invalidInput("due", fmt.Sprintf("time %q is not a canonical HH:MM value", clock))
	}
	return nil
}

// validateDueStepRequest checks everything about the request that the workspace
// has no say in, so a malformed direction or an impossible remembered clock is
// refused before a snapshot is even loaded.
func validateDueStepRequest(req DueStepRequest) error {
	switch req.Direction {
	case DueStepForward, DueStepBackward:
	default:
		return invalidInput("due", fmt.Sprintf(
			"a deadline step is one day forward or backward, not %d", int(req.Direction)))
	}
	for locator, clock := range req.RememberedClocks {
		if !locator.Valid() {
			return invalidInput("task", "project id and task id are required")
		}
		if clock.Hour < 0 || clock.Hour > 23 || clock.Minute < 0 || clock.Minute > 59 {
			return invalidInput("due", fmt.Sprintf(
				"remembered clock %02d:%02d is not a wall clock time", clock.Hour, clock.Minute))
		}
	}
	return nil
}

// plannedDueStep pairs one resolved target with the deadline the calculation
// chose for it, computed against the snapshot the commit will write and before
// any task is mutated.
type plannedDueStep struct {
	ref          TaskRef
	before       *time.Time
	after        *time.Time
	landsOnToday bool
}

// stepDueDates walks every selected task's deadline one calendar place in the
// requested direction and commits the whole selection as ONE transaction. It is
// deliberately not a loop over SetDueDate: the targets land on different dates —
// some cleared, some on today, some neither — so the calculation, the Today
// planning half, and the artifact that records it all have to be decided against
// one snapshot before anything is written. Any refusal aborts the batch with
// every project, planning artifact, and in-memory task exactly where it was.
func (s *Service) stepDueDates(locators []TaskLocator, req DueStepRequest) (TaskOperationResult, error) {
	if err := validateOperationTargets(locators); err != nil {
		return TaskOperationResult{}, err
	}
	if err := validateDueStepRequest(req); err != nil {
		return TaskOperationResult{}, err
	}
	builder := s.defaultedPlanningArtifacts(nil)
	if builder == nil {
		return TaskOperationResult{}, fmt.Errorf("app: deadline stepping requires a planning artifact builder")
	}
	snap, err := s.repo.Snapshot()
	if err != nil {
		return TaskOperationResult{}, err
	}
	refs, err := resolveRequestedTasks(snap, nil, locators)
	if err != nil {
		return TaskOperationResult{}, err
	}
	before := cloneSnapshot(snap)
	useRollbackDocuments(before)
	committed := false
	defer func() {
		if !committed {
			restoreSnapshotProjects(snap, before)
		}
	}()

	// One clock read: the reference day every target's sequence is anchored to and
	// the touch time every mutated task receives are the same instant, so a
	// selection stepped across midnight cannot land half on one day and half on
	// the next.
	now := s.clock.Now()
	planned, err := planDueSteps(refs, req, dayOf(now), now)
	if err != nil {
		return TaskOperationResult{}, err
	}

	targets := make([]PlanningReconcileTarget, 0, len(planned))
	for _, step := range planned {
		step.ref.Task.DueDate = cloneDueDatePtr(step.after)
		step.ref.Task.UpdatedAt = now
		if !step.landsOnToday {
			continue
		}
		target, _, mutationErr := ApplyPlanningActionToTask(step.ref.Task, step.ref.Project.ID, PlanningAddToday, now)
		if mutationErr != nil {
			return TaskOperationResult{}, mutationErr
		}
		targets = append(targets, target)
	}

	commits, indexes := distinctProjectCommits(refs, snap)
	if planningErr := attachPlanningArtifacts(commits, targets, builder); planningErr != nil {
		return TaskOperationResult{}, planningErr
	}
	if guardErr := attachReadSetRevisionGuards(commits); guardErr != nil {
		return TaskOperationResult{}, guardErr
	}
	undoAction := "Step Due Date"
	awaitBulkCommitBarrier(refs, snap, undoAction)
	outcome, err := s.commitProjects(commits, undoAction, dueStepUndoContext(req.Direction))
	if err != nil {
		return TaskOperationResult{}, err
	}
	committed = true

	result := TaskOperationResult{
		Tasks:         make([]TaskResult, 0, len(refs)),
		DueSteps:      make([]DueStepOutcome, 0, len(planned)),
		TransactionID: transactionIDAt(outcome.results, 0),
	}
	for _, step := range planned {
		result.Tasks = append(result.Tasks, newTaskResult(
			step.ref,
			revisionAt(outcome.results, indexes[step.ref.Project.ID]),
			transactionIDAt(outcome.results, indexes[step.ref.Project.ID]),
		))
		result.DueSteps = append(result.DueSteps, DueStepOutcome{
			Target: TaskLocator{ProjectID: step.ref.Project.ID, TaskID: step.ref.Task.ID},
			Before: step.before,
			After:  cloneDueDatePtr(step.after),
		})
	}
	for _, commit := range commits {
		result.TouchedProjectIDs = append(result.TouchedProjectIDs, commit.project.ID)
	}
	return result, nil
}

// planDueSteps is the whole validation phase: every target's destination is
// calculated and every Today transition is rehearsed on a copy, so a refusal
// anywhere — a clock the zone skips, a date the schema cannot store, a terminal
// task that would have to enter Today — rejects the complete selection with no
// task yet touched.
func planDueSteps(refs []TaskRef, req DueStepRequest, reference calendarDay, now time.Time) ([]plannedDueStep, error) {
	planned := make([]plannedDueStep, 0, len(refs))
	for _, ref := range refs {
		locator := TaskLocator{ProjectID: ref.Project.ID, TaskID: ref.Task.ID}
		input := dueStepInput{current: ref.Task.DueDate, timed: ref.Task.DueHasTime()}
		if clock, ok := req.RememberedClocks[locator]; ok {
			remembered := clock
			input.remembered = &remembered
		}
		after, err := stepDueDate(input, req.Direction, reference)
		if err != nil {
			return nil, err
		}
		step := plannedDueStep{
			ref:          ref,
			before:       cloneDueDatePtr(ref.Task.DueDate),
			after:        after,
			landsOnToday: after != nil && dayOf(*after).sameDate(reference),
		}
		if step.landsOnToday {
			rehearsal := *ref.Task
			if _, _, validationErr := ApplyPlanningActionToTask(
				&rehearsal, ref.Project.ID, PlanningAddToday, now,
			); validationErr != nil {
				return nil, validationErr
			}
		}
		planned = append(planned, step)
	}
	return planned, nil
}

func dueStepUndoContext(direction DueStepDirection) string {
	if direction == DueStepBackward {
		return "-1 day"
	}
	return "+1 day"
}

// attachReadSetRevisionGuards hangs one strict revision guard per project this
// mutation read on the first changeset. A guard is not the same contract as the
// project's own ExpectedRevision: that one opts into three-way merging, which
// would let a deadline calculated against one document land on a different one.
// Every guard rides one changeset because preconditions describe a single read
// set and are validated for the whole commit in one phase, ahead of any merge,
// conflict bundle, ledger image, or staged byte.
func attachReadSetRevisionGuards(commits []projectCommit) error {
	if len(commits) == 0 {
		return fmt.Errorf("app: revision guards require at least one project commit")
	}
	guards := make([]storage.ProjectRevisionPrecondition, 0, len(commits))
	for _, commit := range commits {
		if commit.project == nil {
			continue
		}
		guards = append(guards, storage.ProjectRevisionPrecondition{
			ProjectID:        commit.project.ID,
			ProjectName:      commit.project.Name,
			Path:             commit.project.Path,
			ExpectedRevision: commit.expectedRevision,
		})
	}
	commits[0].preconditions = append(commits[0].preconditions, guards...)
	return nil
}
