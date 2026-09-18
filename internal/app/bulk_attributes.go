package app

import (
	"fmt"
	"time"

	"github.com/sglavoie/tt/internal/models"
)

// Deliberately absent: BulkSetSection and BulkSetStage.
//
// The locked design retires task sections and custom stages in favor of one
// WorkflowState (see the migration precedence in the workspace design). A
// view's legacy "move selection to section X" bulk action is therefore not a
// new attribute use case — it is a state transition, and its target section
// maps onto a WorkflowState (Next->next, Waiting->waiting, Someday->someday,
// Inbox->inbox, Completed->done, ...). Route those through BulkTransition,
// which validates the transition and runs the same atomic single-commit path
// these attribute operations use.
//
// Custom-stage bulk assignment has no WorkflowState equivalent and is retired
// outright: when a view that offered a bulk "set stage" action migrates, that
// action is dropped rather than reimplemented here. This is why no
// BulkSetStage exists to call.

// bulkAttribute is the shared runner for every bulk attribute mutation. It
// mirrors the BulkTransition path: load a
// revision-consistent snapshot, resolve the distinct target tasks (duplicates
// collapsed, an unresolved id aborts the batch), validate every resolved task
// before touching any, apply mutate to each, and commit every distinct project
// as ONE transaction. validate runs against all refs first, so an invalid input
// aborts the whole batch before any mutation persists — the batch is
// all-or-nothing, matching the atomic guarantee every other bulk use case gives.
// A nil validate skips the pre-check; a nil mutate is a programming error.
func (s *Service) bulkAttribute(locators []TaskLocator, undoAction, undoContext string, validate func(TaskRef) error, mutate func(*models.Task)) (BulkResult, error) {
	return s.bulkAttributeOverSnapshot(locators, undoAction, undoContext, func(_ Snapshot, refs []TaskRef) (func(TaskRef) error, error) {
		if validate != nil {
			for _, r := range refs {
				if validationErr := validate(r); validationErr != nil {
					return nil, validationErr
				}
			}
		}
		return func(r TaskRef) error { mutate(r.Task); return nil }, nil
	})
}

// bulkAttributeOverSnapshot is bulkAttribute for the mutations whose validation
// asks the workspace a question rather than the value one. plan runs once, after
// the targets resolve and before any task is touched, against the very snapshot
// the commit will write, and returns the per-target mutation. A refusal from
// plan aborts the whole batch with nothing persisted, so every target is judged
// on one coherent view of the workspace.
//
// The mutation may refuse too, for the mutations whose one writer is a validating
// model primitive: a refusal there aborts the batch on the same terms plan's
// does, restoring every project the loop had already touched, rather than
// committing a selection the caller would have no way to tell apart from a
// complete one.
func (s *Service) bulkAttributeOverSnapshot(
	locators []TaskLocator,
	undoAction, undoContext string,
	plan func(Snapshot, []TaskRef) (func(TaskRef) error, error),
) (BulkResult, error) {
	if len(locators) == 0 {
		return BulkResult{}, invalidInput("tasks", "at least one task id is required")
	}
	snap, err := s.repo.Snapshot()
	if err != nil {
		return BulkResult{}, err
	}
	refs, err := resolveRequestedTasks(snap, nil, locators)
	if err != nil {
		return BulkResult{}, err
	}
	before, err := cloneSnapshot(snap)
	if err != nil {
		return BulkResult{}, fmt.Errorf("capture bulk attribute rollback state: %w", err)
	}
	useRollbackDocuments(before)
	committed := false
	defer func() {
		if !committed {
			restoreSnapshotProjects(snap, before)
		}
	}()
	mutate, err := plan(snap, refs)
	if err != nil {
		return BulkResult{}, err
	}
	for _, r := range refs {
		if mutationErr := mutate(r); mutationErr != nil {
			return BulkResult{}, mutationErr
		}
	}

	commits, idx := distinctProjectCommits(refs, snap)
	awaitBulkCommitBarrier(refs, snap, undoAction)
	results, err := s.commitProjects(commits, undoAction, undoContext)
	if err != nil {
		return BulkResult{}, err
	}
	committed = true

	out := BulkResult{
		Tasks:         make([]TaskResult, 0, len(refs)),
		TransactionID: transactionIDAt(results.results, 0),
	}
	for _, r := range refs {
		out.Tasks = append(out.Tasks, newTaskResult(
			r,
			revisionAt(results.results, idx[r.Project.ID]),
			transactionIDAt(results.results, idx[r.Project.ID]),
		))
	}
	for _, c := range commits {
		out.TouchedProjectIDs = append(out.TouchedProjectIDs, c.project.ID)
	}
	return out, nil
}

