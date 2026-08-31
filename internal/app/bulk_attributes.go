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
	return s.bulkAttributeOverSnapshot(locators, undoAction, undoContext, func(_ Snapshot, refs []TaskRef) (func(TaskRef), error) {
		if validate != nil {
			for _, r := range refs {
				if validationErr := validate(r); validationErr != nil {
					return nil, validationErr
				}
			}
		}
		return func(r TaskRef) { mutate(r.Task) }, nil
	})
}

// bulkAttributeOverSnapshot is bulkAttribute for the mutations whose validation
// asks the workspace a question rather than the value one. plan runs once, after
// the targets resolve and before any task is touched, against the very snapshot
// the commit will write, and returns the per-target mutation. A refusal from
// plan aborts the whole batch with nothing persisted, so every target is judged
// on one coherent view of the workspace.
func (s *Service) bulkAttributeOverSnapshot(
	locators []TaskLocator,
	undoAction, undoContext string,
	plan func(Snapshot, []TaskRef) (func(TaskRef), error),
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
		mutate(r)
	}

	commits, idx := distinctProjectCommits(refs, snap)
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
	return s.bulkSetDueDate(locators, due)
}

// BulkSetDueDateAndPlanToday sets one due date and adds every target
// to Today in the same transaction. It is the application boundary used by
// interactive adapters whose "due today" gesture also carries explicit planning
// intent; the ordinary due-date methods remain attribute-only.
func (s *Service) BulkSetDueDateAndPlanToday(locators []TaskLocator, due time.Time) (BulkResult, error) {
	return s.bulkSetDueDateAndPlanToday(locators, due)
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
	return s.bulkAttribute(locators, "Clear Due Date", "due date", nil, func(t *models.Task) {
		t.DueDate = nil
		t.UpdatedAt = s.clock.Now()
	})
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

// taskContextEdit is an optional singleton-context write carried by a tag
// mutation. Riding the same mutate closure keeps a simultaneous tag and context
// change inside one transaction and one undo entry.
type taskContextEdit struct {
	Set   bool
	Value string
}

func (edit taskContextEdit) normalize() (taskContextEdit, error) {
	if !edit.Set || edit.Value == "" {
		return taskContextEdit{Set: edit.Set}, nil
	}
	normalized, err := models.NormalizeTaskContext(edit.Value)
	if err != nil {
		return taskContextEdit{}, invalidInput("context", err.Error())
	}
	return taskContextEdit{Set: true, Value: normalized}, nil
}

func (edit taskContextEdit) apply(t *models.Task) {
	if edit.Set {
		t.Context = edit.Value
	}
}

// bulkSetTags replaces the tag set on every selected task with tags, committing
// every touched project as ONE transaction. The tags are normalized, validated,
// and de-duplicated once up front — they are uniform across the batch — so a
// malformed tag aborts before any snapshot is mutated. An empty tags slice clears
// every task's tags. Tasks may span projects.
func (s *Service) bulkSetTags(
	locators []TaskLocator,
	tags []string,
	context taskContextEdit,
) (BulkResult, error) {
	normalized, err := normalizeBulkTags(tags)
	if err != nil {
		return BulkResult{}, err
	}
	contextEdit, err := context.normalize()
	if err != nil {
		return BulkResult{}, err
	}
	return s.bulkAttribute(locators, "Set Tags", "tags", nil, func(t *models.Task) {
		t.Tags = append([]string{}, normalized...)
		contextEdit.apply(t)
		t.UpdatedAt = s.clock.Now()
	})
}

// bulkPatchTags applies a tri-state tag edit to every selected task. Tags in
// add are made present everywhere, tags in remove are made absent everywhere,
// and all other memberships are preserved independently. The two sets are
// validated before any task is touched and every project commits in the same
// transaction.
func (s *Service) bulkPatchTags(
	locators []TaskLocator,
	add []string,
	remove []string,
	context taskContextEdit,
) (BulkResult, error) {
	normalizedAdd, err := normalizeBulkTags(add)
	if err != nil {
		return BulkResult{}, err
	}
	contextEdit, err := context.normalize()
	if err != nil {
		return BulkResult{}, err
	}
	normalizedRemove, err := normalizeTagList(remove)
	if err != nil {
		return BulkResult{}, err
	}
	removeSet := make(map[string]bool, len(normalizedRemove))
	for _, tag := range normalizedRemove {
		removeSet[tag] = true
	}
	for _, tag := range normalizedAdd {
		if removeSet[tag] {
			return BulkResult{}, invalidInput("tag", fmt.Sprintf("%q cannot be added and removed", tag))
		}
	}
	addSet := make(map[string]bool, len(normalizedAdd))
	for _, tag := range normalizedAdd {
		addSet[tag] = true
	}
	patchedTags := func(current []string) ([]string, error) {
		next := make([]string, 0, len(current)+len(normalizedAdd))
		for _, existing := range models.DedupeTags(current) {
			if !removeSet[existing] {
				next = append(next, existing)
			}
		}
		next = append(next, normalizedAdd...)
		return models.NormalizeTaskTags(next)
	}
	validate := func(ref TaskRef) error {
		if _, err := patchedTags(ref.Task.Tags); err != nil {
			return invalidInput("tag", err.Error())
		}
		return nil
	}
	return s.bulkAttribute(locators, "Set Tags", "tags", validate, func(t *models.Task) {
		t.Tags, _ = patchedTags(t.Tags)
		contextEdit.apply(t)
		t.UpdatedAt = s.clock.Now()
	})
}

// BulkToggleTag applies one all-or-none toggle to a frozen target
// set. If every task already carries tag, it is removed everywhere; otherwise
// it is added everywhere. The membership decision and mutation share the same
// repository snapshot, so concurrent presentation state cannot split a bulk
// toggle into mixed results.
func (s *Service) BulkToggleTag(locators []TaskLocator, tag string) (BulkResult, error) {
	normalized := models.NormalizeTag(tag)
	if err := models.ValidateTag(normalized); err != nil {
		return BulkResult{}, invalidInput("tag", err.Error())
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
		return BulkResult{}, fmt.Errorf("capture bulk tag toggle rollback state: %w", err)
	}
	useRollbackDocuments(before)
	committed := false
	defer func() {
		if !committed {
			restoreSnapshotProjects(snap, before)
		}
	}()

	remove := true
	for _, ref := range refs {
		remove = remove && ref.Task.HasTag(normalized)
	}
	now := s.clock.Now()
	for _, ref := range refs {
		if remove {
			ref.Task.RemoveTag(normalized)
		} else {
			ref.Task.AddTag(normalized)
		}
		ref.Task.UpdatedAt = now
	}

	action := "Add Tag"
	if remove {
		action = "Remove Tag"
	}
	commits, index := distinctProjectCommits(refs, snap)
	results, err := s.commitProjects(commits, action, normalized)
	if err != nil {
		return BulkResult{}, err
	}
	committed = true
	out := BulkResult{
		Tasks:         make([]TaskResult, 0, len(refs)),
		TransactionID: transactionIDAt(results.results, 0),
	}
	for _, ref := range refs {
		out.Tasks = append(out.Tasks, newTaskResult(
			ref,
			revisionAt(results.results, index[ref.Project.ID]),
			transactionIDAt(results.results, index[ref.Project.ID]),
		))
	}
	for _, commit := range commits {
		out.TouchedProjectIDs = append(out.TouchedProjectIDs, commit.project.ID)
	}
	return out, nil
}

func normalizeBulkTags(tags []string) ([]string, error) {
	normalized, err := models.NormalizeTaskTags(tags)
	if err != nil {
		return nil, invalidInput("tag", err.Error())
	}
	return normalized, nil
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