// BulkSetPriority sets one priority on every selected task, committing every
// touched project as ONE transaction. The priority is validated once up front —
// it is uniform across the batch — so an out-of-range value aborts before any
// snapshot is mutated. Tasks may span projects.
func (s *Service) BulkSetPriority(locators []TaskLocator, priority int) (BulkResult, error) {
	if err := models.ValidatePriority(priority, true); err != nil {
		return BulkResult{}, invalidInput("priority", err.Error())
	}
	return s.bulkAttribute(locators, "Set Priority", fmt.Sprintf("priority %d", priority), nil, func(t *models.Task) {
		t.Priority = priority
		t.UpdatedAt = s.clock.Now()
	})
}

// BulkSetDueDate sets one calendar due date on every selected task,
// committing every touched project as ONE transaction. Use
// BulkClearDueDate to remove a due date. Tasks may span projects.
func (s *Service) BulkSetDueDate(locators []TaskLocator, due time.Time) (BulkResult, error) {
	return withDueEdits(s.bulkSetDueDate(locators, due))
}

// BulkSetDueDateAndPlanToday sets one due date and adds every target
// to Today in the same transaction. It is the application boundary used by
// interactive adapters whose "due today" gesture also carries explicit planning
// intent; the ordinary due-date methods remain attribute-only.
func (s *Service) BulkSetDueDateAndPlanToday(locators []TaskLocator, due time.Time) (BulkResult, error) {
	return withDueEdits(s.bulkSetDueDateAndPlanToday(locators, due))
}

// withDueEdits names every task a committed bulk mutation gave an explicit
// deadline instruction. It wraps the three due-date entry points rather than
// reaching into the shared bulk machinery, because only the entry point knows
// that the attribute it just wrote was the deadline.
func withDueEdits(result BulkResult, err error) (BulkResult, error) {
	if err != nil {
		return BulkResult{}, err
	}
	result.DueEdits = taskResultLocators(result.Tasks)
	return result, nil
}

func (s *Service) bulkSetDueDateAndPlanToday(locators []TaskLocator, due time.Time) (BulkResult, error) {
	if len(locators) == 0 {
		return BulkResult{}, invalidInput("tasks", "at least one task id is required")
	}
	builder := s.defaultedPlanningArtifacts(nil)
	if builder == nil {
		return BulkResult{}, fmt.Errorf("app: planning mutations require a planning artifact builder")
	}
	snap, err := s.repo.Snapshot()
	if err != nil {
		return BulkResult{}, err
	}
	refs, err := resolveRequestedTasks(snap, nil, locators)
	if err != nil {
		return BulkResult{}, err
	}
	before, err := cloneSnapshot(snap)
	if err != nil {
		return BulkResult{}, fmt.Errorf("capture due-today rollback state: %w", err)
	}
	useRollbackDocuments(before)
	committed := false
	defer func() {
		if !committed {
			restoreSnapshotProjects(snap, before)
		}
	}()

	now := s.clock.Now()
	for _, ref := range refs {
		clone := *ref.Task
		if _, _, validationErr := ApplyPlanningActionToTask(
			&clone, ref.Project.ID, PlanningAddToday, now,
		); validationErr != nil {
			return BulkResult{}, validationErr
		}
	}
	targets := make([]PlanningReconcileTarget, 0, len(refs))
	for _, ref := range refs {
		ref.Task.DueDate = cloneDueDatePtr(&due)
		ref.Task.UpdatedAt = now
		target, _, mutationErr := ApplyPlanningActionToTask(
			ref.Task, ref.Project.ID, PlanningAddToday, now,
		)
		if mutationErr != nil {
			return BulkResult{}, mutationErr
		}
		targets = append(targets, target)
	}

	commits, indexes := distinctProjectCommits(refs, snap)
	if planningErr := attachPlanningArtifacts(commits, targets, builder); planningErr != nil {
		return BulkResult{}, planningErr
	}
	outcome, err := s.commitProjects(commits, "Set Due Date", "due today")
	if err != nil {
		return BulkResult{}, err
	}
	committed = true

	result := BulkResult{
		Tasks:         make([]TaskResult, 0, len(refs)),
		TransactionID: transactionIDAt(outcome.results, 0),
	}
	for _, ref := range refs {
		result.Tasks = append(result.Tasks, newTaskResult(
			ref,
			revisionAt(outcome.results, indexes[ref.Project.ID]),
			transactionIDAt(outcome.results, indexes[ref.Project.ID]),
		))
	}
	for _, commit := range commits {
		result.TouchedProjectIDs = append(result.TouchedProjectIDs, commit.project.ID)
	}
	return result, nil
}

func (s *Service) bulkSetDueDate(locators []TaskLocator, due time.Time) (BulkResult, error) {
	return s.bulkAttribute(locators, "Set Due Date", "due date", nil, func(t *models.Task) {
		t.DueDate = cloneDueDatePtr(&due)
		t.UpdatedAt = s.clock.Now()
	})
}

// BulkClearDueDate removes the due date from every
// selected task, committing every touched project as ONE transaction. Clearing a
// task that has no due date is a no-op that still commits the (unchanged)
// project, matching the idempotent single-task attribute mutations. Tasks may
// span projects.
func (s *Service) BulkClearDueDate(locators []TaskLocator) (BulkResult, error) {
	return withDueEdits(s.bulkAttribute(locators, "Clear Due Date", "due date", nil, func(t *models.Task) {
		t.DueDate = nil
		t.UpdatedAt = s.clock.Now()
	}))
}

// BulkDefer hides every selected task until the given date, or clears the
// deferral when until is nil. Deferring moves each nonterminal task to Backlog and
// removes it from Today and Next atomically with the task writes. A terminal
// task rejects the complete batch. Clearing preserves lifecycle and planning.
func (s *Service) BulkDefer(locators []TaskLocator, until *time.Time) (BulkResult, error) {
	if until == nil {
		return s.bulkAttribute(locators, "Defer Tasks", "clear", nil, func(t *models.Task) {
			t.DeferredUntil = nil
			t.UpdatedAt = s.clock.Now()
		})
	}
	if len(locators) == 0 {
		return BulkResult{}, invalidInput("tasks", "at least one task id is required")
	}
	snap, err := s.repo.Snapshot()
	if err != nil {
		return BulkResult{}, err
	}
	refs, err := resolveRequestedTasks(snap, nil, locators)
	if err != nil {
		return BulkResult{}, err
	}
	before, err := cloneSnapshot(snap)
	if err != nil {
		return BulkResult{}, fmt.Errorf("capture bulk deferral rollback state: %w", err)
	}
	useRollbackDocuments(before)
	committed := false
	defer func() {
		if !committed {
			restoreSnapshotProjects(snap, before)
		}
	}()
	for _, ref := range refs {
		if refusal := deferralRefusesState(ref.Task.State()); refusal != nil {
			return BulkResult{}, refusal
		}
	}
	now := s.clock.Now()
	for _, ref := range refs {
		u := *until
		if stateErr := ref.Task.ApplyDeferAt(u, now); stateErr != nil {
			return BulkResult{}, invalidInput("defer", stateErr.Error())
		}
	}

	commits, indexes := distinctProjectCommits(refs, snap)
	targetLocators := make([]TaskLocator, 0, len(refs))
	for _, ref := range refs {
		targetLocators = append(targetLocators, TaskLocator{ProjectID: ref.Project.ID, TaskID: ref.Task.ID})
	}
	if planningErr := attachPlanningArtifacts(
		commits,
		collectPlanningReconcileTargets(commits, nil, targetLocators),
		s.defaultedPlanningArtifacts(nil),
	); planningErr != nil {
		return BulkResult{}, planningErr
	}
	outcome, err := s.commitProjects(commits, "Defer Tasks", "defer")
	if err != nil {
		return BulkResult{}, err
	}
	committed = true

	result := BulkResult{
		Tasks:         make([]TaskResult, 0, len(refs)),
		TransactionID: transactionIDAt(outcome.results, 0),
	}
	for _, ref := range refs {
		result.Tasks = append(result.Tasks, newTaskResult(
			ref,
			revisionAt(outcome.results, indexes[ref.Project.ID]),
			transactionIDAt(outcome.results, indexes[ref.Project.ID]),
		))
	}
	for _, commit := range commits {
		result.TouchedProjectIDs = append(result.TouchedProjectIDs, commit.project.ID)
	}
	return result, nil
}

// BulkSetPinned pins or unpins every selected task, committing every
// touched project as one transaction. Each newly pinned task receives the same
// operation timestamp; unpinning clears both the flag and ordering timestamp.
func (s *Service) BulkSetPinned(locators []TaskLocator, pinned bool) (BulkResult, error) {
	now := s.clock.Now()
	action := "Unpin Tasks"
	if pinned {
		action = "Pin Tasks"
	}
	return s.bulkAttribute(locators, action, "pin", nil, func(t *models.Task) {
		t.Pinned = pinned
		if pinned {
			pinnedAt := now
			t.PinnedAt = &pinnedAt
		} else {
			t.PinnedAt = nil
		}
		t.UpdatedAt = now
	})
}

// bulkToggleReview flips the review flag over the whole selection as ONE
// decision: every target is flagged unless all of them already are, in which
// case every target is cleared. The direction is computed here, in the
// application layer, from the very snapshot the commit writes — never from a
// presentation's cursor and never from a second read — so a selection can never
// flag half of itself, and one target and many take the identical path.
//
// Review coexists with every other axis, so no target is refusable: the flag
// rides ApplyReview, which no-ops on an already-correct member and therefore
// leaves that task's touch time, lifecycle state and clock, dates, planning
// membership, and pin exactly where they were. Only the explicitly targeted
// tasks are touched; a parent's review flag says nothing about its children.
func (s *Service) bulkToggleReview(locators []TaskLocator) (BulkResult, error) {
	return s.bulkAttributeOverSnapshot(locators, "Toggle Review", "review", func(_ Snapshot, refs []TaskRef) (func(TaskRef) error, error) {
		review := false
		for _, ref := range refs {
			if !ref.Task.Review {
				review = true
				break
			}
		}
		now := s.clock.Now()
		return func(ref TaskRef) error { ref.Task.ApplyReview(review, now); return nil }, nil
	})
}
